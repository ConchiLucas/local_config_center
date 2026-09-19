package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"local_config_center/internal/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	settings, err := app.SettingsFromEnvironment()
	if err != nil {
		logger.Error("invalid settings", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, settings, logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
