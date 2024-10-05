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
	testFileName := "groupsha.yml"
	testCfgFile, err := os.CreateTemp("", testFileName)
	if err != nil {
		t.Fatalf("Unable to create config test file: %v", err)
	}
	defer os.Remove(testCfgFile.Name())
	_, err = testCfgFile.WriteString(`---
groups:
  foo:
    hash: 2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae
  bar:
    hash: fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9
  baz:
    hash: baa5a0964d3320fbc0c6a922140453c8513ea24ab8fd0577034804a967248096
exporter:
  address: localhost
  port: 12345
logging:
  journal: true
  level: debug
`)
	if err != nil {
		t.Fatalf("Unable to write to config test file: %v", err)
	}
	testCfgFile.Close()

	cfg, err := ParseConfig(testCfgFile.Name())
	if err != nil {
		t.Fatalf("ParseConfig failed with: %v", err)
	}
	// As a quick test, we compare the hash of the array keys.
	// Eg. sha256("foo") = 2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae
	for k, v := range cfg.Groups {
		hash := hashMaker(k)
		if hash != v.Hash {
			t.Errorf("Unexpected hash for %s.  Expected=%s, Got=%s", k, v, hash)
		}
	}
	if !cfg.Logging.Journal {
		t.Fatal("cfg.logging.journal should be true")
	}
}

func TestFlags(t *testing.T) {
	f := ParseFlags()
	expectingConfig := "examples/sha_exporter.yml"
	if f.Config != expectingConfig {
		t.Fatalf("Unexpected config flag: Expected=%s, Got=%s", expectingConfig, f.Config)
	}
	if f.Debug {
		t.Fatal("Unexpected debug flag: Expected=false, Got=true")
	}
}
