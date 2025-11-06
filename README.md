# Telegram File Downloader Bot

A simple Telegram bot written in Go that downloads files from authorized users to a specified folder. The bot is containerized with Docker and can be easily deployed.

## Features

- 🤖 **File Downloads**: Download documents, photos, audio, and videos
- 🔐 **User Authentication**: Restrict access to specific Telegram user IDs
- 📁 **Configurable Storage**: Set custom download folder
- 📋 **File Type Filtering**: Restrict allowed file extensions
- 📏 **Size Limits**: Set maximum file size limits
- 🐳 **Docker Ready**: Deploy with Docker and Docker Compose
- 📊 **Status Monitoring**: Bot status and configuration commands

## Quick Start

### 1. Get a Bot Token

1. Start a chat with [@BotFather](https://t.me/botfather) on Telegram
2. Send `/newbot` and follow the instructions
3. Copy the bot token

### 2. Get Your User ID

1. Start a chat with [@userinfobot](https://t.me/userinfobot) on Telegram
2. Send any message to get your user ID

### 3. Configure Environment Variables

Create a `.env` file based on `configs/.env.example`:

```bash
cp configs/.env.example .env
```

Edit `.env` with your values:

```bash
TELEGRAM_BOT_TOKEN=your_bot_token_here
ALLOWED_USER_IDS=123456789,987654321
DOWNLOAD_FOLDER=downloads
ALLOWED_FILE_TYPES=.pdf,.doc,.docx,.txt,.jpg,.jpeg,.png,.zip,.rar
MAX_FILE_SIZE_MB=50
```

### 4. Run with Docker Compose

```bash
docker-compose up -d
```

Your files will be downloaded to the `./downloads` folder on your host machine.

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TELEGRAM_BOT_TOKEN` | Yes | - | Bot token from @BotFather |
| `ALLOWED_USER_IDS` | Yes | - | Comma-separated Telegram user IDs |
| `DOWNLOAD_FOLDER` | No | `/app/downloads` | Download directory inside container |
| `ALLOWED_FILE_TYPES` | No | `.pdf,.doc,.docx,.txt,.jpg,.jpeg,.png,.zip,.rar` | Allowed file extensions |
| `MAX_FILE_SIZE_MB` | No | `20` | Maximum file size in megabytes |

### Adding Multiple Users

Add multiple user IDs as a comma-separated list:

```bash
ALLOWED_USER_IDS=123456789,987654321,555666777
```

## Bot Commands

- `/start` or `/help` - Show help message
- `/status` - Show bot status and configuration

## Usage

1. Send any file (document, photo, audio, video) to the bot
2. If you're authorized and the file meets the criteria, the bot will download it
3. Files are saved with unique names to avoid overwrites
4. You'll receive a confirmation message when the download is complete

## Development

### Building Locally

```bash
# Install dependencies
go mod download

# Run the bot
go run cmd/bot/main.go
```

### Building Binary

```bash
go build -o bot ./cmd/bot
./bot
```

### Docker Commands

```bash
# Build image
docker build -t booklore-tg-bot .

# Run container
docker run -d \
  --name telegram-bot \
  -v $(pwd)/downloads:/app/downloads \
  -e TELEGRAM_BOT_TOKEN=your_token \
  -e ALLOWED_USER_IDS=your_user_id \
  booklore-tg-bot
```

## Project Structure

```
booklore-tg-bot/
├── cmd/bot/                # Application entry point
├── internal/
│   ├── bot/               # Main bot logic and handlers
│   ├── config/            # Configuration management
│   ├── auth/              # User authentication
│   └── downloader/        # File download functionality
├── configs/               # Configuration templates
├── downloads/             # Default download folder
├── Dockerfile            # Docker configuration
├── docker-compose.yml    # Docker Compose setup
└── README.md             # This file
```

## Security Considerations

- Only authorized users can use the bot (whitelist approach)
- File type restrictions prevent malicious file uploads
- File size limits prevent abuse
- Non-root user execution in Docker container
- Input validation and error handling

## Troubleshooting

### Bot Not Responding

1. Check if the bot token is correct
2. Verify the bot is running (`docker-compose logs telegram-bot`)
3. Ensure your user ID is in the allowed list

### Files Not Downloading

1. Check file size limits
2. Verify file type is allowed
3. Check available disk space
4. Review logs for error messages

### Docker Issues

```bash
# View logs
docker-compose logs telegram-bot

# Restart container
docker-compose restart telegram-bot

# Check container status
docker-compose ps
```

## License

This project is open source. Feel free to use, modify, and distribute according to your needs.