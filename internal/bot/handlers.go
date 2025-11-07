package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

func (b *Bot) handleMessage(message *tgbotapi.Message) {
	userID := message.From.ID
	b.config.Logger.Debug("Received message",
		zap.Int64("user_id", userID),
		zap.String("username", message.From.UserName),
		zap.String("message_type", "text"))

	// Check if user is authorized
	if !b.auth.IsUserAllowed(userID) {
		b.sendUnauthorizedMessage(message.Chat.ID)
		return
	}

	// Handle different message types
	switch {
	case message.Document != nil:
		b.handleDocument(message)
	case message.Photo != nil:
		b.handlePhoto(message)
	case message.Audio != nil:
		b.handleAudio(message)
	case message.Video != nil:
		b.handleVideo(message)
	case message.Voice != nil:
		b.handleVoice(message)
	case message.Text != "":
		b.handleTextMessage(message)
	default:
		b.sendUnsupportedMessage(message.Chat.ID)
	}
}

func (b *Bot) handleDocument(message *tgbotapi.Message) {
	document := message.Document
	userID := message.From.ID

	b.config.Logger.Info("Processing document",
		zap.Int64("user_id", userID),
		zap.String("file_name", document.FileName),
		zap.String("mime_type", document.MimeType),
		zap.Int("file_size", document.FileSize))

	// Check file size
	if !b.downloader.IsFileSizeAllowed(int64(document.FileSize)) {
		msg := tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("❌ File too large! Maximum size is %d MB.", b.config.MaxFileSizeMB))
		b.api.Send(msg)
		return
	}

	// Get file URL
	fileURL, err := b.getFileURL(document.FileID)
	if err != nil {
		b.config.Logger.Error("Failed to get file URL",
			zap.String("file_id", document.FileID),
			zap.Error(err))

		// Provide more specific error messages based on the error type
		errorMsg := "Failed to get file URL"
		if containsIgnoreCase(err.Error(), "wrong file_id") || containsIgnoreCase(err.Error(), "temporarily unavailable") {
			errorMsg = "❌ File is no longer available on Telegram servers. Please resend the file."
		} else if containsIgnoreCase(err.Error(), "too many requests") {
			errorMsg = "⏳ Telegram is rate limiting requests. Please try again in a moment."
		}

		b.sendErrorMessage(message.Chat.ID, errorMsg)
		return
	}

	// Download file
	_, err = b.downloader.DownloadFile(fileURL, document.FileName)
	if err != nil {
		b.config.Logger.Error("Failed to download file",
			zap.String("file_name", document.FileName),
			zap.Error(err))
		b.sendErrorMessage(message.Chat.ID, fmt.Sprintf("Failed to download file: %s", err.Error()))
		return
	}

	// Trigger Booklore import if enabled
	importStatus := b.triggerBookloreImport(message.Chat.ID, document.FileName)

	// Prepare success message
	successMsg := fmt.Sprintf("✅ File '%s' downloaded successfully!", document.FileName)
	if importStatus != "" {
		successMsg = importStatus
	}

	// Send success message
	msg := tgbotapi.NewMessage(message.Chat.ID, successMsg)
	b.api.Send(msg)
}

func (b *Bot) handlePhoto(message *tgbotapi.Message) {
	photos := message.Photo
	if len(photos) == 0 {
		return
	}

	// Get the highest quality photo
	photo := photos[len(photos)-1]
	userID := message.From.ID

	b.config.Logger.Info("Processing photo",
		zap.Int64("user_id", userID),
		zap.Int("file_size", photo.FileSize),
		zap.Int("width", photo.Width),
		zap.Int("height", photo.Height))

	// Generate filename
	filename := fmt.Sprintf("photo_%s_%d.jpg", message.From.UserName, message.MessageID)

	// Get file URL
	fileURL, err := b.getFileURL(photo.FileID)
	if err != nil {
		b.config.Logger.Error("Failed to get photo URL",
			zap.String("file_id", photo.FileID),
			zap.Error(err))

		// Provide more specific error messages based on the error type
		errorMsg := "Failed to get photo URL"
		if containsIgnoreCase(err.Error(), "wrong file_id") || containsIgnoreCase(err.Error(), "temporarily unavailable") {
			errorMsg = "❌ Photo is no longer available on Telegram servers. Please resend the photo."
		} else if containsIgnoreCase(err.Error(), "too many requests") {
			errorMsg = "⏳ Telegram is rate limiting requests. Please try again in a moment."
		}

		b.sendErrorMessage(message.Chat.ID, errorMsg)
		return
	}

	// Download photo
	_, err = b.downloader.DownloadFile(fileURL, filename)
	if err != nil {
		b.config.Logger.Error("Failed to download photo",
			zap.String("filename", filename),
			zap.Error(err))
		b.sendErrorMessage(message.Chat.ID, fmt.Sprintf("Failed to download photo: %s", err.Error()))
		return
	}

	// Trigger Booklore import if enabled
	importStatus := b.triggerBookloreImport(message.Chat.ID, filename)

	// Prepare success message
	successMsg := fmt.Sprintf("✅ Photo '%s' downloaded successfully!", filename)
	if importStatus != "" {
		successMsg = importStatus
	}

	// Send success message
	msg := tgbotapi.NewMessage(message.Chat.ID, successMsg)
	b.api.Send(msg)
}

