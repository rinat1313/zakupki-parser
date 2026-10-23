package main

import (
	"os"
	"testing"

	"github.com/rinat1313/zakupki-parser/internal/docextract/config"
)

func TestConfigRequiresAPIKey(t *testing.T) {
	os.Unsetenv("DOC_EXTRACT_API_KEY")
	os.Unsetenv("DOC_PARSER_API_KEY")
	os.Unsetenv("API_KEY")
	cfg := config.FromEnv()
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error")
	}
}
