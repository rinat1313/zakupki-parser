package config

import (
	"os"
	"testing"
)

func TestFromEnvDefaults(t *testing.T) {
	os.Unsetenv("DOC_PARSER_API_KEY")
	os.Unsetenv("API_KEY")
	os.Unsetenv("HTTP_ADDR")
	cfg := FromEnv()
	if cfg.HTTPAddr != ":8094" {
		t.Fatalf("HTTPAddr=%q", cfg.HTTPAddr)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validate error without api key")
	}
}

func TestAPIKeyFallback(t *testing.T) {
	os.Unsetenv("DOC_PARSER_API_KEY")
	os.Setenv("API_KEY", "secret-key")
	defer os.Unsetenv("API_KEY")
	cfg := FromEnv()
	if cfg.APIKey != "secret-key" {
		t.Fatalf("APIKey=%q", cfg.APIKey)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSplitBrokers(t *testing.T) {
	os.Setenv("KAFKA_BROKERS", "a:9092, b:9092")
	defer os.Unsetenv("KAFKA_BROKERS")
	cfg := FromEnv()
	if len(cfg.KafkaBrokers) != 2 {
		t.Fatalf("brokers=%v", cfg.KafkaBrokers)
	}
}