func (b *Bot) handleAudio(message *tgbotapi.Message) {
	audio := message.Audio
	b.downloadMediaFile(message, audio.FileID, audio.FileName, "audio", int64(audio.FileSize))
}

func (b *Bot) handleVideo(message *tgbotapi.Message) {
	video := message.Video
	b.downloadMediaFile(message, video.FileID, video.FileName, "video", int64(video.FileSize))
}

func (b *Bot) handleVoice(message *tgbotapi.Message) {
	voice := message.Voice
	filename := fmt.Sprintf("voice_%s_%d.ogg", message.From.UserName, message.MessageID)
	b.downloadMediaFile(message, voice.FileID, filename, "voice", int64(voice.FileSize))
}

func (b *Bot) downloadMediaFile(message *tgbotapi.Message, fileID, filename, mediaType string, fileSize int64) {
	userID := message.From.ID

	b.config.Logger.Info("Processing "+mediaType,
		zap.Int64("user_id", userID),
		zap.String("file_name", filename),
		zap.Int64("file_size", fileSize))

	// Check file size
	if !b.downloader.IsFileSizeAllowed(fileSize) {
		msg := tgbotapi.NewMessage(message.Chat.ID,
			fmt.Sprintf("❌ File too large! Maximum size is %d MB.", b.config.MaxFileSizeMB))
		b.api.Send(msg)
		return
	}

	// Get file URL
	fileURL, err := b.getFileURL(fileID)
	if err != nil {
		b.config.Logger.Error("Failed to get file URL",
			zap.String("file_id", fileID),
			zap.Error(err))

		// Provide more specific error messages based on the error type
		errorMsg := "Failed to get file URL"
		if containsIgnoreCase(err.Error(), "wrong file_id") || containsIgnoreCase(err.Error(), "temporarily unavailable") {
			errorMsg = "❌ File is no longer available on Telegram servers. Please resend the file."
		} else if containsIgnoreCase(err.Error(), "too many requests") {
			errorMsg = "⏳ Telegram is rate limiting requests. Please try again in a moment."
		}

		b.sendErrorMessage(message.Chat.ID, errorMsg)
		return
	}

	// Download file
	_, err = b.downloader.DownloadFile(fileURL, filename)
	if err != nil {
		b.config.Logger.Error("Failed to download "+mediaType,
			zap.String("filename", filename),
			zap.Error(err))
		b.sendErrorMessage(message.Chat.ID, fmt.Sprintf("Failed to download %s: %s", mediaType, err.Error()))
		return
	}

	// Trigger Booklore import if enabled
	importStatus := b.triggerBookloreImport(message.Chat.ID, filename)

	// Prepare success message
	successMsg := fmt.Sprintf("✅ %s '%s' downloaded successfully!", mediaType, filename)
	if importStatus != "" {
		successMsg = importStatus
	}

	// Send success message
	msg := tgbotapi.NewMessage(message.Chat.ID, successMsg)
	b.api.Send(msg)
}

func (b *Bot) handleTextMessage(message *tgbotapi.Message) {
	text := message.Text

	// Handle commands
	if text == "/start" || text == "/help" {
		b.sendHelpMessage(message.Chat.ID)
		return
	}

	if text == "/status" {
		b.sendStatusMessage(message.Chat.ID)
		return
	}

	// Default text response
	msg := tgbotapi.NewMessage(message.Chat.ID,
		"👋 Send me a file and I'll download it for you!\n\nUse /help for more information.")
	b.api.Send(msg)
}

func (b *Bot) getFileURL(fileID string) (string, error) {
	// Add some logging to debug the file ID and bot configuration
	b.config.Logger.Debug("Attempting to get file URL",
		zap.String("file_id", fileID),
		zap.String("bot_token_prefix", b.config.BotToken[:min(len(b.config.BotToken), 10)]+"..."))

	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		b.config.Logger.Error("Failed to get file info from Telegram",
			zap.String("file_id", fileID),
			zap.Error(err))
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	b.config.Logger.Debug("Got file info from Telegram",
		zap.String("file_id", fileID),
		zap.String("file_path", file.FilePath))

	// Try to get direct file URL
	fileURL, err := b.api.GetFileDirectURL(file.FilePath)
	if err != nil {
		b.config.Logger.Error("Failed to get direct file URL",
			zap.String("file_id", fileID),
			zap.String("file_path", file.FilePath),
			zap.Error(err))

		// Try alternative URL format as fallback
		// Sometimes the API endpoint format can cause issues
		botToken := b.config.BotToken
		alternativeURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, file.FilePath)

		b.config.Logger.Info("Trying alternative file URL format",
			zap.String("file_id", fileID),
			zap.String("alternative_url", alternativeURL))

		return alternativeURL, nil
	}

	b.config.Logger.Debug("Successfully generated file URL",
		zap.String("file_id", fileID),
		zap.String("file_url", fileURL))

	return fileURL, nil
}

