package pinger

import (
	"../logger"
	"golang.org/x/net/icmp"
	"sync"
	"net"
	"fmt"
	"errors"
	"golang.org/x/net/ipv4"
	"strings"
	"../httpclient"
	"net/http"
)

type PingDaemon struct {
	Listener		*icmp.PacketConn
	ListenerLock	sync.Mutex
	Jobs			sync.Map
}

var Pinger PingDaemon

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

func (p *PingDaemon) PingNow(host string, probes int) (*PingResult, error) {
	// find valid ip
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			logger.Err("'%s' is not ip, but also cannot resolve it with dns.", host)
			return nil, errors.New(fmt.Sprintf("'%s' is not ip, but also cannot resolve it with dns.", host))
		} else {
			ip = ips[0]
		}
	}

	// check if there is no such running job
	if _, running := p.Jobs.Load(ip.String()); running {
		logger.Err("Ping job for '%s' is already running.", ip.String())
		return nil, errors.New(fmt.Sprintf("Ping job for '%s' is already running.", ip.String()))
	}

	job := NewJob(ip.String())
	p.Jobs.Store(ip.String(), job)
	result := job.Run(probes)

	return result, nil
}

func (p *PingDaemon) PingResultUrl(host string, probes int, url string) {
	// find valid ip
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			logger.Err("'%s' is not ip, but also cannot resolve it with dns.", host)
			return
		} else {
			ip = ips[0]
		}
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


	return
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