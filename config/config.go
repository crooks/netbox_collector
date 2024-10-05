package config

import (
	"flag"
	"os"

	"gopkg.in/yaml.v3"
)

// Flags are the command line Flags
type Flags struct {
	Config string
	Debug  bool
}

type GroupMetric struct {
	Hash string `yaml:"hash"`
}

type FileMetric struct {
	Path string `yaml:"path"`
	Hash string `yaml:"hash"`
}

// Config contains all the configuration settings
type Config struct {
	// Groups is keyed by groupname and contains the sha256 has (in hex, formatted as a string)
	Groups         map[string]GroupMetric `yaml:"groups"`
	GroupFile      string                 `yaml:"groupfile"`
	Files          map[string]FileMetric  `yaml:"files"`
	ScrapeInterval int                    `yaml:"scrape_interval"`
	Exporter       struct {
		Address string `yaml:"address"`
		Port    int    `yaml:"port"`
	}
	Logging struct {
		Journal  bool   `yaml:"journal"`
		LevelStr string `yaml:"level"`
	} `yaml:"logging"`
}

// ParseConfig imports a yaml formatted config file into a Config struct
func ParseConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{}
	d := yaml.NewDecoder(file)
	if err := d.Decode(&config); err != nil {
		return nil, err
	}
	// Define some defaults
	if config.GroupFile == "" {
		config.GroupFile = "/etc/group"
	}
	if config.ScrapeInterval == 0 {
		config.ScrapeInterval = 60
	}
	if config.Exporter.Address == "" {
		config.Exporter.Address = "0.0.0.0"
	}
	if config.Exporter.Port == 0 {
		config.Exporter.Port = 9773
	}
	if config.Logging.LevelStr == "" {
		config.Logging.LevelStr = "info"
	}
	return config, nil
}

// parseFlags processes arguments passed on the command line in the format
// standard format: --foo=bar
func ParseFlags() *Flags {
	f := new(Flags)
	flag.StringVar(&f.Config, "config", "examples/sha_exporter.yml", "Path to sha_exporter configuration file")
	flag.BoolVar(&f.Debug, "debug", false, "Expand logging with Debug level messaging and format")
	flag.Parse()
	return f
}
