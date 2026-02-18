package config

import (
	"errors"
	"os"
	"time"

	"github.com/livekit/protocol/auth"
	"gopkg.in/yaml.v3"
)

type LiveKitConfig struct {
	ApiKey    string `yaml:"key"`
	ApiUrl    string `yaml:"url"`
	ApiSecret string `yaml:"secret"`
}

type RedisConfig struct {
	Host     string `yaml:"host" env:"REDIS_HOST" default:"localhost"`
	Port     int    `yaml:"port" env:"REDIS_PORT" default:"6379"`
	Password string `yaml:"password" env:"REDIS_PASSWORD"`
	DB       int    `yaml:"db" env:"REDIS_DB" default:"0"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
}

type LoggerConfig struct {
	Level string `yaml:"level"`
}

type Config struct {
	LiveKitCfg   LiveKitConfig `yaml:"livekit"`
	RedisCfg     RedisConfig   `yaml:"redis"`
	ServerCfg    ServerConfig  `yaml:"server"`
	LoggerConfig LoggerConfig  `yaml:"logger"`
}

var (
	ErrNoDataInCfg   = errors.New("config: no data in config file")
	ErrInvalidConfig = errors.New("config: invalid configuration")
	ErrMissingField  = errors.New("config: missing required field")
)

func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrNoDataInCfg
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.LiveKitCfg.ApiKey == "" {
		return ErrMissingField
	}
	if c.LiveKitCfg.ApiUrl == "" {
		return ErrMissingField
	}
	if c.LiveKitCfg.ApiSecret == "" {
		return ErrMissingField
	}
	return nil
}

func NewLiveKitConfigFromConfig(liveKitCfg LiveKitConfig) *LiveKitConfig {
	return &LiveKitConfig{
		ApiKey:    liveKitCfg.ApiKey,
		ApiUrl:    liveKitCfg.ApiUrl,
		ApiSecret: liveKitCfg.ApiSecret,
	}
}

func (c *LiveKitConfig) GenerateToken(room string, id string) (string, error) {
	at := auth.NewAccessToken(c.ApiKey, c.ApiSecret)

	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     room,
	}
	at.SetVideoGrant(grant).SetIdentity(id).SetValidFor(time.Hour * 8)
	return at.ToJWT()
}
