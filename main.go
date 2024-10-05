package main

import (
	"os"

	"github.com/Masterminds/log-go"
	"github.com/crooks/jlog"
	loglevel "github.com/crooks/log-go-level"
	"github.com/crooks/netbox_collector/config"
	"github.com/crooks/netbox_collector/omeapi"
)

var (
	cfg   *config.Config
	flags *config.Flags
)

func InitAPI(username, password, cert string) {
	api := omeapi.NewBasicAuthClient(username, password, cert)
	return api
}

func main() {
	var err error
	flags = config.ParseFlags()
	cfg, err = config.ParseConfig(flags.Config)
	if err != nil {
		log.Fatalf("Cannot parse config: %v", err)
	}
	loglev, err := loglevel.ParseLevel(cfg.Logging.LevelStr)
	if err != nil {
		log.Fatalf("Unable to set log level: %v", err)
	}
	if cfg.Logging.Journal && !jlog.Enabled() {
		log.Warn("Cannot log to systemd journal")
	}
	if cfg.Logging.Journal && jlog.Enabled() {
		log.Current = jlog.NewJournal(loglev)
		log.Debugf("Logging to journal has been initialised at level: %s", cfg.Logging.LevelStr)
	} else {
		if cfg.Logging.Filename == "" {
			log.Fatal("Cannot log to file, no filename specified in config")
		}
		logWriter, err := os.OpenFile(cfg.Logging.Filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0640)
		if err != nil {
			log.Fatalf("Unable to open logfile: %s", err)
		}
		defer logWriter.Close()
		stdlog.SetOutput(logWriter)
		log.Current = log.StdLogger{Level: loglev}
		log.Debugf("Logging to file %s has been initialised at level: %s", cfg.Logging.Filename, cfg.Logging.LevelStr)
	}
}
