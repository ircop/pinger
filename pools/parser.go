package pools

import (
	"fmt"
	"strings"
	"net"
	"pinger/logger"
)

/*
Constants for string variable types
 */
const (
	StrFloat64 = "float64"		// float64 string const
	StrString  = "string"		// string const
	StrMap     = "map"			// map const
	StrSlice   = "slice"		// slice const
	StrBool    = "bool"			// bool const
)

// todo: remove default url from config
// todo: remove timeout from config

// ParseTopics - parse json input (post or file contents) and return []Topic slice or error
func ParseTopics(topics map[string]interface{}, defProbes int, defInterval int64) ([]*Topic, error) {

	returnTopics := make([]*Topic, 0)

	// range over json topics
	for topicName, topicMap := range topics {
		if gettype(topicMap) != StrMap {
			return nil, fmt.Errorf("ParseTopics: %s should be map[string]interface, but %s given", topicName, gettype(topicMap))
		}

		topicMap := topicMap.(map[string]interface{})
		topic := Topic{
			Probes: defProbes,
			Interval: defInterval,
			Name: topicName,
		}

		// parse probes
		if probes, ok := topicMap["Probes"]; ok && gettype(probes) == StrFloat64 {
			topic.Probes = int(probes.(float64))
		}
		// parse interval
		if interval, ok := topicMap["Interval"]; ok && gettype(interval) == StrFloat64 {
			topic.Interval = int64(interval.(float64))
		}
		// parse url
		if url, ok := topicMap["UpdateURL"]; ok && gettype(url) == StrString {
			topic.UpdateURL = url.(string)
		}

		hosts, ok := topicMap["Hosts"]
		if ok && gettype(hosts) == StrSlice {
			// Parse hosts
			hosts, err := ParseHosts(hosts.([]interface{}), topic.Probes, topic.Interval, topic.UpdateURL)
			if err != nil {
				logger.Err("Error parsing hosts in topic '%s': %s", topicName, err.Error())
			} else {
				for n := range hosts {
					topic.Hosts.Store(hosts[n].IP.String(), hosts[n])
				}
				returnTopics = append(returnTopics, &topic)
			}
		} else {
			// Empty hosts slice
			returnTopics = append(returnTopics, &topic)
		}
	}

	return returnTopics, nil
}

// ParseHosts - parse hosts from json slice; return DBHost slice or error
func ParseHosts(hosts []interface{}, probes int, interval int64, url string) ([]*DBHost, error) {
	newHosts := make([]*DBHost, 0)

	for i, hostInt := range hosts {
		if gettype(hostInt) != "map" {
			return []*DBHost{}, fmt.Errorf("host %d is not a map", i)
		}

		newHost := DBHost{
			Probes:    probes,
			Interval:  interval,
			UpdateURL: url,
		}
		hostmap := hostInt.(map[string]interface{})

		// parse ip
		if ip, ok := hostmap["host"]; ok && gettype(ip) == StrString && net.ParseIP(ip.(string)) != nil {
			newHost.IP = net.ParseIP(ip.(string))
		} else {
			return []*DBHost{}, fmt.Errorf("wrong 'host' parameter in host %d", i)
		}

		// parse `alive`
		if aliveVal, ok := hostmap["alive"]; ok && gettype(aliveVal) == StrBool && aliveVal.(bool) {
			newHost.Alive = true
		} else {
			newHost.Alive = false
		}

		// interval
		if intVal, ok := hostmap["Interval"]; ok && gettype(intVal) == StrFloat64 {
			newHost.Interval = int64(intVal.(float64))
		}
		// probes
		if probesVal, ok := hostmap["Probes"]; ok && gettype(probesVal) == StrFloat64 {
			newHost.Probes = int(probesVal.(float64))
		}
		// URL
		if urlVal, ok := hostmap["UpdateURL"]; ok && gettype(urlVal) == StrString {
			newHost.UpdateURL = urlVal.(string)
		}

		newHosts = append(newHosts, &newHost)
	}

	return newHosts, nil
}

func gettype(variable interface{}) string {
	switch v := variable.(type) {
	default:
		t := fmt.Sprintf("%T", v)
		if strings.HasPrefix(t, "[]") {
			t = "slice"
		} else if strings.HasPrefix(t, "map") {
			t = "map"
		}
		return t
	}
}
