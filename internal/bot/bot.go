package bot

import (
	"fmt"

	"github.com/brauni/booklore-tg-bot/internal/auth"
	"github.com/brauni/booklore-tg-bot/internal/booklore"
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
	booklore   *booklore.Client
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

	// Initialize Booklore client
	bookloreClient := booklore.NewClient(cfg.BookloreAPI.APIURL, cfg.BookloreAPI.APIToken, cfg.Logger)

	return &Bot{
		api:        api,
		config:     cfg,
		auth:       authenticator,
		downloader: dl,
		booklore:   bookloreClient,
	}, nil
}

func (b *Bot) Start() error {
	b.config.Logger.Info("Starting Telegram bot",
		zap.String("bot_username", b.api.Self.UserName),
		zap.Int("allowed_users_count", b.auth.GetAllowedUsersCount()))

	// Log Booklore API status
	if b.booklore.IsEnabled() {
		b.config.Logger.Info("Booklore API integration enabled",
			zap.String("api_url", b.config.BookloreAPI.APIURL),
			zap.Bool("auto_import", b.config.BookloreAPI.AutoImport))
	} else {
		b.config.Logger.Info("Booklore API integration disabled")
	}

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
