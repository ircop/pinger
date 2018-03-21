package pools

import (
	"pinger/logger"
	"sync"
)

/*
Topic - struct for keep in memory Topic parameters and hosts
 */
type Topic struct {
	Name      string
	Probes    int
	Interval  int64
	UpdateURL string
	Mx        sync.Mutex
	Hosts     sync.Map
}

// Lock - lock topic mutex
func (t *Topic) Lock() {
	logger.DebugLock("Topic %s: lock", t.Name)
	t.Mx.Lock()
}

// Unlock - unlock topic mutex
func (t *Topic) Unlock() {
	logger.DebugLock("Topic %s: unlock", t.Name)
	t.Mx.Unlock()
}

/*
AddHost - adding host to topic
todo: check if host is alive in hostpool?
*/
func (t *Topic) AddHost(host *DBHost) {
	logger.Debug("Adding host %s to topic %s", host.IP.String(), t.Name)

	t.Hosts.Store(host.IP.String(), host)
	// add host to hostpool if it doesnt exist there
	if hp, ok := PingPool.Hosts.Load(host.IP.String()); !ok {
		if err := PingPool.AddHost(host.IP.String(), host.Probes, host.Interval, host.UpdateURL); err != nil {
			logger.Err("Topic.AddHost: Cannot add host '%s' to PingPool: %s", host.IP.String(), err.Error())
		}
		//time.Sleep(10 * time.Millisecond)
	} else {
		HP := hp.(*Host)
		if int64(HP.Interval) > host.Interval || HP.Probes != host.Probes || HP.URL != host.UpdateURL {
			HP.Update(host.Interval, host.Probes, host.UpdateURL)
		}
	}
}

/*
RemoveHost - remove host instance from current Topic.
Check for existance of this host in other topics. If not exist - remove topic host from PingPool
 */
func (t *Topic) RemoveHost(key string) {
	logger.Debug("Removing host %s from topic %s", key, t.Name)

	t.Hosts.Delete(key)
	// check this host in other topics
	found := false
	TopicPool.Topics.Range(func(topicName, topic interface{}) bool {
		if topicName.(string) == t.Name {
			return true
		}

		if _, exist := topic.(*Topic).Hosts.Load(key); exist {
			found = true
			return false
		}

		return true
	})

	// if there is no such host in other topics, remove it from hostpool
	if !found {
		if host, ok := PingPool.Hosts.Load(key); ok {
			logger.Debug("Removing host %s from hostpool", key)
			host.(*Host).Stop()
			PingPool.Hosts.Delete(key)
		}
	}
}
