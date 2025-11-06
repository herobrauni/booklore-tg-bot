package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/brauni/booklore-tg-bot/internal/bot"
	"github.com/brauni/booklore-tg-bot/internal/config"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	defer cfg.Logger.Sync()

	cfg.Logger.Info("Starting Telegram File Downloader Bot")

	// Create bot instance
	botInstance, err := bot.NewBot(cfg)
	if err != nil {
		cfg.Logger.Fatal("Failed to create bot instance",
			zap.Error(err))
	}

	cfg.Logger.Info("Bot created successfully",
		zap.String("bot_info", botInstance.GetBotInfo()))

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start bot in a goroutine
	go func() {
		if err := botInstance.Start(); err != nil {
			cfg.Logger.Error("Bot stopped with error",
				zap.Error(err))
			cancel()
		}
	}()

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		cfg.Logger.Info("Received shutdown signal",
			zap.String("signal", sig.String()))
	case <-ctx.Done():
		cfg.Logger.Info("Context cancelled, shutting down")
	}

	// Graceful shutdown
	botInstance.Stop()
	cfg.Logger.Info("Bot shutdown complete")
}