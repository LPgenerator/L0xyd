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


type BaseConfig struct {
	Concurrent int             `toml:"concurrent" json:"concurrent"`
	User       string         `toml:"user" json:"user"`
}

type Config struct {
	BaseConfig
	ModTime time.Time `json:"-"`
	Loaded  bool      `json:"-"`
}

func NewConfig() *Config {
	return &Config{
		BaseConfig: BaseConfig{
			Concurrent: 1,
			User: "unknown",
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