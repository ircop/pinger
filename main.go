package main

import (
	"flag"
	"./logger"
	"./ccfg"
	"./pinger"
	"./hostpool"
	"net"
	"fmt"
	"log"
	"net/http"
	"github.com/gorilla/mux"
	"errors"
	"strconv"
	"strings"
	"encoding/json"
)

var cfg *ccfg.Cfg

func main() {
	configPath := flag.String("c", "./pinger.toml", "Config file location")
	flag.Parse()

	logger.Debug("Reading config file '%s'...", *configPath)
	cfg = ccfg.New(configPath)
	logger.SetPath(cfg.LogPath)
	logger.SetDebug(cfg.LogDebug)


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
	router.HandleFunc("/store-host", StoreHost)
	router.HandleFunc("/remove-host", RemoveHost)
	router.Use(Middleware)

	pinger.Pinger.Init()
	if cfg.Ssl {
		log.Fatal(http.ServeTLS(listener, router, cfg.SslCert, cfg.SslKey))
	} else {
		log.Fatal(http.Serve(listener, router))
	}
}

func RemoveHost(w http.ResponseWriter, r *http.Request) {
	params := GetParams(r)
	hostParam, ok := params["host"]
	if !ok {
		ReturnError(w,r,"Missing 'host' parameter", http.StatusBadRequest)
		return
	}

	if host, ok := hostpool.Pool.Hosts.Load(hostParam); ok {
		logger.Debug("Removing host '%s' from pool", hostParam)
		fmt.Fprintf(w, `{"ok":true}`)
		host.(*hostpool.Host).Stop()
		return
	} else {
		ReturnError(w,r, "Host not found", http.StatusBadRequest)
		return
	}
}

func StoreHost(w http.ResponseWriter, r *http.Request) {
	params := GetParams(r)
	host, ok := params["host"]
	if !ok {
		ReturnError(w,r,"Missing 'host' parameter", http.StatusBadRequest)
		return
	}
	ip := net.ParseIP(host)
	if ip == nil {
		ReturnError(w,r,fmt.Sprintf("Cannot parpse '%s' into IP address", host), http.StatusBadRequest)
		return
	}

	probesStr, ok := params["probes"]
	probes := cfg.DefaultProbes
	if ok {
		p, err := strconv.ParseInt(probesStr, 10, 32)
		if err != nil {
			ReturnError(w,r,"Cannot parse 'probes', not integer?", http.StatusBadRequest)
			return
		}
		probes = int(p)
	}

	intervalStr, ok := params["interval"]
	if !ok {
		ReturnError(w,r,"Missing 'interval' parameter", http.StatusBadRequest)
		return
	}
	i, err := strconv.ParseInt(intervalStr, 10, 32)
	if err != nil {
		ReturnError(w,r,fmt.Sprintf("Cannot parse '%s' into interval integer", intervalStr), http.StatusBadRequest)
		return
	}
	if i < 30 {
		ReturnError(w,r,fmt.Sprintf("Minimal interval is 30 sec, %d given", i), http.StatusBadRequest)
		return
	}
	interval := i

	timeout := cfg.DefaultTimeout
	if toutStr, ok := params["timeout"]; ok {
		t, err := strconv.ParseInt(toutStr, 10, 64)
		if err != nil {
			ReturnError(w,r, fmt.Sprintf("Cannot parse interval '%s' to integer", toutStr), http.StatusBadRequest)
			return
		}
		timeout = t
	}

	err = hostpool.Pool.AddHost(host, probes, interval, timeout, cfg.ResultUrl)
	if err != nil {
		ReturnError(w,r,fmt.Sprintf("Failed to add host: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, `{"ok":true}`)
	return
	// need: host ; intervalSec ; timeout
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
		ReturnError(w, r,"Missing host parameter", http.StatusBadRequest)
		return
	}

	probes := 5
	if probesStr, ok := params["probes"]; ok {
		p, err := strconv.ParseInt(probesStr, 10, 32)
		if err != nil {
			ReturnError(w, r,"Cannot parse 'probes'", http.StatusBadRequest)
			return
		}
		probes = int(p)
	}

	if "now" == pingType {
		result, err := pinger.Pinger.PingNow(host, probes)
		if err != nil {
			ReturnError(w, r, fmt.Sprintf("Ping : %s (%s)", err.Error()), http.StatusInternalServerError)
			return
		}
		bytes, e := json.Marshal(result)
		if e != nil {
			ReturnError(w, r, fmt.Sprintf("Internal error: %s", e.Error()), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "%s", string(bytes))
		return
	} else if "api" == pingType {
		if "" == cfg.ResultUrl {
			ReturnError(w, r, "Missing result.api-url in config", http.StatusInternalServerError)
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

func ReturnError(w http.ResponseWriter, r *http.Request, err string, status int) {
	http.Error(w, err, status)
	logger.Err("[web]: Error: %s (%s)", err, r.URL)
	return
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