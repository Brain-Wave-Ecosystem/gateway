package main

import (
	"context"
	"errors"
	gateway "github.com/Brain-Wave-Ecosystem/gateway/internal/config"
	"github.com/Brain-Wave-Ecosystem/gateway/internal/server"
	"github.com/Brain-Wave-Ecosystem/go-common/pkg/abstractions"
	"github.com/Brain-Wave-Ecosystem/go-common/pkg/config"
	"golang.org/x/exp/slog"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg, err := config.Load[gateway.Config]()
	if err != nil {
		slog.Error("Error loading config: ", "error", err)
		return
	}

	startCtx, startCancel := context.WithTimeout(context.Background(), cfg.StartTimeout)
	defer startCancel()

	var srv abstractions.Server

	srv, err = server.NewServer(startCtx, cfg)
	if err != nil {
		slog.Error("Error starting server: ", "error", err)
		return
	}

	go func() {
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

		<-signalCh

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer shutdownCancel()

		if err = srv.Shutdown(shutdownCtx); err != nil {
			if err != io.EOF || !errors.Is(context.Canceled, err) {
				slog.Warn("Server forced to shutdown", "error", err)
			}
		}
	}()

	if err = srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("Server failed to start", "error", err)
	}
}
