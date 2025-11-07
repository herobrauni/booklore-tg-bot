package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/brauni/booklore-tg-bot/internal/booklore"
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

	if text == "/bookdrop" {
		b.handleBookdropCommand(message.Chat.ID)
		return
	}

	if text == "/rescan" {
		b.handleRescanCommand(message.Chat.ID)
		return
	}

	if text == "/import" {
		b.handleImportCommand(message.Chat.ID)
		return
	}

	if text == "/debug_bookdrop" {
		b.handleDebugBookdropCommand(message.Chat.ID)
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
/status - Show bot status and settings`

	if b.booklore.IsEnabled() {
		helpText += `
/bookdrop - List all files in bookdrop
/rescan - Scan bookdrop for new files
/import - Select files for import to library`
		// Debug command - not shown in help but available
		// /debug_bookdrop - Test different API endpoints
	}

	helpText += `

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

func (b *Bot) handleBookdropCommand(chatID int64) {
	if !b.booklore.IsEnabled() {
		msg := tgbotapi.NewMessage(chatID, "❌ Booklore integration is not enabled. Please configure the API token.")
		b.api.Send(msg)
		return
	}

	// Send typing indicator to show we're working
	action := tgbotapi.NewChatAction(chatID, "typing")
	b.api.Send(action)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get all files from bookdrop (no status filter)
	b.config.Logger.Info("Fetching bookdrop files",
		zap.String("api_url", b.config.BookloreAPI.APIURL))

	files, err := b.booklore.GetBookdropFilesNoStatus(ctx, 0, 50) // Get up to 50 files
	if err != nil {
		b.config.Logger.Error("Failed to get bookdrop files",
			zap.Error(err),
			zap.String("api_url", b.config.BookloreAPI.APIURL))
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Failed to retrieve bookdrop files: %s", err.Error()))
		b.api.Send(msg)
		return
	}

	b.config.Logger.Info("Bookdrop files retrieved",
		zap.Int("total_files", files.TotalElements),
		zap.Int("files_in_content", len(files.Content)))

	if files.TotalElements == 0 {
		msg := tgbotapi.NewMessage(chatID, "📂 Bookdrop is empty. No files found.")
		b.api.Send(msg)
		return
	}

	// Format and send the bookdrop contents
	message := fmt.Sprintf("📂 *Bookdrop Contents*\n\nFound %d files:\n\n", files.TotalElements)

	for i, file := range files.Content {
		status := "⏳ " + file.Status
		if file.Status == "NEW" {
			status = "🆕 " + file.Status
		} else if file.Status == "IMPORTED" {
			status = "✅ " + file.Status
		} else if file.Status == "FAILED" {
			status = "❌ " + file.Status
		}

		message += fmt.Sprintf("%d. %s\n   📄 %s\n   📏 %d KB\n   📅 %s\n\n",
			i+1, status, file.FileName, file.FileSize/1024, file.DateAdded)

		// Split long messages to avoid Telegram limits
		if len(message) > 3500 {
			msg := tgbotapi.NewMessage(chatID, message)
			msg.ParseMode = "Markdown"
			b.api.Send(msg)
			message = ""
		}
	}

	if message != "" {
		msg := tgbotapi.NewMessage(chatID, message)
		msg.ParseMode = "Markdown"
		b.api.Send(msg)
	}

	// Add suggestion for import
	if files.TotalElements > 0 {
		hint := tgbotapi.NewMessage(chatID,
			"💡 Use /rescan to refresh the bookdrop or /import to select files for import.")
		b.api.Send(hint)
	}
}

func (b *Bot) handleRescanCommand(chatID int64) {
	if !b.booklore.IsEnabled() {
		msg := tgbotapi.NewMessage(chatID, "❌ Booklore integration is not enabled. Please configure the API token.")
		b.api.Send(msg)
		return
	}

	// Send typing indicator
	action := tgbotapi.NewChatAction(chatID, "typing")
	b.api.Send(action)

	msg := tgbotapi.NewMessage(chatID, "🔄 Scanning bookdrop folder for new files...")
	b.api.Send(msg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := b.booklore.RescanBookdrop(ctx); err != nil {
		b.config.Logger.Error("Failed to rescan bookdrop",
			zap.Error(err))
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Failed to scan bookdrop: %s", err.Error()))
		b.api.Send(errorMsg)
		return
	}

	successMsg := tgbotapi.NewMessage(chatID, "✅ Bookdrop folder scanned successfully!\n\n💡 Use /bookdrop to see the updated contents.")
	b.api.Send(successMsg)
}

func (b *Bot) handleImportCommand(chatID int64) {
	if !b.booklore.IsEnabled() {
		msg := tgbotapi.NewMessage(chatID, "❌ Booklore integration is not enabled. Please configure the API token.")
		b.api.Send(msg)
		return
	}

	// Send typing indicator
	action := tgbotapi.NewChatAction(chatID, "typing")
	b.api.Send(action)

	msg := tgbotapi.NewMessage(chatID, "🔄 Preparing import options...")
	b.api.Send(msg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get only NEW files for import
	files, err := b.booklore.GetBookdropFiles(ctx, "NEW", 0, 20) // Get up to 20 new files
	if err != nil {
		b.config.Logger.Error("Failed to get bookdrop files for import",
			zap.Error(err))
		errorMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Failed to retrieve files for import: %s", err.Error()))
		b.api.Send(errorMsg)
		return
	}

	if files.TotalElements == 0 {
		msg := tgbotapi.NewMessage(chatID, "📂 No new files found in bookdrop for import.\n\n💡 Use /rescan to check for new files, or /bookdrop to see all files.")
		b.api.Send(msg)
		return
	}

	// Create inline keyboard for file selection
	var keyboard [][]tgbotapi.InlineKeyboardButton

	for i, file := range files.Content {
		if i >= 10 { // Limit to 10 files to keep message manageable
			break
		}

		buttonText := fmt.Sprintf("📄 %s (%.1f MB)",
			truncateString(file.FileName, 40),
			float64(file.FileSize)/1024/1024)

		callbackData := fmt.Sprintf("import_%d", file.ID)

		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			{Text: buttonText, CallbackData: &callbackData},
		})
	}

	// Add "Import All" and "Cancel" buttons
	if files.TotalElements > 1 {
		importAllBtn := tgbotapi.InlineKeyboardButton{
			Text:         "📥 Import All",
			CallbackData: &[]string{"import_all"}[0],
		}
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{importAllBtn})
	}

	cancelBtn := tgbotapi.InlineKeyboardButton{
		Text:         "❌ Cancel",
		CallbackData: &[]string{"import_cancel"}[0],
	}
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{cancelBtn})

	replyMarkup := tgbotapi.NewInlineKeyboardMarkup(keyboard...)

	message := fmt.Sprintf("📥 *Select files to import*\n\nFound %d new files:\n\n💡 Choose files below or use 📥 Import All for all new files.",
		files.TotalElements)

	telegramMsg := tgbotapi.NewMessage(chatID, message)
	telegramMsg.ParseMode = "Markdown"
	telegramMsg.ReplyMarkup = replyMarkup
	b.api.Send(telegramMsg)
}

