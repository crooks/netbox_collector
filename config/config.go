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
	OmeApi struct {
		Url      string `yaml:"url"`
		UserID   string `yaml:"userid"`
		Password string `yaml:"password"`
		CertFile string `yaml:"certfile"`
		// Page determines how many records to retreive per json paginated output
		Page int `yaml:"page"`
	} `yaml:"omeapi"`

    Database struct {
        Host string `yaml:"hostname"`
        Port int `yaml:"port"`
        User string `yaml:"userid"`
        Password string `yaml:"password"`
        DbName string `yaml:"dbname"`
        SSLMode bool `yaml:"sslmode"`
    } `yaml:"database"`

	Logging struct {
		Journal  bool   `yaml:"journal"`
		LevelStr string `yaml:"level"`
		Filename string `yamn:"filename"`
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
	if config.OmeApi.Page == 0 {
		config.OmeApi.Page = 5
	}
    if config.Database.Host == "" {
        config.Database.Host = "localhost"
    }
    if config.Database.port == 0 {
        config.Database.port = 5432
    }
    if config.Database.DbName == "" {
        config.Database.DbName = "assets"
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
	flag.StringVar(&f.Config, "config", "examples/netbox_collector.yml", "Path to netbox_collector configuration file")
	flag.BoolVar(&f.Debug, "debug", false, "Expand logging with Debug level messaging and format")
	flag.Parse()
	return f
}
