package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	LiquidsoaplAddr     string        `envconfig:"LIQUIDSOAP_ADDR"       default:"liquidsoap:1234"`
	MusicRoot           string        `envconfig:"MUSIC_ROOT"            default:"/music"`
	QueueName           string        `envconfig:"QUEUE_NAME"            default:"manual"`
	HTTPPort            int           `envconfig:"HTTP_PORT"             default:"8010"`
	LogLevel            string        `envconfig:"LOG_LEVEL"             default:"info"`
	AuthUser            string        `envconfig:"AUTH_USER"`
	AuthPass            string        `envconfig:"AUTH_PASS"`
	LastFMKey           string        `envconfig:"LASTFM_APIKEY"`
	LastFMURL           string        `envconfig:"LASTFM_URL"            default:"https://ws.audioscrobbler.com/2.0"`
	HistoryDB           string        `envconfig:"HISTORY_DB"            default:"/data/history.db"`
	HistoryPollInterval time.Duration `envconfig:"HISTORY_POLL_INTERVAL" default:"30s"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) HTTPAddr() string {
	return fmt.Sprintf(":%d", c.HTTPPort)
}

func (c *Config) AuthEnabled() bool {
	return c.AuthUser != "" && c.AuthPass != ""
}
