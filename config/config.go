package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/taehwanyang/flowmancer/internal/anomaly"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Detector DetectorConfig `yaml:"detector"`
}

type ServerConfig struct {
	BuildDuration    Duration `yaml:"buildDuration"`
	WindowSize       Duration `yaml:"windowSize"`
	MaxWindowSamples int      `yaml:"maxWindowSamples"`
}

type DetectorConfig struct {
	Enabled bool `yaml:"enabled"`

	NewDestination RuleToggle `yaml:"newDestination"`

	RareDestination RareDestinationConfig `yaml:"rareDestination"`
	VolumeAnomaly   VolumeAnomalyConfig   `yaml:"volumeAnomaly"`
}

type RuleToggle struct {
	Enabled bool `yaml:"enabled"`
}

type RareDestinationConfig struct {
	Enabled bool `yaml:"enabled"`

	MaxDaysSeen        int     `yaml:"maxDaysSeen"`
	MaxTotalCount      uint64  `yaml:"maxTotalCount"`
	MinCurrentWinCount uint64  `yaml:"minCurrentWinCount"`
	MinSpikeMultiplier float64 `yaml:"minSpikeMultiplier"`
}

type VolumeAnomalyConfig struct {
	Enabled bool `yaml:"enabled"`

	MinSampleWindows int     `yaml:"minSampleWindows"`
	MinCurrentCount  uint64  `yaml:"minCurrentCount"`
	AvgMultiplier    float64 `yaml:"avgMultiplier"`
	P95Multiplier    float64 `yaml:"p95Multiplier"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}

	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}

	d.Duration = parsed
	return nil
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			BuildDuration:    Duration{Duration: 5 * time.Minute},
			WindowSize:       Duration{Duration: 1 * time.Minute},
			MaxWindowSamples: 288,
		},
		Detector: DetectorConfig{
			Enabled: true,
			NewDestination: RuleToggle{
				Enabled: true,
			},
			RareDestination: RareDestinationConfig{
				Enabled:            true,
				MaxDaysSeen:        2,
				MaxTotalCount:      20,
				MinCurrentWinCount: 10,
				MinSpikeMultiplier: 3.0,
			},
			VolumeAnomaly: VolumeAnomalyConfig{
				Enabled:          true,
				MinSampleWindows: 10,
				MinCurrentCount:  20,
				AvgMultiplier:    3.0,
				P95Multiplier:    1.5,
			},
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config file %s: %w", path, err)
	}

	return cfg, nil
}

func (c RareDestinationConfig) ToAnomalyConfig() anomaly.RareDestinationConfig {
	return anomaly.RareDestinationConfig{
		MaxDaysSeen:        c.MaxDaysSeen,
		MaxTotalCount:      c.MaxTotalCount,
		MinCurrentWinCount: c.MinCurrentWinCount,
		MinSpikeMultiplier: c.MinSpikeMultiplier,
	}
}

func (c VolumeAnomalyConfig) ToAnomalyConfig() anomaly.VolumeAnomalyConfig {
	return anomaly.VolumeAnomalyConfig{
		MinSampleWindows: c.MinSampleWindows,
		MinCurrentCount:  c.MinCurrentCount,
		AvgMultiplier:    c.AvgMultiplier,
		P95Multiplier:    c.P95Multiplier,
	}
}
