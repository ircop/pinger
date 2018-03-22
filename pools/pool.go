package pools

import (
	"pinger/logger"
	"sync"
	"encoding/json"
	"io/ioutil"
	"time"
)

// DBPool - inmemory pool of topics
type DBPool struct {
	Topics 			sync.Map
	SavePath		string
	SaveInterval	int64

	Mx     sync.Mutex
}

// Lock - lock pool mutex
func (p *DBPool) Lock() {
	logger.DebugLock("topicPool: lock")
	p.Mx.Lock()
}

// Unlock - unlock pool mutex
func (p *DBPool) Unlock() {
	logger.DebugLock("topicPool: unlock")
	p.Mx.Unlock()
}

// TopicPool - single global instance of Topic pool
var TopicPool = DBPool{}

/*
StartSaver - if config savepath and saveInterval are set, start "saver" vorker for saving host states each N seconds
 */
func (p *DBPool) StartSaver() {
	sec := p.SaveInterval
	ticker := time.NewTicker(time.Duration(sec) * time.Second)
	for {
		select {
		case <- ticker.C:
			p.Save()
		}
	}
}

/*
Init - initialize global pool.
Load hosts from stored file.
*/
func (p *DBPool) Init(savePath string, saveInterval int64, defaultProbes int, defaultInterval int64) {
	p.SaveInterval = saveInterval
	p.SavePath = savePath

	// read saved hosts from hdd if file set and exist
	if p.SavePath != "" {
		contents, err := ioutil.ReadFile(p.SavePath)
		if err != nil {
			logger.Err("Cannot read saved hosts: %s", err.Error())
			go p.StartSaver()
			return
		}

		jsonParams := make(map[string]interface{})
		//jsonErr := json.Unmarshal([]byte(contents), &jsonParams)
		jsonErr := json.Unmarshal(contents, &jsonParams)
		if jsonErr != nil {
			logger.Err("Cannot parse saved hosts: %s", jsonErr.Error())
			return
		}

		topics, err := ParseTopics(jsonParams, defaultProbes, defaultInterval)
		if err != nil {
			logger.Err("Cannot parse saved hosts: %s", err.Error())
			return
		}

		loadedTopics := 0
		loadedHosts := 0
		TopicPool.Lock()
		for n := range topics {
			loadedTopics++
			// get topic len
			topics[n].Hosts.Range(func (k, v interface{}) bool {
				loadedHosts++
				return true
			})
			TopicPool.Topics.Store(topics[n].Name, topics[n])
		}
		TopicPool.Unlock()

		logger.Log("Loaded %d topics and %d hosts from '%s'", loadedTopics, loadedHosts, savePath)

		go p.StartSaver()
	}
}
// Save - create json object from all of the topics and hosts and store it to file
func (p *DBPool) Save() {
	if p.SavePath == "" {
		return
	}
	TopicPool.Lock()
	defer TopicPool.Unlock()

	topics := make(map[string]interface{})
	TopicPool.Topics.Range(func (k, v interface{}) bool {
		topic := v.(*Topic)

		sTopic := make(map[string]interface{})
		sTopic["Name"] = k.(string)
		sTopic["Probes"] = topic.Probes
		sTopic["Interval"] = topic.Interval
		sTopic["UpdateURL"] = topic.UpdateURL
		hosts := make([]map[string]interface{}, 0)
		v.(*Topic).Hosts.Range(func(hk, h interface{}) bool {
			host := h.(*DBHost)
			host.Lock("Save")
			sHost := make(map[string]interface{})
			sHost["host"] = host.IP.String()
			if host.Probes != topic.Probes {
				sHost["Probes"] = host.Probes
			}
			if host.Interval != topic.Interval {
				sHost["Interval"] = host.Interval
			}
			if host.UpdateURL != topic.UpdateURL {
				sHost["UpdateURL"] = host.UpdateURL
			}
			sHost["alive"] = host.Alive
			hosts = append(hosts, sHost)
			host.Unlock("Save")
			return true
		})
		sTopic["Hosts"] = hosts
		topics[k.(string)] = sTopic
		return true
	})

	bytes, e := json.Marshal(topics)
	if e != nil {
		logger.Err("Cannot marshal topics during Save()")
		return
	}

	err := ioutil.WriteFile(p.SavePath, bytes, 0644)
	if err != nil {
		logger.Err("Cannot save hosts: %s", err.Error())
	} else {
		logger.Debug("Hosts saved")
	}
}

