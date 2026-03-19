package modules

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/telegram-ai-assistant/root/pkg/logging"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const welcomeMessage = "Привет! Я ИИ-ассистент, отвечаю на вопросы по книге «Книга ангелов»."
const chatAlreadyStartedMessage = "Чат уже запущен."
const chatResetMessage = "Чат сброшен. Отправьте /start для начала."
const previewLen = 350

var logHandler = logging.New("tg-bot.handler")

func isStartCommand(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	return t == "/start" || strings.HasPrefix(t, "/start ")
}

func isRestartCommand(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	return t == "/restart" || t == "/reset" || strings.HasPrefix(t, "/restart ") || strings.HasPrefix(t, "/reset ")
}

// SerializedChat runs fn with per-chat serialization and debounce.
func (b *Bot) SerializedChat(chatID int64, fn func()) {
	b.ChatMuGuard.Lock()
	c, ok := b.ChatMu[chatID]
	if !ok {
		c = make(chan struct{}, 1)
		c <- struct{}{}
		b.ChatMu[chatID] = c
	}
	b.ChatMuGuard.Unlock()
	<-c
	fn()
	c <- struct{}{}
}

// HandleUpdate routes update to processMessage or attachment.
func (b *Bot) HandleUpdate(u tgbotapi.Update) {
	ctx := context.Background()
	reqID := uuid.New().String()

	if u.Message == nil {
		return
	}
	chatID := u.Message.Chat.ID
	userID := u.Message.From.ID

	b.SerializedChat(chatID, func() {
		time.Sleep(b.Debounce)
		b.processMessage(ctx, u, chatID, userID, reqID)
	})
}

func (b *Bot) processMessage(ctx context.Context, u tgbotapi.Update, chatID int64, userID int64, requestID string) {
	msg := u.Message
	if msg.Document != nil || msg.Photo != nil || msg.Voice != nil {
		b.handleAttachment(ctx, u, chatID, requestID)
		return
	}
	if msg.Text == "" {
		return
	}
	text := strings.TrimSpace(msg.Text)
	if isRestartCommand(text) {
		b.handleRestart(ctx, chatID, userID)
		return
	}
	if isStartCommand(text) {
		b.handleStart(ctx, chatID, userID, msg.Chat.UserName)
		return
	}

	key := fmt.Sprintf("user:%d", userID)
	if b.PerChatLimiter != nil && !b.PerChatLimiter.Allow(key) {
		logHandler.Warn(ctx, "per-user rate limit", logging.KV{"user_id", userID})
		return
	}

	sessionID, err := b.EnsureSession(ctx, chatID, userID, msg.Chat.UserName)
	if err != nil {
		logHandler.Error(ctx, "ensure session", logging.KV{"error", err})
		return
	}
	replyToTgID := 0
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.IsBot {
		replyToTgID = msg.ReplyToMessage.MessageID
	}

	typingMsgID := b.SendReplyTyping(ctx, chatID)

	replyText, debugMessage, messageIDStr, err := b.CallChat(ctx, sessionID, chatID, userID, msg.Chat.UserName, msg.Text, replyToTgID, requestID)
	if err != nil {
		logHandler.Error(ctx, "proxy call", logging.KV{"error", err})
		hint := "Сервис временно недоступен. Запустите mcp-proxy (docker compose up -d mcp-proxy). В контейнерах MCP_PROXY_URL=http://mcp-proxy:8083; с хоста/Cursor — http://127.0.0.1:8083. Не используйте host.docker.internal на Linux к проброшенному порту — часто connection refused."
		if errStr := err.Error(); errStr != "" && len(errStr) < 120 {
			hint += " (" + errStr + ")"
		} else if len(errStr) >= 120 {
			hint += " (" + errStr[:117] + "...)"
		}
		if typingMsgID > 0 {
			b.EditMessageText(ctx, chatID, typingMsgID, hint)
		} else {
			b.SendReply(ctx, chatID, hint)
		}
		return
	}

	if debugMessage != "" {
		b.SendReply(ctx, chatID, debugMessage)
	}
	firstMsgID := b.SendLongReply(ctx, chatID, typingMsgID, replyText)
	if messageIDStr != "" {
		if msgID, err := uuid.Parse(messageIDStr); err == nil && firstMsgID > 0 {
			_ = b.UpdateMessageTelegramID(ctx, msgID, firstMsgID)
		}
	}
}

