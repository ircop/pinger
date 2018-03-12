package httpclient

import (
	"net"
	"net/http"
	"time"
)

type HttpConfig struct {
	ConnectTimeout		time.Duration
	RwTimeout			time.Duration
}

func TimeoutDialer(config *HttpConfig) func (net, addr string) (c net.Conn, err error) {
	return func(netw, addr string) (net.Conn, error) {
		conn, err := net.DialTimeout(netw, addr, config.ConnectTimeout)
		if err != nil {
			return nil, err
		}
		conn.SetDeadline(time.Now().Add(config.RwTimeout))
		return conn, nil
	}
}

func NewTimeoutClient(args ...interface{}) *http.Client {
	config := &HttpConfig{
		ConnectTimeout: 1 * time.Second,
		RwTimeout: 1 * time.Second,
	}

	if len(args) == 1 {
		timeout := args[0].(time.Duration)
		config.ConnectTimeout = timeout
		config.RwTimeout = timeout
	}

	if len(args) == 2 {
		config.ConnectTimeout = args[0].(time.Duration)
		config.RwTimeout = args[1].(time.Duration)
	}

	return &http.Client{
		Transport: &http.Transport{
			Dial: TimeoutDialer(config),
		},
	}
}
