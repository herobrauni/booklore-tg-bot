package bot

import (
	"fmt"

	"github.com/brauni/booklore-tg-bot/internal/auth"
	"github.com/brauni/booklore-tg-bot/internal/config"
	"github.com/brauni/booklore-tg-bot/internal/downloader"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

type Bot struct {
	api        *tgbotapi.BotAPI
	config     *config.Config
	auth       *auth.Authenticator
	downloader *downloader.Downloader
}

func NewBot(cfg *config.Config) (*Bot, error) {
	// Initialize Telegram bot API
	api, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		cfg.Logger.Error("Failed to initialize Telegram bot API",
			zap.Error(err))
		return nil, fmt.Errorf("failed to initialize Telegram bot API: %w", err)
	}

	// Initialize authenticator
	authenticator := auth.NewAuthenticator(cfg.AllowedUserIDs, cfg.Logger)

	// Initialize downloader
	dl := downloader.NewDownloader(cfg.DownloadFolder, cfg.AllowedFileTypes, cfg.MaxFileSizeMB, cfg.Logger)

	return &Bot{
		api:        api,
		config:     cfg,
		auth:       authenticator,
		downloader: dl,
	}, nil
}

func (b *Bot) Start() error {
	b.config.Logger.Info("Starting Telegram bot",
		zap.String("bot_username", b.api.Self.UserName),
		zap.Int("allowed_users_count", b.auth.GetAllowedUsersCount()))

	// Set up update configuration
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// Get updates channel
	updates := b.api.GetUpdatesChan(u)

	// Process updates
	for update := range updates {
		if update.Message != nil {
			b.handleMessage(update.Message)
		}
	}

	return nil
}

func (b *Bot) Stop() {
	b.config.Logger.Info("Stopping Telegram bot")
}

func (b *Bot) GetBotInfo() string {
	return fmt.Sprintf("Bot: %s (@%s)", b.api.Self.FirstName, b.api.Self.UserName)
}