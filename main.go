package main

import (
	"flag"
	"./logger"
	"./ccfg"
	"./pinger"
	"net"
	"fmt"
	"log"
	"net/http"
	"github.com/gorilla/mux"
	"errors"
	//"golang.org/x/net/icmp"
	//"golang.org/x/net/ipv4"
	//"os"
	"strconv"
	"strings"
	"encoding/json"
)

var cfg *ccfg.Cfg

func main() {
	configPath := flag.String("c", "./pinger.toml", "Config file location")
	flag.Parse()

	logger.Debug("Reading config file '%s'...", *configPath)
	//cfg := ccfg.New(configPath)
	cfg = ccfg.New(configPath)


	proto := "http"
	if cfg.Ssl {
		proto = "https"
	}

	logger.Log("Listening on %s://%s:%s", proto, cfg.ListenIp, cfg.ListenPort)
	listener, err := net.Listen("tcp4", fmt.Sprintf("%s:%s", cfg.ListenIp, cfg.ListenPort))
	if err != nil {
		panic(err)
	}

	// Serve http(s)
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", Dummy)
	router.HandleFunc("/ping-now", PingNow)
	router.HandleFunc("/ping-api", PingApi)
	router.Use(Middleware)

	pinger.Pinger.Init()
	if cfg.Ssl {
		log.Fatal(http.ServeTLS(listener, router, cfg.SslCert, cfg.SslKey))
	} else {
		log.Fatal(http.Serve(listener, router))
	}
}

func PingNow(w http.ResponseWriter, r *http.Request) {
	Ping(w,r,"now")
}
func PingApi(w http.ResponseWriter, r *http.Request) {
	Ping(w,r,"api")
}

func Ping(w http.ResponseWriter, r *http.Request, pingType string) {
	params := GetParams(r)
	host, ok := params["host"]
	if !ok {
		http.Error(w, "Missing host", http.StatusBadRequest)
		logger.Err("[web]: missing host parameter (%s)", r.URL)
		return
	}

	probes := 5
	if probesStr, ok := params["probes"]; ok {
		p, err := strconv.ParseInt(probesStr, 10, 32)
		if err != nil {
			http.Error(w, "Cannot parse 'probes'", http.StatusBadRequest)
			logger.Err("[web]: Cannot parse 'probes' (%s)", http.StatusBadRequest, r.URL)
			return
		}
		probes = int(p)
	}

	if "now" == pingType {
		result, err := pinger.Pinger.PingNow(host, probes)
		if err != nil {
			http.Error(w, fmt.Sprintf("Internal error: %s", err.Error()), http.StatusInternalServerError)
			logger.Err("[web]: Ping : %s (%s)", err.Error(), r.URL)
			return
		}
		bytes, e := json.Marshal(result)
		if e != nil {
			http.Error(w, fmt.Sprintf("Internal error: %s", e.Error()), http.StatusInternalServerError)
			logger.Err("[web]: Cannot marshal result: %s", e.Error())
			return
		}

		fmt.Fprintf(w, "%s", string(bytes))
		return
	} else if "api" == pingType {
		if "" == cfg.ResultUrl {
			http.Error(w, "Missing result.api-url in config", http.StatusInternalServerError)
			return
		}

		go pinger.Pinger.PingResultUrl(host, probes, cfg.ResultUrl)
		fmt.Fprintf(w, `{"ok":true}`)
		return
	}
}

func Dummy(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("Dummy answer"))
}

func GetParams(r *http.Request) map[string]string {
	params := make(map[string]string)
	for param, val := range r.URL.Query() {
		param := strings.ToLower(param)
		v := strings.Join(val, "")
		if len(v) > 0 {
			params[param] = v
		}
	}

	return params
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				var e error
				switch x := r.(type) {
				case string:
					e = errors.New(x)
				case error:
					e = x
				default:
					e = errors.New("Unknown panic")
				}
				logger.Err("[web]: recovered in middleware: %#v", e.Error())
				http.Error(w, "Internal error", 500)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, req);
	})
}