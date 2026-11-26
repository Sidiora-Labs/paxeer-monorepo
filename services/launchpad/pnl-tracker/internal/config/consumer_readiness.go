package config

import (
	"fmt"
	"time"
)

func (c Config) ConsumerReadinessBound() (time.Duration, error) {
	raw := c.ConsumerReadinessTimeout
	if raw == "" {
		raw = "5m"
	}
	bound, err := time.ParseDuration(raw)
	if err != nil || bound < 30*time.Second || bound > time.Hour {
		return 0, fmt.Errorf("config: PNL_CONSUMER_READINESS_TIMEOUT must be between 30s and 1h (default 5m)")
	}
	return bound, nil
}
