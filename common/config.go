package common

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	log "github.com/Sirupsen/logrus"
	"path/filepath"
)


type Config struct {
	BaseConfig
	ModTime time.Time `json:"-"`
	Loaded  bool      `json:"-"`
}

type Server struct {
	Url           string
	Weight        int
}

type BaseConfig struct {
	ApiAddress                     string  `toml:"api-address"`
	LbApiLogin                     string  `toml:"api-login"`
	LbApiPassword                  string  `toml:"api-password"`
	LbAddress                      string  `toml:"lb-address"`
	LbLogFile                      string  `toml:"lb-log-file"`

	LbStreamRetryConditions        string  `toml:"stream_retry_conditions"`
	LbMonitorBrokenBackends        bool    `toml:"monitor_broken_backend"`
	LbMonitorRemoveBrokenBackends  bool    `toml:"remove_broken_backends"`
	LbStats                        bool    `toml:"statistics_enabled"`
	LbMonitorCheckPeriod           int     `toml:"check_period"`
	LbMonitorMaxFails              int     `toml:"max_fails"`
	LbMonitorFailTimeout           int     `toml:"fail_timeout"`
	LbMonitorBashScript            string  `toml:"bash_script"`
	LbMonitorWebUrl                string  `toml:"web_url"`
	LbEnableRebalancer             bool    `toml:"enable_rebalancer"`

	LbEnableConnlimit              bool    `toml:"enable_connlimit"`
	LbConnlimitConnections         int     `toml:"connlimit_connections"`
	LbConnlimitVariable            string  `toml:"connlimit_variable"`

	Servers    map[string]Server
}


func NewConfig() *Config {
	return &Config{
		BaseConfig: BaseConfig{
			ApiAddress: "0.0.0.0:9090",
			LbApiLogin: "lb",
			LbApiPassword: "7eNQ4iWLgDw4Q6w",
			LbAddress: "127.0.0.1:8080",
			LbLogFile: "",

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