func (b *Bot) handleDebugBookdropCommand(chatID int64) {
	if !b.booklore.IsEnabled() {
		msg := tgbotapi.NewMessage(chatID, "❌ Booklore integration is not enabled.")
		b.api.Send(msg)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	debugMsg := "🔍 *Bookdrop Debug Information*\n\n"

	// Test different API endpoints
	testCases := []struct {
		name   string
		status string
	}{
		{"No status filter", ""},
		{"NEW status", "NEW"},
		{"PROCESSED status", "PROCESSED"},
		{"IMPORTED status", "IMPORTED"},
		{"FAILED status", "FAILED"},
	}

	for _, tc := range testCases {
		debugMsg += fmt.Sprintf("📋 Testing: %s\n", tc.name)

		var err error
		var result *booklore.PageBookdropFile

		if tc.status == "" {
			result, err = b.booklore.GetBookdropFilesNoStatus(ctx, 0, 10)
		} else {
			result, err = b.booklore.GetBookdropFiles(ctx, tc.status, 0, 10)
		}

		if err != nil {
			debugMsg += fmt.Sprintf("   ❌ Error: %s\n\n", err.Error())
		} else {
			debugMsg += fmt.Sprintf("   ✅ Success: %d total files, %d in response\n\n",
				result.TotalElements, len(result.Content))
		}
	}

	// Also test the notification endpoint
	debugMsg += "📊 Testing notification endpoint...\n"
	notification, err := b.booklore.GetBookdropNotification(ctx)
	if err != nil {
		debugMsg += fmt.Sprintf("   ❌ Error: %s\n", err.Error())
	} else {
		debugMsg += fmt.Sprintf("   ✅ Success: Total: %d, New: %d, Imported: %d\n",
			notification.TotalFiles, notification.NewFiles, notification.ImportedFiles)
	}

	// Truncate if too long for Telegram
	if len(debugMsg) > 4000 {
		debugMsg = debugMsg[:3950] + "\n... (truncated)"
	}

	msg := tgbotapi.NewMessage(chatID, debugMsg)
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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// First rescan the bookdrop folder
	if err := b.booklore.RescanBookdrop(ctx); err != nil {
		b.config.Logger.Error("Failed to rescan bookdrop folder",
			zap.String("filename", filename),
			zap.Error(err))
		return fmt.Sprintf("📥 File downloaded, but failed to trigger Booklore scan: %s", err.Error())
	}

	// Wait a moment for Booklore to process the file, then retry import
	maxRetries := 3
	retryDelay := 3 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			b.config.Logger.Info("Waiting before retry",
				zap.String("filename", filename),
				zap.Int("attempt", attempt+1),
				zap.Duration("delay", retryDelay))

			select {
			case <-ctx.Done():
				return "📥 File downloaded, but Booklore import timed out"
			case <-time.After(retryDelay):
			}
		}

		// Finalize all imports
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
				zap.Int("imported_count", result.ImportedCount),
				zap.Int("attempt", attempt+1))
			return fmt.Sprintf("📚 File downloaded and imported to Booklore successfully! (%d books imported)", result.ImportedCount)
		}

		// If no files were imported on this attempt, try again
		b.config.Logger.Info("No files imported on this attempt, retrying",
			zap.String("filename", filename),
			zap.Int("attempt", attempt+1),
			zap.Int("remaining_attempts", maxRetries-attempt-1))
	}

	// All retries exhausted
	b.config.Logger.Info("Booklore import completed after retries",
		zap.String("filename", filename),
		zap.Int("total_attempts", maxRetries))
	return "📥 File downloaded to bookdrop, but no new books were imported after multiple attempts"
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

