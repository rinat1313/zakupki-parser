package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/rinat1313/zakupki-parser/internal/docparser/channels"
	"github.com/rinat1313/zakupki-parser/internal/docparser/config"
	"github.com/rinat1313/zakupki-parser/internal/docparser/core"
	"github.com/rinat1313/zakupki-parser/internal/docparser/httpapi"
	"github.com/rinat1313/zakupki-parser/internal/docparser/kafka"
	"github.com/rinat1313/zakupki-parser/internal/docparser/logsetup"
)

func main() {
	cfg := config.FromEnv()
	logsetup.Init(cfg.LogLevel)
	if err := cfg.Validate(); err != nil {
		slog.Error("config invalid", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := kafka.ValidateEnabled(cfg.KafkaEnabled, cfg.KafkaBrokers); err != nil {
		slog.Error("kafka config invalid", "err", err)
		os.Exit(1)
	}

	chStore := channels.NewStore(channels.State{
		HTTPIngest:  cfg.HTTPIngest,
		KafkaIngest: cfg.KafkaIngest && cfg.HasKafkaBrokers(),
	}, cfg.HasKafkaBrokers)

	extractor := core.DefaultExtractor{}
	api := httpapi.New(cfg, chStore, extractor)

	readHdr, read, write, idle := httpapi.ServerTimeouts()
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Mux,
		ReadHeaderTimeout: readHdr,
		ReadTimeout:       read,
		WriteTimeout:      write,
		IdleTimeout:       idle,
	}

	var kafkaReader kafka.MessageReader
	var kafkaWriter kafka.MessageWriter
	if cfg.KafkaEnabled && cfg.HasKafkaBrokers() {
		kafkaReader = kafka.NewReader(cfg.KafkaBrokers, cfg.KafkaTopicIn, cfg.KafkaGroupID)
		kafkaWriter = kafka.NewWriter(cfg.KafkaBrokers, cfg.KafkaTopicOut)
		defer kafkaReader.Close()
		defer kafkaWriter.Close()
		runner := kafka.NewRunner(kafkaReader, kafkaWriter, chStore, extractor)
		go func() {
			if err := runner.Run(ctx); err != nil && ctx.Err() == nil {
				slog.Error("kafka runner stopped", "err", err)
			}
		}()
	}

	go func() {
		slog.Info("doc-parser listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpapi.ShutdownTimeout())
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
