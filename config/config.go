package config

import (
	_ "embed"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// //go:embed config.dev.yaml
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

type AsyncConfig struct {
	Enabled     bool        `yaml:"enabled"`
	WorkerCount int         `yaml:"worker_count"`
	Redis       RedisConfig `yaml:"redis"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	QueueKey string `yaml:"queue_key"`
}

// SignatureConfig 签名配置
type SignatureConfig struct {
	Type   string            `yaml:"type"`
	Config map[string]string `yaml:"config"`
}

// PathSignatureMapping 路径签名映射
type PathSignatureMapping struct {
	Path      string          `yaml:"path"`
	Signature SignatureConfig `yaml:"signature"`
}

type Config struct {
	Port           int                     `yaml:"port"`
	Database       DatabaseConfig          `yaml:"database"`
	Auth           AuthConfig              `yaml:"auth"`
	Async          AsyncConfig             `yaml:"async"` // 异步任务配置
	Targets        map[string]TargetConfig `yaml:"targets"`
	PathSignatures []PathSignatureMapping  `yaml:"path_signatures"`
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
