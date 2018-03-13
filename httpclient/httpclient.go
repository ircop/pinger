package httpclient

import (
	"net"
	"net/http"
	"time"
)

// HTTPConfig is struct for handling http client configuration
type HTTPConfig struct {
	ConnectTimeout time.Duration
	RwTimeout      time.Duration
}

// TimeoutDialer returns net.Conn with timeout set
func TimeoutDialer(config *HTTPConfig) func(net, addr string) (c net.Conn, err error) {
	return func(netw, addr string) (net.Conn, error) {
		conn, err := net.DialTimeout(netw, addr, config.ConnectTimeout)
		if err != nil {
			return nil, err
		}
		if err := conn.SetDeadline(time.Now().Add(config.RwTimeout)); err != nil {
			return nil, err
		}
		return conn, nil
	}
}

// NewTimeoutClient returns custom http(s) client with timeouts set
func NewTimeoutClient(args ...interface{}) *http.Client {
	config := &HTTPConfig{
		ConnectTimeout: 1 * time.Second,
		RwTimeout:      1 * time.Second,
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
