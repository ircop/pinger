package pinger

import (
	"../httpclient"
	"../logger"
	"fmt"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"net"
	"net/http"
	"strings"
	"sync"
)

// PingDaemon is a global and unique struct for our pinger
type PingDaemon struct {
	Listener     *icmp.PacketConn
	ListenerLock sync.Mutex
	Jobs         sync.Map
}

// Pinger is PingDaemon instance
var Pinger PingDaemon

// Init - initializing pinger daemon; starting listener
func (p *PingDaemon) Init() error {
	logger.Debug("Starting pinger instance")

	// start listener
	var err error
	Pinger.Listener, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return err
	}

	go p.listen()

	return nil
}

// PingNow - pings host and returns result without any goroutines
func (p *PingDaemon) PingNow(host string, probes int) (*PingResult, error) {
	// find valid ip
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			logger.Err("'%s' is not ip, but also cannot resolve it with dns.", host)
			return nil, fmt.Errorf("'%s' is not ip, but also cannot resolve it with dns", host)
		}

		ip = ips[0]
	}

	// check if there is no such running job
	if _, running := p.Jobs.Load(ip.String()); running {
		logger.Err("Ping job for '%s' is already running.", ip.String())
		return nil, fmt.Errorf("Ping job for '%s' is already running", ip.String())
	}

	job := NewJob(ip.String())
	p.Jobs.Store(ip.String(), job)
	result := job.Run(probes)

	return result, nil
}

// PingResultURL - pings host in goroutine and sends result to result URL
func (p *PingDaemon) PingResultURL(host string, probes int, url string) {
	// find valid ip
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			logger.Err("'%s' is not ip, but also cannot resolve it with dns.", host)
			return
		}
		ip = ips[0]
	}

	// check if there is no such running job
	if _, running := p.Jobs.Load(ip.String()); running {
		logger.Err("Ping job for '%s' is already running.", ip.String())
		return
	}

	job := NewJob(ip.String())
	p.Jobs.Store(ip.String(), job)
	result := job.Run(probes)

	// form url
	url = strings.Replace(url, `{host}`, host, -1)
	if result.Alive {
		url = strings.Replace(url, `{alive}`, "true", -1)
	} else {
		url = strings.Replace(url, `{alive}`, "false", -1)
	}
	url = strings.Replace(url, `{ns}`, fmt.Sprintf("%d", result.AvgRttNs), -1)
	url = strings.Replace(url, `{ms}`, fmt.Sprintf("%f", result.AvgRttMs), -1)
	logger.Debug("API CALL: %s", url)

	client := httpclient.NewTimeoutClient()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Err("Error creating request: %s", err.Error())
		return
	}

	_, err = client.Do(req)
	if err != nil {
		logger.Err("Error requesting api: %s", err.Error())
		return
	}
}

func (p *PingDaemon) listen() {
	// todo: recover
	readBuf := make([]byte, 1600)

	for {
		n, peer, err := p.Listener.ReadFrom(readBuf)
		if err != nil {
			logger.Err("Error reading from buffer: %s", err.Error())
			continue
		}
		host := peer.String()
		if j, found := p.Jobs.Load(host); found {

			parsed, parseErr := icmp.ParseMessage(1, readBuf[:n])
			if parseErr != nil {
				logger.Err("Error parsing icmp message: %s", parseErr.Error())
				continue
			}

			if parsed.Type != ipv4.ICMPTypeEchoReply {
				// non-reply message
				//logger.Debug("Non-Reply message from %s", host)
				continue
			}
			//logger.Debug("Message from %s", host)

			job := j.(*PingJob)
			if !job.Done {
				// parse body
				if parsed.Body != nil && parsed.Body.Len(parsed.Type.Protocol()) != 0 {
					// we need mutex to avoid writing to closed channel
					job.ChanMx.Lock()
					if !job.Done {
						job.Reply <- parsed.Body.(*icmp.Echo).Seq
					}
					job.ChanMx.Unlock()
				}
			}
		}
	}
}
