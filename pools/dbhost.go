package pools

import (
	"net"
	"pinger/logger"
	"pinger/pinger"
	"pinger/notify"
	"sync"
)

/*
DBHost - struct for per-topic in-memory instances for host.
Can be multiple DBHosts for each pinged host (in different topics)
 */
type DBHost struct {
	IP        net.IP
	Probes    int
	Timeout   int64
	Interval  int64
	UpdateURL string
	Mx        sync.Mutex
	Alive     bool
}

// Lock - lock host mutex; write log
func (h *DBHost) Lock(where string) {
	logger.DebugLock("%s: DBHost:Lock() | %s", h.IP.String(), where)
	h.Mx.Lock()
}

// Unlock - unlock host mutex; write debug log
func (h *DBHost) Unlock(where string) {
	logger.DebugLock("%s: DBHost:Unlock() | %s", h.IP.String(), where)
	h.Mx.Unlock()
}

/*
Updated - called from pinger when host state is determined: true of false
 */
func (h *DBHost) Updated(result pinger.PingResult) {
	// todo: send update via UpdateURL
	// todo: send udpates to telegram bot (todo: make telegram api)
	h.Lock("Update")
	if result.Alive != h.Alive {
		logger.Debug("[DBHost]: %s: state changed: %v", h.IP.String(), result.Alive)
		h.Alive = result.Alive
		if "" != h.UpdateURL {
			notify.Buffer.BufferResult(h.UpdateURL, h.IP.String(), result)
		}
	} // else {
	//	//logger.Debug("[DBHost]: %s: state not changed: %v", h.IP.String(), result.Alive)
	//}
	h.Unlock("Update")
}
