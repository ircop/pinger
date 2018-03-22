package notify

import (
	"sync"
	"time"
	"pinger/pinger"
	"encoding/json"
	"pinger/logger"
	"pinger/httpclient"
	"net/http"
	"bytes"
)

/*
Notify buffer - buffer host's changes for some period (given in config); every time ticker - sends updates
to update url's

store: [updateurls] => [host=>state][host=>state]
*/

type buffer struct {
	IntervalSec		int64
	Urls			sync.Map
	mx				sync.Mutex
}

// Buffer - global buffer struct instance
var Buffer buffer

func (b *buffer) Lock() {
	b.mx.Lock()
}
func (b *buffer) Unlock() {
	b.mx.Unlock()
}

// BufferResult - add new ping result to result map for furture updates
func (b *buffer) BufferResult(url string, ip string, result pinger.PingResult) {
	resultMap, _ := b.Urls.LoadOrStore(url, &sync.Map{})
	resultMap.(*sync.Map).Store(ip, result)
}

func (b *buffer) Start(interval int64) {
	b.IntervalSec = interval
	ticker := time.NewTicker(time.Duration(b.IntervalSec) * time.Second)

	for {
		select {
		case <- ticker.C:
			//logger.Debug("[buffer.Start]: <- ticker.C")
			// todo: loop over all urls => hosts
			b.Urls.Range(func(k, v interface{}) bool {							// url->results[ip->result]
				url := k.(string)
				//url := "https://w-tech.ip-home.net/pingupdate404"
				values := make(map[string]bool)
				hostupdates := v.(*sync.Map)
				hostupdates.Range(func(ipInterface, u interface{}) bool {		// ip->result
					ip := ipInterface.(string)
					update := u.(pinger.PingResult)
					values[ip] = update.Alive
					hostupdates.Delete(ip)
					return true
				})

				// send json to url
				jsonValues, err := json.Marshal(values)
				if err != nil {
					logger.Err("buffer.Ticker: Cannot marshal results to json: %s\nValues: %+v", err.Error(), values)
					return true
				}
				logger.Debug("JSON UPDATES for '%s': %+v", url, string(jsonValues))

				if len(values) > 0 {
					client := httpclient.NewTimeoutClient(15 * time.Second, 15 * time.Second)
					req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonValues))
					req.Close = true
					if err != nil {
						logger.Err("[buffer.Ticker]: failed to make new http request: %s", err.Error())
						return true
					}
					req.Header.Set("Content-Type", "application/json")
					response, err := client.Do(req)
					if err != nil {
						logger.Err("[buffer.Ticker]: Failed to make update request: %s", err.Error())
						return true
					}
					defer response.Body.Close()
					response.Close = true

					req.Body.Close()
					if response.StatusCode != http.StatusOK {
						logger.Err("[buffer.Ticker]: Update request on '%s' failed: status %d (%s)", url, response.StatusCode, response.Status)
						return true
					}
				}

				return true
			})
		}
	}
}
