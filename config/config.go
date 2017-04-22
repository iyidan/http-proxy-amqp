package config

import (
	"errors"
	"flag"

	"path/filepath"

	"github.com/iyidan/http-proxy-amqp/jsonconf"
	"github.com/iyidan/http-proxy-amqp/util"
)

// Config is the config of amqp client and has some custom options
type Config struct {
	// DSN is the amqp address
	// The format is amqp://user:password@host:port/vhost
	// such as amqp://iyidan:123456@127.0.0.1:5672//test
	// Notice: user/password must be urlencoded if necessary
	// Notice: vhost can have a leading slash
	DSN string `json:"dsn"`

	MaxChannelsPerConnection int `json:"maxChannelsPerConnection"`
	MaxIdleChannels          int `json:"maxIdleChannels"`
	MaxConnections           int `json:"maxConnections"`
	MinConnections           int `json:"minConnections"`

	// http api listen address
	HTTPListenAddr string `json:"httpListenAddr"`

	Debug bool `json:"debug"`
}

var (
	defaultMaxChannelsPerConnection = 20000
	defaultMaxIdleChannels          = 500
	defaultMaxConnections           = 2000
	defaultMinConnections           = 5
	defaultHTTPListenAddr           = "127.0.0.1:35673"
)

func getDefaultConfig() *Config {
	return &Config{
		DSN: "",
		MaxChannelsPerConnection: defaultMaxChannelsPerConnection,
		MaxIdleChannels:          defaultMaxIdleChannels,
		MaxConnections:           defaultMaxConnections,
		MinConnections:           defaultMinConnections,
		HTTPListenAddr:           defaultHTTPListenAddr,
		Debug:                    false,
	}
}

var (
	flagCfgFile                  = flag.String("config", "", "The config file")
	flagDSN                      = flag.String("dsn", "", "The amqp address")
	flagMaxChannelsPerConnection = flag.Int("maxChannelsPerConnection", 0, "The max channels per connection")
	flagMaxIdleChannels          = flag.Int("maxIdleChannels", 0, "The max idle channels for this process")
	flagMaxConnections           = flag.Int("maxConnections", 0, "The max connections for this process")
	flagMinConnections           = flag.Int("minConnections", 0, "The min connections keeped for this process")
	flagHTTPListenAddr           = flag.String("httpListenAddr", "", "http api listen address")
	flagDebug                    = flag.Bool("debug", false, "if true, will print pool stats per requests")
)

// InitConfig init the current process config
func InitConfig() *Config {

	if !flag.Parsed() {
		flag.Parse()
	}

	cfg := getDefaultConfig()
	if *flagCfgFile != "" {
		if !filepath.IsAbs(*flagCfgFile) {
			*flagCfgFile = filepath.Join(util.GetRootPath(), *flagCfgFile)
		}
		err := jsonconf.ParseJSONFile(*flagCfgFile, cfg)
		if err != nil {
			util.FailOnError(err, "parse config file failed")
		}
	}
	// command line args is high priority
	if *flagDSN != "" {
		cfg.DSN = *flagDSN
	}
	if *flagMaxChannelsPerConnection > 0 {
		cfg.MaxChannelsPerConnection = *flagMaxChannelsPerConnection
	}
	if *flagMaxIdleChannels > 0 {
		cfg.MaxIdleChannels = *flagMaxIdleChannels
	}
	if *flagMaxConnections > 0 {
		cfg.MaxConnections = *flagMaxConnections
	}
	if *flagMinConnections > 0 {
		cfg.MinConnections = *flagMinConnections
	}
	if *flagHTTPListenAddr != "" {
		cfg.HTTPListenAddr = *flagHTTPListenAddr
	}
	if *flagDebug {
		cfg.Debug = *flagDebug
	}

	CheckConfig(cfg)

	return cfg
}

// CheckConfig validate the given cfg
func CheckConfig(cfg *Config) {
	if cfg.DSN == "" {
		util.FailOnError(errors.New("config.DSN empty"), "initConfig")
	}
	if cfg.HTTPListenAddr == "" {
		util.FailOnError(errors.New("config.HTTPListenAddr empty"), "initConfig")
	}

	if cfg.MaxChannelsPerConnection <= 0 ||
		cfg.MaxConnections <= 0 ||
		cfg.MaxIdleChannels <= 0 ||
		cfg.MinConnections <= 0 {
		util.FailOnError(errors.New("config.MaxChannelsPerConnection/MaxConnections/MaxIdleChannels less than 1"), "initConfig")
	}
}