func (b *Bot) sendUnauthorizedMessage(chatID int64) {
	msg := tgbotapi.NewMessage(chatID,
		"🚫 You are not authorized to use this bot.")
	b.api.Send(msg)
}

func (b *Bot) sendErrorMessage(chatID int64, errorMsg string) {
	msg := tgbotapi.NewMessage(chatID,
		fmt.Sprintf("❌ Error: %s", errorMsg))
	b.api.Send(msg)
}

func (b *Bot) sendUnsupportedMessage(chatID int64) {
	msg := tgbotapi.NewMessage(chatID,
		"❓ Unsupported message type. Please send a document, photo, audio, or video file.")
	b.api.Send(msg)
}

func (b *Bot) sendHelpMessage(chatID int64) {
	helpText := `🤖 *Telegram File Downloader Bot*

I can download files you send me and save them to my storage.`

	// Add Booklore integration info if enabled
	if b.booklore.IsEnabled() {
		helpText += `
📚 *Booklore Integration Enabled*
• Automatic import to Booklore library
• Smart book detection and processing`
	}

	helpText += `

*Features:*
• Download documents, photos, audio, and videos
• File type restrictions for security
• File size limits (configurable)
• User access control`

	if b.booklore.IsEnabled() {
		helpText += `
• Automatic Booklore library integration`
	}

	helpText += `

*Commands:*
/start or /help - Show this help message
/status - Show bot status and settings

*Allowed file types:* ` + fmt.Sprintf("%v", b.config.AllowedFileTypes) + `

*Max file size:* ` + fmt.Sprintf("%d MB", b.config.MaxFileSizeMB) + `

Simply send me any file and I'll download it for you!`

	if b.booklore.IsEnabled() {
		helpText += `

If Booklore integration is enabled, your books will be automatically imported to the library.`
	}

	msg := tgbotapi.NewMessage(chatID, helpText)
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

func (b *Bot) sendStatusMessage(chatID int64) {
	statusText := fmt.Sprintf(`📊 *Bot Status*

🤖 Bot: %s
📁 Download folder: %s
📋 Allowed users: %d
📄 Allowed file types: %d
📏 Max file size: %d MB`,
		b.api.Self.UserName,
		b.config.DownloadFolder,
		b.auth.GetAllowedUsersCount(),
		len(b.config.AllowedFileTypes),
		b.config.MaxFileSizeMB)

	// Add Booklore status if configured
	if b.booklore.IsEnabled() {
		statusText += fmt.Sprintf(`

📚 *Booklore Integration*
🔗 API URL: %s
📤 Auto-import: %t`,
			b.config.BookloreAPI.APIURL,
			b.config.BookloreAPI.AutoImport)
	} else {
		statusText += `

📚 Booklore Integration: ❌ Disabled`
	}

	msg := tgbotapi.NewMessage(chatID, statusText)
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

// triggerBookloreImport triggers the Booklore import process after a file download
func (b *Bot) triggerBookloreImport(chatID int64, filename string) string {
	if !b.booklore.IsEnabled() || !b.config.BookloreAPI.AutoImport {
		return ""
	}

	b.config.Logger.Info("Triggering Booklore import",
		zap.String("filename", filename))

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First rescan the bookdrop folder
	if err := b.booklore.RescanBookdrop(ctx); err != nil {
		b.config.Logger.Error("Failed to rescan bookdrop folder",
			zap.String("filename", filename),
			zap.Error(err))
		return fmt.Sprintf("📥 File downloaded, but failed to trigger Booklore scan: %s", err.Error())
	}

	// Then finalize all imports
	result, err := b.booklore.FinalizeAllImports(ctx)
	if err != nil {
		b.config.Logger.Error("Failed to finalize Booklore import",
			zap.String("filename", filename),
			zap.Error(err))
		return fmt.Sprintf("📥 File downloaded, but failed to complete Booklore import: %s", err.Error())
	}

	if result.ImportedCount > 0 {
		b.config.Logger.Info("Booklore import completed successfully",
			zap.String("filename", filename),
			zap.Int("imported_count", result.ImportedCount))
		return fmt.Sprintf("📚 File downloaded and imported to Booklore successfully! (%d books imported)", result.ImportedCount)
	} else if result.FailedCount > 0 {
		return fmt.Sprintf("📥 File downloaded, but %d books failed to import to Booklore", result.FailedCount)
	} else {
		return "📥 File downloaded to bookdrop, but no new books were imported"
	}
}

// Helper function for case-insensitive string matching
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
