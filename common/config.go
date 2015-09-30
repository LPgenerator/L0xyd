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
	ApiAddress string         `toml:"api-address" json:"api-address"`
	LbAddress  string         `toml:"lb-address" json:"lb-address"`
	LbLogFile  string         `toml:"lb-log-file" json:"lb-log-file"`
	Servers map[string]Server
}


func NewConfig() *Config {
	return &Config{
		BaseConfig: BaseConfig{
			ApiAddress: "0.0.0.0:9090",
			LbAddress: "127.0.0.1:8080",
			LbLogFile: "",
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

	// permission denied is soft error
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

	// create directory to store configuration
	os.MkdirAll(filepath.Dir(configFile), 0700)

	// write config file
	if err := ioutil.WriteFile(configFile, newConfig.Bytes(), 0600); err != nil {
		return err
	}

	c.Loaded = true
	return nil
}
