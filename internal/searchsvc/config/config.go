package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	SessionTTL     time.Duration
	EISBaseURL     string
	ParserURL      string
	CoreURL        string
	MaxSearchPages int
	InsecureTLS    bool
}

func FromEnv() Config {
	ttlHours := envInt("SESSION_TTL_HOURS", 72)
	return Config{
		HTTPAddr:       env("HTTP_ADDR", ":8093"),
		DatabaseURL:    env("DATABASE_URL", "postgres://zakupki:zakupki@localhost:5432/zakupki_search?sslmode=disable"),
		SessionTTL:     time.Duration(ttlHours) * time.Hour,
		EISBaseURL:     env("EIS_BASE_URL", "https://zakupki.gov.ru"),
		ParserURL:      stringsTrimRightSlash(env("PARSER_URL", "")),
		CoreURL:        stringsTrimRightSlash(env("CORE_URL", "")),
		MaxSearchPages: envInt("MAX_SEARCH_PAGES", 3),
		InsecureTLS:    env("EIS_INSECURE_TLS", "true") == "true",
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func stringsTrimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
