package config

import (
	"errors"
	"os"
	"strings"
)

const ServiceName = "doc-extract"

type Config struct {
	HTTPAddr       string
	APIKey         string
	LogLevel       string
	KafkaEnabled   bool
	KafkaBrokers   []string
	KafkaTopicIn   string
	KafkaTopicOut  string
	KafkaGroupID   string
	HTTPIngest     bool
	KafkaIngest    bool
}

func FromEnv() Config {
	key := os.Getenv("DOC_EXTRACT_API_KEY")
	if key == "" {
		key = os.Getenv("DOC_PARSER_API_KEY")
	}
	if key == "" {
		key = os.Getenv("API_KEY")
	}
	brokers := splitBrokers(os.Getenv("KAFKA_BROKERS"))
	kafkaOn := envBool("KAFKA_ENABLED", false)
	return Config{
		HTTPAddr:      env("HTTP_ADDR", ":8094"),
		APIKey:        key,
		LogLevel:      env("LOG_LEVEL", "info"),
		KafkaEnabled:  kafkaOn,
		KafkaBrokers:  brokers,
		KafkaTopicIn:  env("KAFKA_TOPIC_IN", "docextract.extract.in"),
		KafkaTopicOut: env("KAFKA_TOPIC_OUT", "docextract.extract.out"),
		KafkaGroupID:  env("KAFKA_GROUP_ID", "docextract"),
		HTTPIngest:    envBoolFirst([]string{"HTTP_INGEST_ENABLED", "HTTP_ENABLED"}, true),
		KafkaIngest:   envBoolFirst([]string{"KAFKA_INGEST_ENABLED", "KAFKA_ENABLED"}, false),
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return errors.New("DOC_EXTRACT_API_KEY, DOC_PARSER_API_KEY or API_KEY is required")
	}
	return nil
}

func (c Config) HasKafkaBrokers() bool {
	return len(c.KafkaBrokers) > 0
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	return envBoolFirst([]string{k}, def)
}

func envBoolFirst(keys []string, def bool) bool {
	for _, k := range keys {
		v := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
		if v == "" {
			continue
		}
		return v == "1" || v == "true" || v == "yes"
	}
	return def
}

func splitBrokers(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
