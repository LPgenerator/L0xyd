package commands

import (
	"github.com/gotlium/lpg-load-balancer/helpers"
	"github.com/gotlium/lpg-load-balancer/common"
	"path/filepath"
	"os"
)

type configOptions struct {
	config *common.Config

	ConfigFile string `short:"c" long:"config" env:"CONFIG_FILE" description:"Config file"`
}

/*
func getDefaultConfigFile() string {
	_, etc_err := os.Stat("/etc/lpg-load-balancer/config.toml");

	homeDir := helpers.GetHomeDir();
	homeCfg := filepath.Join(homeDir, ".lpg-load-balancer", "config.toml")
	_, home_err := os.Stat(homeCfg);

	currentDir := helpers.GetCurrentWorkingDirectory();
	currentCfg := filepath.Join(currentDir, "config.toml")
	_, current_err := os.Stat(currentCfg);

	if os.Getuid() == 0 && etc_err == nil {
		return "/etc/lpg-load-balancer/config.toml"
	} else if homeDir != "" && home_err == nil {
		return homeCfg
	} else if currentDir != "" && current_err == nil {
		return currentCfg
	} else {
		return "/Users/gotlium/config.toml"
		//panic("Cannot get default config file location")
	}
}
*/

func getDefaultConfigFile() string {
	if os.Getuid() == 0 {
		return "/etc/lpg-load-balancer/config.toml"
	} else if homeDir := helpers.GetHomeDir(); homeDir != "" {
		return filepath.Join(homeDir, ".lpg-load-balancer", "config.toml")
	} else if currentDir := helpers.GetCurrentWorkingDirectory(); currentDir != "" {
		return filepath.Join(currentDir, "config.toml")
	} else {
		panic("Cannot get default config file location")
	}
}

func (c *configOptions) saveConfig() error {
	return c.config.SaveConfig(c.ConfigFile)
}

func (c *configOptions) loadConfig() error {
	config := common.NewConfig()
	err := config.LoadConfig(c.ConfigFile)
	if err != nil {
		return err
	}
	c.config = config
	return nil
}

func (c *configOptions) touchConfig() error {
	// try to load existing config
	err := c.loadConfig()
	if err != nil {
		return err
	}

	// save config for the first time
	if !c.config.Loaded {
		return c.saveConfig()
	}
	return nil
}

func init() {
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		os.Setenv("CONFIG_FILE", getDefaultConfigFile())
	}
}
