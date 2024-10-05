package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

func hashMaker(text string) string {
	h := sha256.New()
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}

func TestConfig(t *testing.T) {
	testFileName := "netbox_collector.yml"
	testCfgFile, err := os.CreateTemp("", testFileName)
	if err != nil {
		t.Fatalf("Unable to create config test file: %v", err)
	}
	defer os.Remove(testCfgFile.Name())
	_, err = testCfgFile.WriteString(`---
logging:
  journal: true
  level: debug
  filename: /dir/fake_logfile.log
`)
	if err != nil {
		t.Fatalf("Unable to write to config test file: %v", err)
	}
	testCfgFile.Close()

	cfg, err := ParseConfig(testCfgFile.Name())
	if err != nil {
		t.Errorf("ParseConfig failed with: %v", err)
	}

	if !cfg.Logging.Journal {
		t.Error("cfg.logging.journal should be true")
	}

	if cfg.Logging.Filename != "/dir/fake_logfile.log" {
		t.Errorf("Unexpected logfile: Expected=/dir/fake_logfile/log, Got=%s", cfg.Logging.Filename)
	}

}

func TestFlags(t *testing.T) {
	f := ParseFlags()
	expectingConfig := "examples/netbox_collector.yml"
	if f.Config != expectingConfig {
		t.Errorf("Unexpected config flag: Expected=%s, Got=%s", expectingConfig, f.Config)
	}
	if f.Debug {
		t.Error("Unexpected debug flag: Expected=false, Got=true")
	}
}