// Helper function to truncate strings
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	if maxLength <= 3 {
		return s[:maxLength]
	}
	return s[:maxLength-3] + "..."
}

// handleImportCallback handles callback queries from inline keyboards
func (b *Bot) handleImportCallback(callback *tgbotapi.CallbackQuery) {
	if !b.booklore.IsEnabled() {
		callbackResponse := tgbotapi.NewCallback(callback.ID, "Booklore integration is not enabled")
		b.api.Request(callbackResponse)
		return
	}

	data := callback.Data
	chatID := callback.Message.Chat.ID

	if data == "import_cancel" {
		callbackResponse := tgbotapi.NewCallback(callback.ID, "Import cancelled")
		b.api.Request(callbackResponse)

		editMsg := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, "❌ Import cancelled")
		b.api.Send(editMsg)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if data == "import_all" {
		callbackResponse := tgbotapi.NewCallback(callback.ID, "Importing all new files...")
		b.api.Request(callbackResponse)

		// Show processing message
		editMsg := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, "📥 Importing all new files... This may take a moment.")
		b.api.Send(editMsg)

		// Get all new files
		files, err := b.booklore.GetBookdropFiles(ctx, "NEW", 0, 100)
		if err != nil {
			b.api.Request(tgbotapi.NewCallback(callback.ID, "Failed to get files"))
			editMsg := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, fmt.Sprintf("❌ Failed to get files: %s", err.Error()))
			b.api.Send(editMsg)
			return
		}

		if len(files.Content) == 0 {
			editMsg := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, "📂 No new files found to import.")
			b.api.Send(editMsg)
			return
		}

		// Extract file IDs
		fileIDs := make([]int64, len(files.Content))
		for i, file := range files.Content {
			fileIDs[i] = file.ID
		}

		// Import all files
		result, err := b.booklore.FinalizeImport(ctx, fileIDs)
		if err != nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, fmt.Sprintf("❌ Import failed: %s", err.Error()))
			b.api.Send(editMsg)
			return
		}

		successMessage := fmt.Sprintf("✅ Import completed!\n\n📊 Results:\n📥 Imported: %d\n❌ Failed: %d",
			result.ImportedCount, result.FailedCount)

		editMsg = tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, successMessage)
		b.api.Send(editMsg)
		return
	}

	// Handle individual file import
	if strings.HasPrefix(data, "import_") {
		var fileID int64
		_, err := fmt.Sscanf(data, "import_%d", &fileID)
		if err != nil {
			b.api.Request(tgbotapi.NewCallback(callback.ID, "Invalid file ID"))
			return
		}

		callbackResponse := tgbotapi.NewCallback(callback.ID, "Importing selected file...")
		b.api.Request(callbackResponse)

		// Show processing message
		editMsg := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, "📥 Importing selected file...")
		b.api.Send(editMsg)

		// Import the specific file
		result, err := b.booklore.FinalizeImport(ctx, []int64{fileID})
		if err != nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, fmt.Sprintf("❌ Import failed: %s", err.Error()))
			b.api.Send(editMsg)
			return
		}

		var successMessage string
		if result.ImportedCount > 0 {
			successMessage = "✅ File imported successfully! 📚"
		} else if result.FailedCount > 0 {
			successMessage = "❌ File import failed"
		} else {
			successMessage = "ℹ️ No files were imported"
		}

		editMsg = tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, successMessage)
		b.api.Send(editMsg)
	}
}
