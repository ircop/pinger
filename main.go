package main

import (
	"./ccfg"
	"./hostpool"
	"./logger"
	"./pinger"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"io"
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

	logger.Log("Listening on %s://%s:%s", proto, cfg.ListenIP, cfg.ListenPort)
	listener, err := net.Listen("tcp4", fmt.Sprintf("%s:%s", cfg.ListenIP, cfg.ListenPort))
	if err != nil {
		panic(err)
	}

	// Serve http(s)
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/ping-now", PingNow)
	router.HandleFunc("/ping-api", PingAPI)
	router.HandleFunc("/store-host", StoreHost)
	router.HandleFunc("/remove-host", RemoveHost)
	router.HandleFunc("/dump-hosts", DumpHosts)
	router.Use(Middleware)

	if err := pinger.Pinger.Init(); err != nil {
		logger.Debug("Cannot initialize pinger: %s", err.Error())
		return
	}
	if cfg.Ssl {
		log.Fatal(http.ServeTLS(listener, router, cfg.SslCert, cfg.SslKey))
	} else {
		log.Fatal(http.Serve(listener, router))
	}
}

/*
DumpHosts Output all in-memory hosts
 */
func DumpHosts(w http.ResponseWriter, r *http.Request) {
	//
}

/*
RemoveHost removing host from pool
 */
func RemoveHost(w http.ResponseWriter, r *http.Request) {
	params := GetParams(r)
	hostParam, ok := params["host"]
	if !ok {
		ReturnError(w, r, "Missing 'host' parameter", http.StatusBadRequest)
		return
	}

	if host, ok := hostpool.Pool.Hosts.Load(hostParam); ok {
		logger.Debug("Removing host '%s' from pool", hostParam)
		fmt.Fprintf(w, `{"ok":true}`)
		host.(*hostpool.Host).Stop()
		return
	}

	ReturnError(w, r, "Host not found", http.StatusBadRequest)
}

/*
StoreHost stores host into pool
 */
func StoreHost(w http.ResponseWriter, r *http.Request) {
	params := GetParams(r)
	err := CheckParams(params, []string{"host", "interval"})
	if err != nil {
		ReturnError(w, r, err.Error(), http.StatusBadRequest)
		return
	}

	ip := net.ParseIP(params["host"])
	if ip == nil {
		ReturnError(w, r, fmt.Sprintf("Cannot parpse '%s' into IP address", params["host"]), http.StatusBadRequest)
		return
	}

	probesStr, ok := params["probes"]
	probes := cfg.DefaultProbes
	if ok {
		p, e := strconv.ParseInt(probesStr, 10, 32)
		if e != nil {
			ReturnError(w, r, "Cannot parse 'probes', not integer?", http.StatusBadRequest)
			return
		}
		probes = int(p)
	}

	i, e := strconv.ParseInt(params["interval"], 10, 32)
	if e != nil {
		ReturnError(w, r, fmt.Sprintf("Cannot parse '%s' into interval integer", params["interval"]), http.StatusBadRequest)
		return
	}
	if i < 30 {
		ReturnError(w, r, fmt.Sprintf("Minimal interval is 30 sec, %d given", i), http.StatusBadRequest)
		return
	}
	interval := i

	timeout := cfg.DefaultTimeout
	if toutStr, ok := params["timeout"]; ok {
		t, e := strconv.ParseInt(toutStr, 10, 64)
		if e != nil {
			ReturnError(w, r, fmt.Sprintf("Cannot parse interval '%s' to integer", toutStr), http.StatusBadRequest)
			return
		}
		timeout = t
	}

	err = hostpool.Pool.AddHost(params["host"], probes, interval, timeout, cfg.ResultURL)
	if err != nil {
		ReturnError(w, r, fmt.Sprintf("Failed to add host: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, `{"ok":true}`)
	// need: host ; intervalSec ; timeout
}

/*
PingNow Pinging host instantly ; blocks http stream
 */
func PingNow(w http.ResponseWriter, r *http.Request) {
	Ping(w, r, "now")
}
/*
PingAPI Pinging host in background; after finish - make api call
 */
func PingAPI(w http.ResponseWriter, r *http.Request) {
	Ping(w, r, "api")
}

/*
Ping calls ping now or ping api
 */
func Ping(w io.Writer, r *http.Request, pingType string) {
	params := GetParams(r)
	host, ok := params["host"]
	if !ok {
		ReturnError(w, r, "Missing host parameter", http.StatusBadRequest)
		return
	}

	probes := 5
	if probesStr, ok := params["probes"]; ok {
		p, err := strconv.ParseInt(probesStr, 10, 32)
		if err != nil {
			ReturnError(w, r, "Cannot parse 'probes'", http.StatusBadRequest)
			return
		}
		probes = int(p)
	}

	if "now" == pingType {
		result, err := pinger.Pinger.PingNow(host, probes)
		if err != nil {
			ReturnError(w, r, fmt.Sprintf("Ping : %s", err.Error()), http.StatusInternalServerError)
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
		if "" == cfg.ResultURL {
			ReturnError(w, r, "Missing result.api-url in config", http.StatusInternalServerError)
			return
		}

		go pinger.Pinger.PingResultURL(host, probes, cfg.ResultURL)
		fmt.Fprintf(w, `{"ok":true}`)
		return
	}
}

/*
ReturnError returns an http error and logs it into error log
 */
//func ReturnError(w http.ResponseWriter, r *http.Request, err string, status int) {
func ReturnError(w io.Writer, r *http.Request, err string, status int) {
	//http.Error(w, err, status)
	fmt.Fprintf(w, fmt.Sprintf(`{"ok":false, "message":"%s"}`, err) )
	logger.Err("[web]: Error: %s (%s)", err, r.URL)
}

/*
GetParams parses query string into parameters map
 */
func GetParams(r *http.Request) map[string]string {
	params := make(map[string]string)
	for param, val := range r.URL.Query() {
		p := strings.ToLower(param)
		v := strings.Join(val, "")
		if len(v) > 0 {
			params[p] = v
		}
	}

	return params
}

/*
CheckParams checking url parameters for set of required ones
 */
func CheckParams(params map[string]string, required []string) error {
	for _, p := range required {
		if _, exist := params[p]; !exist {
			return fmt.Errorf("Missing parameter '%s'", p)
		}
	}
	return nil
}

/*
Middleware is router middleware func
 */
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
		next.ServeHTTP(w, req)
	})
}
