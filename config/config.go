package config

import (
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	BootstrapNodes []string `yaml:"bootstrap_nodes"`
	Listen         string   `yaml:"listen"`
	Mongo          string   `yaml:"mongo"`
	Tracker        string   `yaml:"tracker"`
	TrackerLimit   int      `yaml:"tracker_limit"`
	ES             string   `yaml:"es"`
	MaxQueueSize   int      `yaml:"max_queue_size"`
}

func ReadConfigFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
