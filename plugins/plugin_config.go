package plugins

import (
	"errors"
	"fmt"
	"os"
	"path"

	"git.sr.ht/~avery/crankshaft/cdp"
	"github.com/BurntSushi/toml"
)

type authorInfo struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

type entrypoint struct {
	Library       bool `json:"library"`
	Menu          bool `json:"menu"`
	QuickAccess   bool `json:"quickAccess" toml:"quick-access"`
	AppProperties bool `json:"appProperties" toml:"app-properties"`
}

type platform struct {
	Supported bool `json:"supported"`
}

type platforms struct {
	Linux   platform `json:"linux"`
	Windows platform `json:"windows"`
	Darwin  platform `json:"darwin"`
}

type store struct {
	Description string    `json:"description"`
	Platforms   platforms `json:"platforms"`
}

type pluginConfig struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Link    string `json:"link"`
	Source  string `json:"source"`

	Author      authorInfo                `json:"author"`
	Entrypoints map[cdp.UIMode]entrypoint `json:"entrypoints"`
	Store       store                     `json:"store"`
}

func NewPluginConfig(pluginDir string) (*pluginConfig, error) {
	configFilePath := path.Join(pluginDir, "plugin.toml")
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf(`Error opening plugin config at "%s": %v:`, configFilePath, err)
	}

	var config pluginConfig
	if _, err := toml.Decode(string(data), &config); err != nil {
		return nil, fmt.Errorf(`Error decoding plugin config at "%s": %v`, configFilePath, err)
	}

	if err := config.validateConfig(); err != nil {
		return nil, fmt.Errorf(`Error found in plugin configconfig at "%s": %v`, configFilePath, err)
	}

	return &config, nil
}

func (p *pluginConfig) validateConfig() error {
	if _, contains := p.Entrypoints["desktop"]; !contains {
		return errors.New("Config was missing entrypoints.desktop")
	}

	if _, contains := p.Entrypoints["deck"]; !contains {
		return errors.New("Config was missing entrypoints.deck")
	}

	return nil
}
