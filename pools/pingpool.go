package pools

import (
	"fmt"
	"net"
	"pinger/logger"
	"pinger/pinger"
	"sync"
	"time"
)

// Hostpool is a pool of hosts :)
type Hostpool struct {
	Hosts sync.Map
	//Topics		*sync.Map
}

// Host is strcut with all host parameters and channel
type Host struct {
	IP       net.IP
	Interval time.Duration
	Probes   int
	URL      string
	Done     chan bool
	Ticker   *time.Ticker
	Finished bool

	Mx sync.Mutex
}

// Lock host mutex
func (h *Host) Lock(args ...interface{}) {
	logger.DebugLock("PingPool %s: lock %v", h.IP.String(), args)
	//logger.DebugLock("PingPool %s: lock", h.IP.String(), args)
	h.Mx.Lock()
}

// Unlock host mutex
func (h *Host) Unlock(args ...interface{}) {
	logger.DebugLock("PingPool %s: unlock %v", h.IP.String(), args)
	h.Mx.Unlock()
}

// PingPool is global Hostpool instance
var PingPool Hostpool

/*
AddHost - adding host to pool with required parameters
*/
func (p *Hostpool) AddHost(ip string, probes int, interval int64, url string) error {
	netip := net.ParseIP(ip)
	if netip == nil {
		return fmt.Errorf("Cannot parse ip '%s'", netip)
	}

	host := Host{
		IP:       netip,
		Probes:   probes,
		Interval: (time.Duration(interval) * time.Second),
		URL:      url,
		Finished: false,
	}

	if old, found := p.Hosts.Load(ip); found {
		// replace old host with new one
		logger.Debug("Removing old instance of '%s' from pool", ip)
		old.(*Host).Stop()
		p.Hosts.Delete(ip)
	}

	logger.Debug("Adding host '%s' to pool. Interval: %d", ip, interval)
	p.Hosts.Store(ip, &host)
	go host.Run()

	return nil
}

/*
Run - Start monitoring process for current host
*/
func (h *Host) Run() {
	h.Lock("Run")
	h.Done = make(chan bool)
	h.Finished = false
	h.Ticker = time.NewTicker(h.Interval)
	h.Unlock("Run")

	for {
		select {
		//default:
		//	logger.Debug("-----default------")
		//	time.Sleep(time.Second*1)
		case <-h.Ticker.C:
			// run ping
			h.Lock("<- h.Ticker.C")
			//logger.Debug("Pinging %s", h.IP.String())

			if result, err := pinger.Pinger.Ping(h.IP, h.Probes); err != nil {
				logger.Err("Failed to ping %s: %s", h.IP.String(), err.Error())
			} else {
				h.BroadcastResult(result)
			}

			h.Unlock("<- h.Ticker.C")
		case <-h.Done:
			// stop jobs
			h.Lock("<- h.Done")
			close(h.Done)
			logger.Debug("Stopping timer for '%s'", h.IP.String())
			h.Finished = true
			h.Ticker.Stop()
			h.Unlock("<- h.Done")
			return
		}
	}
}

/*
Update - update pingpool host struct in memory
 */
func (h *Host) Update(Interval int64, Probes int, URL string) {
	h.Lock()
	defer h.Unlock()
	logger.Debug("Updating host %s", h.IP.String())
	if h.Finished {
		return
	}
	//if (time.Duration(Interval) * time.Second) != h.Interval {
	if (time.Duration(Interval) * time.Second) < h.Interval {
	// todo: check if tere is several DBHosts with this ip. If one - change interval anyway, if several - use smallest.
		// Re-Add host, because we cannot update timer when it's in use
		if err := PingPool.AddHost(h.IP.String(), h.Probes, Interval, h.URL); err != nil {
			logger.Err("Host.Update: Cannot add host '%s' to PingPool: %s", h.IP.String(), err.Error())
		}
	} else {
		h.Lock("Update")
		h.Probes = Probes
		h.URL = URL
		h.Unlock("Update")
	}
}

// BroadcastResult - send result to all topic's host instances
func (h *Host) BroadcastResult(result *pinger.PingResult) {
	TopicPool.Topics.Range(func(_, topicInterface interface{}) bool {
		if host, ok := topicInterface.(*Topic).Hosts.Load(h.IP.String()); ok {
			go host.(*DBHost).Updated(*result)
		}
		return true
	})
}

// Stop - stop monitoring for current host.
func (h *Host) Stop() {
	if !h.Finished {
		h.Done <- true
	}
}
