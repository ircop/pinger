package hostpool

import (
	"../logger"
	"../pinger"
	"sync"
	"net"
	"time"
	"errors"
	"fmt"
)

type Hostpool struct {
	Hosts		sync.Map
}

type Host struct {
	Ip			net.IP
	Interval	time.Duration
	Timeout		int64
	Probes		int
	Url			string
	Done		chan bool

	Mx			sync.Mutex
}

var Pool Hostpool

func (p *Hostpool) AddHost(ip string, probes int, interval int64, timeout int64, url string) error {
	//if interval <  30 {
	//	return errors.New(fmt.Sprintf("Interval should be 30+ seconds, %d given", interval))
	//}
	netip := net.ParseIP(ip)
	if netip == nil {
		return errors.New(fmt.Sprintf("Cannot parse ip '%s'", netip))
	}

	host := Host{
		Ip: netip,
		Probes: probes,
		Interval: time.Duration(time.Duration(interval) * time.Second),
		Timeout: timeout,
		Url: url,
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

/**
 * Start monitoring
 */
func (h *Host) Run() {
	h.Done = make(chan bool)
	timer := time.NewTicker(h.Interval)

	for {
		select {
		case <-timer.C:
			// run ping
			logger.Debug("Pinging %s", h.Ip.String())
			go pinger.Pinger.PingResultUrl(h.Ip.String(), h.Probes, h.Url)
		case <-h.Done:
			// stop jobs
			logger.Debug("Stopping timer for '%s'", h.Ip.String())
			timer.Stop()
			return
		}
	}
}

func (h *Host) Stop() {
	close(h.Done)
}
