package config

import (
	_ "embed"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

//go:embed config.yaml
var embeddedConfig []byte

var config *Config

type DatabaseConfig struct {
	URL string `yaml:"url"`
	DB  string `yaml:"db"`
}

type TargetConfig struct {
	URL     string `yaml:"url"`
	Timeout int    `yaml:"timeout"`
}

type AuthConfig struct {
	EnableSignature     bool `yaml:"enable_signature"`
	SignatureTimeWindow int  `yaml:"signature_time_window"` // 时间窗口（秒）
}

type Config struct {
	Port     int                     `yaml:"port"`
	Database DatabaseConfig          `yaml:"database"`
	Auth     AuthConfig              `yaml:"auth"`
	Targets  map[string]TargetConfig `yaml:"targets"`
}

func NewConfig() (*Config, error) {
	c := new(Config)

	var configData []byte
	var err error

	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		configData, err = os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
		}
	} else {
		configData = embeddedConfig
	}

	err = yaml.Unmarshal(configData, c)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	config = c
	return c, nil
}

func GetConfig() *Config {
	return config
}
