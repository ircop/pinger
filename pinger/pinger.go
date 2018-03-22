package pinger

import (
	"fmt"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"net"
	"net/http"
	"pinger/httpclient"
	"pinger/logger"
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
	Pinger.ListenerLock.Lock()
	Pinger.Listener, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	Pinger.ListenerLock.Unlock()
	if err != nil {
		return err
	}

	go p.listen()

	return nil
}

// Ping - pinging host right now without any goroutines, return result
//func (p *PingDaemon) Ping(IP net.IP, probes int) (*PingResult, error) {
func (p *PingDaemon) Ping(IP fmt.Stringer, probes int) (*PingResult, error) {
	// check if there is no such running job
	if _, running := p.Jobs.Load(IP.String()); running {
		logger.Err("Ping job for '%s' is already running.", IP.String())
		return nil, fmt.Errorf("Ping job for '%s' is already running", IP.String())
	}

	job := NewJob(IP.String())
	p.Jobs.Store(IP.String(), job)
	result := job.Run(probes)

	return result, nil
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
	return p.Ping(ip, probes)
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
	if "" == url {
		// todo: other notifies?
		return
	}
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
	readBuf := make([]byte, 1500)

	for {
		n, peer, err := p.Listener.ReadFrom(readBuf)
		if err != nil {
			logger.Err("Error reading from buffer: %s", err.Error())
			continue
		}
		//bytes := readBuf[:n]
		copied := make([]byte, len(readBuf[:n]))
		copy(copied, readBuf[:n])
		go func(b []byte) {
			//logger.Debug("Message from %s", peer.String())
			host := peer.String()
			if j, found := p.Jobs.Load(host); found {

				parsed, parseErr := icmp.ParseMessage(1, b)
				if parseErr != nil {
					logger.Err("Error parsing icmp message: %s", parseErr.Error())
					//continue
					return
				}

				if parsed.Type != ipv4.ICMPTypeEchoReply {
					// non-reply message
					//logger.Debug("Non-Reply message from %s: %d; %+v", host, parsed.Type, parsed)
					logger.Debug("Non-Reply message from %s: %d; %+v\nbytes: %+v\nper byte: 0: %+v, 1: %+v, 2: %+v, 3: %+v", host, parsed.Type, parsed, b, b[0],b[1],b[2],b[3])
					//continue
					return
				}
				//logger.Debug("Reply message from %s: %d; %+v", host, parsed.Type, parsed)

				job := j.(*PingJob)
				if !job.Done {
					// parse body
					if parsed.Body != nil && parsed.Body.Len(parsed.Type.Protocol()) != 0 {
						// we need mutex to avoid writing to closed channel
						job.ChanMx.Lock()
						if !job.Done {
							//job.Reply <- parsed.Body.(*icmp.Echo).Seq
							job.Reply <- *parsed.Body.(*icmp.Echo)
						}
						job.ChanMx.Unlock()
					}
				}
			}
		}(copied)
	}
}
