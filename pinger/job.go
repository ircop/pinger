package pinger

import (
	"../logger"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"net"
	"os"
	"sync"
	"time"
)

/*
PingJob is struct for handling pinger jobs
 */
type PingJob struct {
	Host    string
	Started time.Time
	Reply   chan int

	Done   bool
	ChanMx sync.Mutex
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

// Run - runs ping job for host and returns PingResult
func (j *PingJob) Run(maxProbes int) *PingResult {
	j.Reply = make(chan int)

	probes := make([]PingProbe, 0)

	seq := 0
	lastProbeStart := j.sendEcho(seq)

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	for {
		select {
		case replySeq := <-j.Reply:
			// do something only if this is current probe
			if replySeq == seq {
				timer.Reset(2 * time.Second)
				seq++

				// todo: write result somewhere
				latency := time.Since(lastProbeStart)
				//latencyNs := latency.Nanoseconds()
				//latencyMs := float64(latencyNs) / float64(1000000)
				//logger.Debug("Got reply from %s in %d ns / %f ms, sequence %d", j.Host, latencyNs, latencyMs, seq)
				probe := PingProbe{RttNs: latency.Nanoseconds(), Success: true}
				probes = append(probes, probe)

				if seq == maxProbes {
					//logger.Debug("Finishing job")
					j.ChanMx.Lock()
					close(j.Reply)
					j.Done = true
					Pinger.Jobs.Delete(j.Host)
					j.ChanMx.Unlock()
					return j.Result(probes)
				}

				lastProbeStart = j.sendEcho(seq)
			}
		case <-timer.C:
			//logger.Debug("Timeout waiting reply from %s", j.Host )
			seq++
			probe := PingProbe{RttNs: 0, Success: false}
			probes = append(probes, probe)
			if seq == maxProbes {
				//logger.Debug("Finishing job")
				j.ChanMx.Lock()
				close(j.Reply)
				j.Done = true
				Pinger.Jobs.Delete(j.Host)
				j.ChanMx.Unlock()
				return j.Result(probes)
			}
			timer.Reset(2 * time.Second)
		}
	}
}

// Result makes new Result instance
func (j *PingJob) Result(probes []PingProbe) *PingResult {
	result := PingResult{Alive: false, AvgRttMs: 0, AvgRttNs: 0, SuccessPercent: 0}

	var successProbes int64
	var sumRtt int64
	for _, probe := range probes {
		if probe.Success {
			result.Alive = true
			successProbes++
			sumRtt += probe.RttNs
		}
	}

	if successProbes > 0 {
		result.AvgRttNs = sumRtt / successProbes
		result.AvgRttMs = float64(result.AvgRttNs) / float64(1000000)
		result.SuccessPercent = (float64(100) / float64(len(probes))) * float64(successProbes)
	}

	return &result
}

func (j *PingJob) sendEcho(num int) time.Time {
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: num,
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
