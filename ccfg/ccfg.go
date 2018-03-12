package ccfg

import (
	"github.com/spf13/viper"
)

type Cfg struct {
	ListenIp		string
	ListenPort		string
	Ssl				bool
	SslCert			string
	SslKey			string
	LogPath			string
	LogDebug		bool
	ResultUrl		string
	DefaultProbes	int
	DefaultTimeout	int64
}

func New(path *string) *Cfg {

	viper.SetConfigFile(*path)
	err := viper.ReadInConfig()
	if err != nil {
		panic(err.Error())
	}

	c := new (Cfg)
	viper.SetDefault("listen.ip", "0.0.0.0")
	viper.SetDefault("listen.port", "1081")
	viper.SetDefault("listen.ssl", false)
	viper.SetDefault("log.path", "/var/log/pinger.log")
	viper.SetDefault("log.debug", true)
	viper.SetDefault("pinger.default-probes", 3)

	c.ListenIp = viper.GetString("listen.ip")
	c.ListenPort = viper.GetString("listen.port")
	c.Ssl = viper.GetBool("listen.ssl")
	c.SslCert = viper.GetString("listen.cert")
	c.SslKey = viper.GetString("listen.key")

	c.LogPath = viper.GetString("log.path")
	c.LogDebug = viper.GetBool("log.debug")

	c.ResultUrl = viper.GetString("pinger.result-url")
	c.DefaultProbes = viper.GetInt("pinger.default-probes")
	c.DefaultTimeout = viper.GetInt64("pinger.default-timeout")

	// if ssl is enabled, cert & key must exist
	if( c.Ssl ) {
		if( c.SslKey == "" || c.SslCert == "" ) {
			panic("SSL Key and cert must be defined if SSL is enabled!")
		}
	}

	return c
}