/*
GetOrStore - compare given topics with existing ones; modify if needed
Add new hosts and remove old ones
Returns map[hostname]alive(bool)
*/
func (p *DBPool) GetOrStore(topics []*Topic, removeOld bool) map[string]map[string]bool {
	returnTopics := make(map[string]map[string]bool)

	TopicPool.Lock()
	// loop over "new" topics
	for _, newTopic := range topics {
		// determine if newTopic already exists
		if _, ok := p.Topics.Load(newTopic.Name); ok {
			// exists
		} else {
			// This is a new topic. We should create new topic with all given hosts
			topic := Topic{
				Name:      newTopic.Name,
				UpdateURL: newTopic.UpdateURL,
				Interval:  newTopic.Interval,
				Probes:    newTopic.Probes,
			}
			TopicPool.Topics.Store(newTopic.Name, &topic)
		}

		// topic is found or created, it's 100% exist
		topic, _ := TopicPool.Topics.Load(newTopic.Name)
		returnTopics[newTopic.Name] = p.CompareTopic(newTopic, topic.(*Topic), removeOld)
	}

	TopicPool.Unlock()
	return returnTopics
}

/*
CompareTopic - compare topic contents; delete expired hosts; create new hosts
Return map[ip]bool(alive)
*/
func (p *DBPool) CompareTopic(newTopic *Topic, oldTopic *Topic, removeOld bool) map[string]bool {
	topicHosts := make(map[string]bool)

	// loop trough newTopic hosts and add them to the pool if needed
	oldTopic.Lock()
	defer oldTopic.Unlock()

	if oldTopic.UpdateURL != newTopic.UpdateURL {
		oldTopic.UpdateURL = newTopic.UpdateURL
	}
	if oldTopic.Interval != newTopic.Interval {
		oldTopic.Interval = newTopic.Interval
	}
	if oldTopic.Probes != newTopic.Probes {
		oldTopic.Probes = newTopic.Probes
	}

	newTopic.Hosts.Range(func(key, newHost interface{}) bool {
		oldHost, exist := oldTopic.Hosts.Load(key.(string))
		if exist {
			// old host exist. update params (if needed);
			// todo: store host results in some variable
			topicHosts[oldHost.(*DBHost).IP.String()] = oldHost.(*DBHost).Alive
			p.UpdateHost(newHost.(*DBHost), oldHost.(*DBHost))
		} else {
			// There is no such host
			// 1) add host to topic ; 2) add host to hostpool (if needed)
			//p.AddHost(newHost.(*Host), oldTopic)
			topicHosts[newHost.(*DBHost).IP.String()] = newHost.(*DBHost).Alive
			oldTopic.AddHost(newHost.(*DBHost))
		}
		return true
	})

	// 2: loop over old topic and remove non-existing in newTopic hosts
	if removeOld {
		oldTopic.Hosts.Range(func(key, oldHost interface{}) bool {
			if _, exist := newTopic.Hosts.Load(key.(string)); !exist {
				// remove host from oldhosts; remove host from hostpool if no such host in other topics
				oldTopic.RemoveHost(key.(string))
			} else {
				topicHosts[key.(string)] = oldHost.(*DBHost).Alive
			}
			return true
		})
	}

	return topicHosts
}

/*
UpdateHost - updating host (if needed)
oldHost is host in TopicPool, not in hostpool
todo: return oldhost results/stats/status
*/
func (p *DBPool) UpdateHost(newHost *DBHost, oldHost *DBHost) {
	newHost.Lock("UpdateHost (newHost)")
	oldHost.Lock("UpdateHost (oldHost)")

	// todo: update interval only if 1) this host is in multiple topics AND new interval < old interval 2) this host is in only one topic
	if newHost.Interval < oldHost.Interval || newHost.Probes != oldHost.Probes || newHost.UpdateURL != oldHost.UpdateURL || newHost.Alive != oldHost.Alive {
		logger.Debug("updating oldHost")
		oldHost.Interval = newHost.Interval
		oldHost.Probes = newHost.Probes
		oldHost.UpdateURL = newHost.UpdateURL
		oldHost.Alive = newHost.Alive

		// find and update host in hostpool
		hp, ok := PingPool.Hosts.Load(oldHost.IP.String())
		if !ok {
			// todo: something wrong, but anyway add host
			if err := PingPool.AddHost(newHost.IP.String(), newHost.Probes, newHost.Interval, newHost.UpdateURL); err != nil {
				logger.Err("DBPool.UpdateHost: Cannot add host '%s' to PingPool: %s", newHost.IP.String(), err.Error())
			}
		} else {
			hp.(*Host).Update(oldHost.Interval, oldHost.Probes, oldHost.UpdateURL)
		}
	}

	oldHost.Unlock("UpdateHost (oldHost)")
	newHost.Unlock("UpdateHost (newHost)")

}
