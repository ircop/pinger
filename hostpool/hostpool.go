package hostpool

import (
	"../logger"
	"../pinger"
	"fmt"
	"net"
	"sync"
	"time"
)

// Hostpool is a pool of hosts :)
type Hostpool struct {
	Hosts sync.Map
}

// Host is strcut with all host parameters and channel
type Host struct {
	IP       net.IP
	Interval time.Duration
	Timeout  int64
	Probes   int
	URL      string
	Done     chan bool

	Mx sync.Mutex
}

// Pool is global Hostpool instance
var Pool Hostpool

/*
AddHost - adding host to pool with required parameters
 */
func (p *Hostpool) AddHost(ip string, probes int, interval int64, timeout int64, url string) error {
	netip := net.ParseIP(ip)
	if netip == nil {
		return fmt.Errorf("Cannot parse ip '%s'", netip)
	}

	host := Host{
		IP:       netip,
		Probes:   probes,
		//Interval: time.Duration(time.Duration(interval) * time.Second),
		Interval: (time.Duration(interval) * time.Second),
		Timeout:  timeout,
		URL:      url,
	}

	if old, found := p.Hosts.Load(ip); found {
		// replace old host with new one
		logger.Debug("Removing old instance of '%s' from pool", ip)
		old.(*Host).Stop()
		p.Hosts.Delete(ip)
	}

	logger.Debug("Adding host '%s' to pool", ip)
	p.Hosts.Store(ip, &host)
	go host.Run()

	return nil
}

/*
Run - Start monitoring process for current host
 */
func (h *Host) Run() {
	h.Done = make(chan bool)
	timer := time.NewTicker(h.Interval)

	for {
		select {
		case <-timer.C:
			// run ping
			logger.Debug("Pinging %s", h.IP.String())
			go pinger.Pinger.PingResultURL(h.IP.String(), h.Probes, h.URL)
		case <-h.Done:
			// stop jobs
			logger.Debug("Stopping timer for '%s'", h.IP.String())
			timer.Stop()
			return
		}
	}
}

/*
Stop - stop monitoring for current host.
 */
func (h *Host) Stop() {
	close(h.Done)
}
