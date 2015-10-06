package common

import (
	"os"
	"time"
	"bufio"
	"bytes"
	"io/ioutil"
	"path/filepath"

	"github.com/BurntSushi/toml"
	log "github.com/Sirupsen/logrus"
)


type Config struct {
	BaseConfig
	ModTime time.Time `json:"-"`
	Loaded  bool      `json:"-"`
}

type Server struct {
	Url           string
	Weight        int
	Type          string
}

type BaseConfig struct {
	ApiAddress                     string  `toml:"api-address"`
	LbApiLogin                     string  `toml:"api-login"`
	LbApiPassword                  string  `toml:"api-password"`
	LbAddress                      string  `toml:"lb-address"`
	LbLogFile                      string  `toml:"lb-log-file"`
	LbEnableTace                   bool    `toml:"enable-trace"`
	LbTaceFile                     string  `toml:"trace-file"`
	LbEnableRebalancer             bool    `toml:"enable-rebalancer"`
	LbMirroringMethods             string  `toml:"mirror-http-methods"`
	LbMirroringEnabled             bool    `toml:"enable-mirroring"`

	LbStreamRetryConditions        string  `toml:"stream-retry-conditions"`
	LbMonitorBrokenBackends        bool    `toml:"monitor-broken-backend"`
	LbMonitorRemoveBrokenBackends  bool    `toml:"remove-broken-backends"`
	LbStats                        bool    `toml:"statistics-enabled"`
	LbMonitorCheckPeriod           int     `toml:"check-period"`
	LbMonitorMaxFails              int     `toml:"max-fails"`
	LbMonitorFailTimeout           int     `toml:"fail-timeout"`
	LbMonitorBashScript            string  `toml:"bash-script"`
	LbMonitorWebUrl                string  `toml:"web-url"`

	LbEnableConnlimit              bool    `toml:"enable-connlimit"`
	LbConnlimitConnections         int     `toml:"connlimit-connections"`
	LbConnlimitVariable            string  `toml:"connlimit-variable"`

	LbEnableRatelimit              bool    `toml:"enable-ratelimit"`
	LbRatelimitRequests            int     `toml:"ratelimit-requests"`
	LbRatelimitPeriodSeconds       int     `toml:"ratelimit-period-seconds"`
	LbRatelimitBurst               int     `toml:"ratelimit-burst"`
	LbRatelimitVariable            string  `toml:"ratelimit-variable"`

	Servers    map[string]Server
}

func NewConfig() *Config {
	return &Config{
		BaseConfig: BaseConfig{
			ApiAddress: "127.0.0.1:9090",
			LbApiLogin: "lb",
			LbApiPassword: "7eNQ4iWLgDw4Q6w",
			LbAddress: "127.0.0.1:8080",
			LbLogFile: "",
			LbEnableTace: false,
			LbTaceFile: "/tmp/lb.trace.log",
			LbMirroringMethods: "GET|HEAD",
			LbMirroringEnabled: false,

			LbStreamRetryConditions: `IsNetworkError() && Attempts() < 10`,
			LbMonitorBrokenBackends: false,
			LbMonitorRemoveBrokenBackends: true,
			LbEnableRebalancer: true,
			LbMonitorCheckPeriod: 1,
			LbMonitorMaxFails: 10,
			LbMonitorFailTimeout: 10,
			LbMonitorBashScript: "",
			LbMonitorWebUrl: "",
			LbStats: false,
			LbEnableConnlimit: false,
			LbConnlimitConnections: 10,
			LbConnlimitVariable: `client.ip`,
			LbEnableRatelimit: false,
			LbRatelimitRequests: 1,
			LbRatelimitPeriodSeconds: 1,
			LbRatelimitBurst: 3,
			LbRatelimitVariable: `client.ip`,

			Servers: make(map[string]Server),
		},
	}
}

func (c *Config) StatConfig(configFile string) error {
	_, err := os.Stat(configFile)
	if err != nil {
		return err
	}
	return nil
}

func (c *Config) LoadConfig(configFile string) error {
	info, err := os.Stat(configFile)

	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	if _, err = toml.DecodeFile(configFile, &c.BaseConfig); err != nil {
		return err
	}

	c.ModTime = info.ModTime()
	c.Loaded = true
	return nil
}

func (c *Config) SaveConfig(configFile string) error {
	var newConfig bytes.Buffer
	newBuffer := bufio.NewWriter(&newConfig)

	if err := toml.NewEncoder(newBuffer).Encode(&c.BaseConfig); err != nil {
		log.Fatalf("Error encoding TOML: %s", err)
		return err
	}

	if err := newBuffer.Flush(); err != nil {
		return err
	}

	os.MkdirAll(filepath.Dir(configFile), 0700)

	err := ioutil.WriteFile(configFile, newConfig.Bytes(), 0600)
	if err != nil {
		return err
	}

	c.Loaded = true
	return nil
}
