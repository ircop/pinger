package pinger

import (
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"net"
	"pinger/logger"
	"sync"
	"time"
	"math/rand"
)

/*
PingJob is struct for handling pinger jobs
*/
type PingJob struct {
	Host    		string
	Started 		time.Time

	Reply   		chan icmp.Echo
	stopListen		chan bool
	pingReplies		chan map[int]time.Time

	Done   			bool
	ChanMx 			sync.Mutex
}

/*
PingResult is struct with ping results
*/
type PingResult struct {
	Alive          bool
	SuccessPercent float64
	//Probes			[]PingProbe
	AvgRttNs int64
	AvgRttMs float64
}

/*
PingProbe is struct with one of [n] resulting probes
*/
type PingProbe struct {
	Success bool
	RttNs   int64
}

// NewJob returns new PingJob instance
// host: string with pinging host address
func NewJob(host string) *PingJob {
	job := PingJob{
		Host:    host,
		Started: time.Now(),
	}

	return &job
}

func (j *PingJob) listenReplies(pingID int) {
	// Keep sequences & ping reply times in map
	replies := make(map[int]time.Time)		// map[sequence]replyTime
	//logger.Debug("%s: start listenReplies", j.Host)
	for {
		select {
		case echoReply := <-j.Reply:
			//logger.Debug("%s (Run): got reply", j.Host)
			// not our ping
			if echoReply.ID != pingID {
				//logger.Debug("%s: Wrong ping ID: %d, waiting %d", j.Host, echoReply.ID, pingID)
				continue
			}
			replies[echoReply.Seq] = time.Now()
			//logger.Debug("%s: increased map: %+v", replies)
		case <-j.stopListen:
			// stop loop; return replies
			j.pingReplies <- replies
			return
		}
	}
}

/*
Run - start ping job for host.
@param maxProbes int - number of ping probes to send
 */
func (j *PingJob) Run(maxProbes int) *PingResult {
	// init channels
	j.Reply = make(chan icmp.Echo)
	j.stopListen = make(chan bool)
	j.pingReplies = make(chan map[int]time.Time)

	// Generate Ping ID
	pingID := (rand.Intn(9998) + 1)

	// run listener
	go j.listenReplies(pingID)

	echos := make(map[int]time.Time)
	for i := 0; i < maxProbes; i++ {
		tm := j.sendEcho(i+1, pingID)
		echos[i+1] = tm
		time.Sleep(time.Second * 2)
	}

	// send stop signal
	close(j.stopListen)
	// wait for replies from goroutine
	replies := <- j.pingReplies
	// on receive, close all channels (since listener shouldn't use them more)
	close(j.pingReplies)

	probes := make([]PingProbe, 0)
	for seq, sentTime := range echos {
		probe := PingProbe{}
		if rcvTime, ok := replies[seq]; ok {
			latency := rcvTime.Sub(sentTime)
			probe.RttNs = latency.Nanoseconds()
			probe.Success = true
		} else {
			probe.Success = false
		}
		probes = append(probes, probe)
	}

	// Remove job from queue
	j.ChanMx.Lock()
	j.Done = true
	Pinger.Jobs.Delete(j.Host)
	j.ChanMx.Unlock()

	return j.Result(probes)
}

// Result makes new Result instance
func (j *PingJob) Result(probes []PingProbe) *PingResult {
	result := PingResult{Alive: false, AvgRttMs: 0, AvgRttNs: 0, SuccessPercent: 0}

	successProbes := 0
	var sumRtt int64
	for _, probe := range probes {
		if probe.Success {
			result.Alive = true
			successProbes++
			sumRtt += probe.RttNs
		}
	}

	if successProbes > 0 {
		result.AvgRttNs = sumRtt / int64(successProbes)
		result.AvgRttMs = float64(result.AvgRttNs) / float64(1000000)
		result.SuccessPercent = (float64(100) / float64(len(probes))) * float64(successProbes)
	}

	return &result
}

func (j *PingJob) sendEcho(seq int, id int) time.Time {
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: id & 0xffff, Seq: seq,
			Data: []byte("HELO"),
		},
	}

	writebuf, err := msg.Marshal(nil)
	if err != nil {
		logger.Err("Cannot marshal writebuf msg: %s", err.Error())
		return time.Now()
	}

	Pinger.ListenerLock.Lock()
	if _, err := Pinger.Listener.WriteTo(writebuf, &net.IPAddr{IP: net.ParseIP(j.Host)}); err != nil {
		logger.Err("Cannot send echo request: %s", err.Error())
	}
	defer Pinger.ListenerLock.Unlock()

	return time.Now()
}
