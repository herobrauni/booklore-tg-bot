# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Telegram File Downloader Bot written in Go that downloads files from authorized users to a specified folder. The bot supports multiple file types (documents, photos, audio, video) and includes security features like user authentication, file type filtering, and size limits.

**Booklore Integration**: The bot now supports automatic integration with Booklore servers, allowing downloaded books to be automatically imported into the Booklore library via API calls.

## Development Commands

### Building and Running
```bash
# Install dependencies
go mod download

# Run the bot locally
go run cmd/bot/main.go

# Build binary
go build -o bot ./cmd/bot
./bot

# Run tests (if any exist)
go test ./...
```

### Docker Development
```bash
# Build Docker image
docker build -t booklore-tg-bot .

# Run with Docker Compose
docker-compose up -d

# View logs
docker-compose logs telegram-bot

# Stop container
docker-compose down
```

## Architecture

The project follows a clean architecture pattern with clear separation of concerns:

### Core Components

- **`cmd/bot/main.go`**: Application entry point with graceful shutdown handling
- **`internal/bot/`**: Main bot logic and message handlers
  - `bot.go`: Bot initialization and update processing with Booklore client
  - `handlers.go`: Message type handlers (documents, photos, audio, video, text) with Booklore integration
- **`internal/config/`**: Configuration management with environment variable loading including Booklore API settings
- **`internal/auth/`**: User authentication for access control
- **`internal/downloader/`**: File download functionality with validation
- **`internal/booklore/`**: Booklore API client package
  - `client.go`: HTTP client with Bearer token authentication for Booklore API
  - `types.go`: API request/response data structures
  - `errors.go`: Custom error handling for API interactions

### Key Dependencies
- `github.com/go-telegram-bot-api/telegram-bot-api/v5` - Telegram Bot API client
- `go.uber.org/zap` - Structured logging
- `github.com/joho/godotenv` - Environment variable loading for local development

### Message Flow
1. Telegram sends updates to bot via long polling
2. Bot authenticates user in `auth/auth.go:IsUserAllowed()`
3. Message is routed to appropriate handler in `bot/handlers.go`
4. File validation happens in `downloader/downloader.go`
5. Files are downloaded to configured folder with unique naming
6. **Booklore Integration** (if enabled):
   - Bot calls Booklore API to rescan bookdrop folder
   - Bot triggers finalization to import files to main library
   - User receives status feedback on both download and import

### Booklore API Integration
The bot integrates with Booklore via REST API calls:
- **Authentication**: Bearer token authentication
- **Endpoints Used**:
  - `POST /api/v1/bookdrop/rescan`: Scan bookdrop folder for new files
  - `POST /api/v1/bookdrop/imports/finalize`: Import scanned files to library
- **Error Handling**: Graceful degradation if API calls fail
- **User Feedback**: Import status included in success messages

### Configuration
All configuration is handled via environment variables:

#### Bot Configuration
- `TELEGRAM_BOT_TOKEN`: Bot token from @BotFather (required)
- `ALLOWED_USER_IDS`: Comma-separated user IDs (required)
- `DOWNLOAD_FOLDER`: Download directory (default: "downloads")
- `ALLOWED_FILE_TYPES`: Comma-separated file extensions
- `MAX_FILE_SIZE_MB`: Maximum file size in MB (default: 20)

#### Booklore API Integration (Optional)
- `BOOKLORE_API_URL`: Booklore server URL (default: https://booklore.brauni.dev)
- `BOOKLORE_API_TOKEN`: Bearer token for Booklore API authentication
- `BOOKLORE_AUTO_IMPORT`: Enable automatic import after download (default: true when token provided)

**Note**: Booklore integration is only enabled when `BOOKLORE_API_TOKEN` is provided.

### File Handling Features
- Unique filename generation to prevent overwrites
- File size validation before and after download
- File type extension filtering
- Support for documents, photos, audio, video, and voice messages
- Error handling with user-friendly messages
- **Automatic Booklore library import** (when configured)
- Import status feedback with detailed success/failure information

### Deployment
- Docker multi-stage build (Go builder + Debian slim runtime)
- Non-root user execution for security
- GitHub Actions CI/CD for automated Docker builds
- Pre-built images available at `ghcr.io/brauni/booklore-tg-bot:latest`

## Security Considerations
- User whitelist authentication (only allowed users can access)
- File type restrictions prevent malicious uploads
- File size limits prevent abuse
- Non-root Docker execution
- Input validation throughout the codebase

## Testing Environment Variables
Create a `.env` file for local development:

```bash
# Required Bot Configuration
TELEGRAM_BOT_TOKEN=your_test_token
ALLOWED_USER_IDS=your_user_id

# Optional Bot Configuration
DOWNLOAD_FOLDER=downloads
ALLOWED_FILE_TYPES=.pdf,.txt,.jpg,.epub,.mobi
MAX_FILE_SIZE_MB=10

# Optional Booklore Integration
BOOKLORE_API_URL=https://your-booklore-server.com
BOOKLORE_API_TOKEN=your_bearer_token_here
BOOKLORE_AUTO_IMPORT=true
```

## Common Issues
- Files becoming unavailable on Telegram servers (handled with fallback URLs)
- Rate limiting from Telegram API (handled with retry messaging)
- Permission issues with download folders (handled with proper directory creation)
- **Booklore API failures**: Bot continues to download files even if API calls fail, with appropriate error messages sent to users
- **Authentication errors**: Invalid Booklore API tokens are handled gracefully with user feedback

## Troubleshooting Booklore Integration
1. **API not working**: Verify `BOOKLORE_API_TOKEN` is valid and has proper permissions
2. **No auto-import**: Check that `BOOKLORE_AUTO_IMPORT=true` and the download folder matches Booklore's bookdrop location
3. **Import failures**: Check Booklore server logs for file processing errors
4. **Network issues**: Verify bot can reach the Booklore server URL specified in `BOOKLORE_API_URL`