func (b *Bot) handleStart(ctx context.Context, chatID, userID int64, username string) {
	var count int64
	err := b.Pool.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM chat.messages m WHERE m.session_id = s.id)
		FROM chat.sessions s WHERE s.telegram_id = $1 AND s.chat_id = $2
	`, userID, chatID).Scan(&count)
	if err == nil && count > 0 {
		b.SendReply(ctx, chatID, chatAlreadyStartedMessage)
		return
	}
	_, _ = b.EnsureSession(ctx, chatID, userID, username)
	b.SendReply(ctx, chatID, welcomeMessage)
}

func (b *Bot) handleRestart(ctx context.Context, chatID, userID int64) {
	_ = b.DeleteSession(ctx, userID, chatID)
	b.SendReply(ctx, chatID, chatResetMessage)
}

func (b *Bot) handleAttachment(ctx context.Context, u tgbotapi.Update, chatID int64, requestID string) {
	msg := u.Message
	if b.Bot == nil {
		return
	}
	userID := int64(0)
	username := ""
	if msg.From != nil {
		userID = msg.From.ID
		username = msg.From.UserName
	}
	sessionID, err := b.EnsureSession(ctx, chatID, userID, username)
	if err != nil {
		logHandler.Warn(ctx, "ensure session for attachment", logging.KV{"error", err})
		b.SendReply(ctx, chatID, "Ошибка сессии.")
		return
	}
	typingMsgID := b.SendReplyTyping(ctx, chatID)

	var fileID, objectKey, fileName string
	if msg.Document != nil {
		fileID = msg.Document.FileID
		fileName = msg.Document.FileName
		objectKey = "attachments/" + requestID + "/" + fileName
	} else if len(msg.Photo) > 0 {
		fileID = msg.Photo[len(msg.Photo)-1].FileID
		fileName = "photo.jpg"
		objectKey = "attachments/" + requestID + "/" + fileName
	} else if msg.Voice != nil {
		fileID = msg.Voice.FileID
		fileName = "voice.ogg"
		objectKey = "attachments/" + requestID + "/" + fileName
	} else {
		return
	}

	file, err := b.Bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		logHandler.Warn(ctx, "telegram get file", logging.KV{"error", err})
		b.EditMessageText(ctx, chatID, typingMsgID, "Не удалось получить файл.")
		return
	}
	downloadURL := "https://api.telegram.org/file/bot" + b.Bot.Token + "/" + file.FilePath
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logHandler.Warn(ctx, "download file from telegram", logging.KV{"error", err})
		b.EditMessageText(ctx, chatID, typingMsgID, "Не удалось скачать файл.")
		return
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		logHandler.Warn(ctx, "read telegram file body", logging.KV{"error", err})
		b.EditMessageText(ctx, chatID, typingMsgID, "Ошибка чтения файла.")
		return
	}

	if _, err := b.Minio.Put(ctx, objectKey, bytes.NewReader(data), "application/octet-stream", int64(len(data))); err != nil {
		logHandler.Warn(ctx, "minio put attachment", logging.KV{"error", err})
		b.SendReply(ctx, chatID, "Ошибка сохранения файла.")
		return
	}

	extracted, userErr, err := b.CallExtract(ctx, data, fileName)
	if err != nil {
		logHandler.Warn(ctx, "extract failed", logging.KV{"error", err})
		b.EditMessageText(ctx, chatID, typingMsgID, "Не удалось обработать файл.")
		return
	}
	if userErr != "" {
		b.EditMessageText(ctx, chatID, typingMsgID, userErr)
		return
	}
	_ = b.InsertAttachment(ctx, sessionID, objectKey, extracted)

	userMsg := "Обработай следующий текст из вложения и ответь по существу:\n\n" + extracted
	if strings.TrimSpace(extracted) == "" {
		b.SendReply(ctx, chatID, "Файл обработан, текст не извлечён.")
		return
	}

	key := fmt.Sprintf("user:%d", userID)
	if b.PerChatLimiter != nil && !b.PerChatLimiter.Allow(key) {
		b.EditMessageText(ctx, chatID, typingMsgID, "Слишком много запросов, подождите.")
		return
	}

	replyText, debugMessage, messageIDStr, err := b.CallChat(ctx, sessionID, chatID, userID, username, userMsg, 0, requestID)
	if err != nil {
		logHandler.Warn(ctx, "proxy call for attachment", logging.KV{"error", err})
		b.EditMessageText(ctx, chatID, typingMsgID, "Не удалось получить ответ модели. Извлечённый текст сохранён в контексте чата.")
		return
	}
	if debugMessage != "" {
		b.SendReply(ctx, chatID, debugMessage)
	}
	replyToUser := replyText
	if len(extracted) > 0 {
		preview := extracted
		if len(preview) > previewLen {
			preview = preview[:previewLen] + "..."
		}
		replyToUser = "Извлечённый текст (начало):\n" + preview + "\n\nОтвет:\n" + replyText
	}
	firstMsgID := b.SendLongReply(ctx, chatID, typingMsgID, replyToUser)
	if messageIDStr != "" {
		if msgID, err := uuid.Parse(messageIDStr); err == nil && firstMsgID > 0 {
			_ = b.UpdateMessageTelegramID(ctx, msgID, firstMsgID)
		}
	}
}
