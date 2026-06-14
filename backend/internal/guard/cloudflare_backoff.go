package guard

import (
	"math"
	"time"
)

// CloudflareBackoffConfig defines the progressive backoff parameters.
type CloudflareBackoffConfig struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`
	InitialCooldown time.Duration `yaml:"initial_cooldown" json:"initial_cooldown"`
	MaxCooldown     time.Duration `yaml:"max_cooldown" json:"max_cooldown"`
	BackoffFactor   int           `yaml:"backoff_factor" json:"backoff_factor"`
}

// DefaultCloudflareBackoffConfig returns sensible defaults.
func DefaultCloudflareBackoffConfig() CloudflareBackoffConfig {
	return CloudflareBackoffConfig{
		Enabled:         true,
		InitialCooldown: 10 * time.Second,
		MaxCooldown:     120 * time.Second,
		BackoffFactor:   3,
	}
}

// CloudflareBackoff calculates the cooldown duration for a given
// backoff level using exponential progression.
//
// Level 0: InitialCooldown
// Level 1: InitialCooldown * BackoffFactor
// Level 2: InitialCooldown * BackoffFactor^2
// ...
// Cap: MaxCooldown
func CloudflareBackoff(level int, cfg *CloudflareBackoffConfig) time.Duration {
	if cfg == nil {
		cfg = &CloudflareBackoffConfig{
			InitialCooldown: 10 * time.Second,
			MaxCooldown:     120 * time.Second,
			BackoffFactor:   3,
		}
	}
	if !cfg.Enabled {
		return 0
	}
	if level < 0 {
		level = 0
	}
	factor := cfg.BackoffFactor
	if factor <= 1 {
		factor = 3
	}
	d := time.Duration(float64(cfg.InitialCooldown) * math.Pow(float64(factor), float64(level)))
	if d > cfg.MaxCooldown {
		d = cfg.MaxCooldown
	}
	if d < cfg.InitialCooldown {
		d = cfg.InitialCooldown
	}
	return d
}
