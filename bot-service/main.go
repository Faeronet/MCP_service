package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/logging"
	"github.com/telegram-ai-assistant/root/pkg/queue"
	"github.com/telegram-ai-assistant/root/pkg/ratelimit"
	"github.com/telegram-ai-assistant/root/pkg/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var log = logging.New("bot-service")

func main() {
	ctx := context.Background()
	token := config.LoadString("TELEGRAM_BOT_TOKEN", "")
	if token == "" {
		log.Warn(ctx, "TELEGRAM_BOT_TOKEN not set; bot will not receive updates")
	}

	pool, err := pgxpool.New(ctx, config.LoadPostgres().DSN)
	if err != nil {
		log.Error(ctx, "postgres connect", logging.KV{"error", err})
		os.Exit(1)
	}
	defer pool.Close()

	minioCfg := config.LoadMinIO()
	minioClient, err := storage.New(ctx, storage.Config{
		Endpoint:  minioCfg.Endpoint,
		AccessKey: minioCfg.AccessKey,
		SecretKey: minioCfg.SecretKey,
		Bucket:    minioCfg.AttachmentsBucket,
		UseSSL:    minioCfg.UseSSL,
	})
	if err != nil {
		log.Error(ctx, "minio connect", logging.KV{"error", err})
		os.Exit(1)
	}

	var rmq *queue.Client
	rmqURL := config.LoadRabbitMQ().URL
	for i := 0; i < 30; i++ {
		rmq, err = queue.New(ctx, rmqURL)
		if err == nil {
			break
		}
		log.Warn(ctx, "rabbitmq connect retry", logging.KV{"error", err}, logging.KV{"attempt", i + 1})
		time.Sleep(2 * time.Second)
	}
	if rmq == nil {
		log.Error(ctx, "rabbitmq connect failed after retries", logging.KV{"error", err})
		os.Exit(1)
	}
	defer rmq.Close()

	workerConcurrency := config.LoadInt("WORKER_CONCURRENCY", 64)
	maxInflightLLM := config.LoadInt("MAX_INFLIGHT_LLM", 32)
	deboounceMs := config.LoadInt("PER_CHAT_DEBOUNCE_MS", 500)
	mcpReadURL := config.LoadString("MCP_READ_URL", "http://mcp-read:8082")
	extractToolURL := strings.TrimSuffix(config.LoadString("EXTRACT_TOOL_URL", config.LoadString("OCR_SERVICE_URL", config.LoadString("ASR_SERVICE_URL", "http://extract-tool:8004"))), "/")
	vllmBase := config.LoadString("LLM_BINDING_HOST", "")
	if vllmBase == "" {
		vllmBase = config.LoadString("VLLM_OPENAI_BASE", "http://vllm:8000/v1")
	}
	vllmBase = strings.TrimSuffix(vllmBase, "/")
	llmModel := config.LoadString("LLM_MODEL", "Qwen/Qwen3-0.6B")
	llmAPIKey := config.LoadString("LLM_BINDING_API_KEY", "")

	llmLimiter := ratelimit.NewInFlight(maxInflightLLM)
	perChatLimiter := ratelimit.NewPerKey(5, 1*time.Minute)

	app := &Bot{
		bot:           nil, // set below if token present
		pool:          pool,
		minio:         minioClient,
		queue:         rmq,
		mcpReadURL:    mcpReadURL,
		extractToolURL: extractToolURL,
		vllmBase:      vllmBase,
		llmModel:      llmModel,
		llmAPIKey:     llmAPIKey,
		llmLimiter:    llmLimiter,
		perChatLimiter: perChatLimiter,
		debounce:      time.Duration(deboounceMs) * time.Millisecond,
		chatMu:        make(map[int64]chan struct{}),
		chatMuGuard:   &sync.Mutex{},
	}

	// Worker pool for updates
	updatesCh := make(chan tgbotapi.Update, 512)
	for i := 0; i < workerConcurrency; i++ {
		go app.worker(updatesCh)
	}

	// HTTP health
	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
		_ = http.ListenAndServe(":8081", nil)
	}()

	if token == "" {
		log.Info(ctx, "bot idle (no token); health on :8081")
		select {}
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Error(ctx, "telegram bot init", logging.KV{"error", err})
		os.Exit(1)
	}
	bot.Debug = config.LoadBool("TELEGRAM_DEBUG", false)
	app.bot = bot
	log.Info(ctx, "bot authorized", logging.KV{"username", bot.Self.UserName})

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	go func() {
		for update := range updates {
			select {
			case updatesCh <- update:
			default:
				log.Warn(ctx, "updates queue full, dropping", logging.KV{"chat_id", getChatID(update)})
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info(ctx, "shutting down")
}

func getChatID(u tgbotapi.Update) int64 {
	if u.Message != nil {
		return u.Message.Chat.ID
	}
	if u.CallbackQuery != nil {
		return u.CallbackQuery.Message.Chat.ID
	}
	return 0
}

type Bot struct {
	bot            *tgbotapi.BotAPI
	pool           *pgxpool.Pool
	minio          *storage.Client
	queue          *queue.Client
	mcpReadURL     string
	extractToolURL string
	vllmBase       string
	llmModel       string
	llmAPIKey      string
	llmLimiter     *ratelimit.InFlight
	perChatLimiter *ratelimit.PerKey
	debounce       time.Duration
	chatMu         map[int64]chan struct{}
	chatMuGuard    *sync.Mutex
}

func (b *Bot) serializedChat(chatID int64, fn func()) {
	b.chatMuGuard.Lock()
	c, ok := b.chatMu[chatID]
	if !ok {
		c = make(chan struct{}, 1)
		c <- struct{}{}
		b.chatMu[chatID] = c
	}
	b.chatMuGuard.Unlock()
	<-c
	fn()
	c <- struct{}{}
}

func (b *Bot) worker(updatesCh <-chan tgbotapi.Update) {
	for u := range updatesCh {
		b.handleUpdate(u)
	}
}

func (b *Bot) handleUpdate(u tgbotapi.Update) {
	ctx := context.Background()
	reqID := uuid.New().String()
	ctx = logging.WithRequestID(ctx, reqID)

	if u.Message == nil {
		return
	}
	chatID := u.Message.Chat.ID
	userID := u.Message.From.ID

	// Debounce: skip if we already have recent activity for this chat (simplified: serialize per chat)
	b.serializedChat(chatID, func() {
		time.Sleep(b.debounce)
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

	key := fmt.Sprintf("user:%d", userID)
	if !b.perChatLimiter.Allow(key) {
		log.Warn(ctx, "per-user rate limit", logging.KV{"user_id", userID})
		// could send "slow down" to user
		return
	}

	if err := b.llmLimiter.Acquire(ctx); err != nil {
		log.Warn(ctx, "llm queue full", logging.KV{"error", err})
		return
	}
	defer b.llmLimiter.Release()

	sessionID, err := b.ensureSession(ctx, chatID, userID, msg.Chat.UserName)
	if err != nil {
		log.Error(ctx, "ensure session", logging.KV{"error", err})
		return
	}
	_ = b.appendMessage(ctx, sessionID, "user", msg.Text)

	attachmentsText := b.getAttachmentsText(ctx, sessionID)
	contextText, err := b.buildContext(ctx, requestID, msg.Text, attachmentsText, 4000, "default")
	if err != nil {
		log.Warn(ctx, "build_context failed, using empty context", logging.KV{"error", err})
		contextText = ""
	}

	reply, err := b.callLLM(ctx, requestID, contextText, msg.Text)
	if err != nil {
		log.Error(ctx, "llm call", logging.KV{"error", err}, logging.KV{"vllm_base", b.vllmBase})
		// Краткая подсказка пользователю; полная ошибка — в логах бота
		hint := "Модель недоступна. Проверьте, что vLLM запущен (docker compose --profile vllm up -d) и в .env указан VLLM_OPENAI_BASE."
		if errStr := err.Error(); len(errStr) < 120 {
			hint = "Ошибка LLM: " + errStr
		}
		b.sendReply(ctx, chatID, hint)
		return
	}
	_ = b.appendMessage(ctx, sessionID, "assistant", reply)
	b.sendReply(ctx, chatID, reply)
}

func (b *Bot) ensureSession(ctx context.Context, chatID, userID int64, username string) (uuid.UUID, error) {
	var id uuid.UUID
	err := b.pool.QueryRow(ctx, `
		INSERT INTO chat.sessions (telegram_id, chat_id, last_active)
		VALUES ($1, $2, NOW())
		ON CONFLICT (telegram_id, chat_id) DO UPDATE SET last_active = NOW()
		RETURNING id
	`, userID, chatID).Scan(&id)
	return id, err
}

func (b *Bot) appendMessage(ctx context.Context, sessionID uuid.UUID, role, content string) error {
	_, err := b.pool.Exec(ctx, `INSERT INTO chat.messages (session_id, role, content) VALUES ($1, $2, $3)`, sessionID, role, content)
	return err
}

// getAttachmentsText возвращает объединённый текст из вложений сессии (OCR/ASR), чтобы передать в build_context.
func (b *Bot) getAttachmentsText(ctx context.Context, sessionID uuid.UUID) string {
	rows, err := b.pool.Query(ctx,
		`SELECT extracted_text FROM chat.attachments WHERE session_id = $1 AND status = 'done' AND extracted_text IS NOT NULL AND extracted_text != '' ORDER BY created_at DESC LIMIT 10`,
		sessionID)
	if err != nil {
		return ""
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil && t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n\n")
}

func (b *Bot) buildContext(ctx context.Context, requestID, query, attachmentsText string, tokenBudget int, mode string) (string, error) {
	body := map[string]interface{}{
		"query_text":          query,
		"acl_token":           "placeholder",
		"token_budget":        tokenBudget,
		"mode":                mode,
		"attachments_text":    attachmentsText,
	}
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, b.mcpReadURL+"/mcp/build_context", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", requestID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bb, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("mcp-read %d: %s", resp.StatusCode, string(bb))
	}
	var out struct {
		Context string `json:"context"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", fmt.Errorf("%s", out.Error)
	}
	return out.Context, nil
}

func (b *Bot) callLLM(ctx context.Context, requestID, contextText, userQuery string) (string, error) {
	// OpenAI-compatible chat completion to vLLM
	messages := []map[string]string{
		{"role": "system", "content": "You are a helpful assistant. Use the following context to answer.\n\n" + contextText},
		{"role": "user", "content": userQuery},
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"model": b.llmModel, "messages": messages, "max_tokens": 512,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, b.vllmBase+"/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", requestID)
	if b.llmAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+b.llmAPIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bb, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("vllm %d: %s", resp.StatusCode, string(bb))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("no choices")
	}
	return out.Choices[0].Message.Content, nil
}

func (b *Bot) sendReply(ctx context.Context, chatID int64, text string) {
	if b.bot == nil {
		return
	}
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.bot.Send(msg); err != nil {
		log.Warn(ctx, "send reply", logging.KV{"error", err}, logging.KV{"chat_id", chatID})
	}
}

func (b *Bot) handleAttachment(ctx context.Context, u tgbotapi.Update, chatID int64, requestID string) {
	msg := u.Message
	if b.bot == nil {
		return
	}
	userID := int64(0)
	username := ""
	if msg.From != nil {
		userID = msg.From.ID
		if msg.From.UserName != "" {
			username = msg.From.UserName
		}
	}
	sessionID, err := b.ensureSession(ctx, chatID, userID, username)
	if err != nil {
		log.Warn(ctx, "ensure session for attachment", logging.KV{"error", err})
		b.sendReply(ctx, chatID, "Ошибка сессии.")
		return
	}
	var fileID string
	var objectKey string
	var fileName string
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
	file, err := b.bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		log.Warn(ctx, "telegram get file", logging.KV{"error", err}, logging.KV{"file_id", fileID})
		b.sendReply(ctx, chatID, "Не удалось получить файл.")
		return
	}
	downloadURL := "https://api.telegram.org/file/bot" + b.bot.Token + "/" + file.FilePath
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Warn(ctx, "download file from telegram", logging.KV{"error", err})
		b.sendReply(ctx, chatID, "Не удалось скачать файл.")
		return
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Warn(ctx, "read telegram file body", logging.KV{"error", err})
		b.sendReply(ctx, chatID, "Ошибка чтения файла.")
		return
	}
	if _, err := b.minio.Put(ctx, objectKey, bytes.NewReader(data), "application/octet-stream", int64(len(data))); err != nil {
		log.Warn(ctx, "minio put attachment", logging.KV{"error", err}, logging.KV{"object_key", objectKey})
		b.sendReply(ctx, chatID, "Ошибка сохранения файла.")
		return
	}
	// Извлечение текста через extract-tool (/extract: текст, PDF, OCR, ASR, архивы)
	extracted, userErr, err := b.callExtract(ctx, data, fileName)
	if err != nil {
		log.Warn(ctx, "extract failed", logging.KV{"error", err})
		b.sendReply(ctx, chatID, "Не удалось обработать файл.")
		return
	}
	if userErr != "" {
		b.sendReply(ctx, chatID, userErr)
		return
	}
	_, err = b.pool.Exec(ctx,
		`INSERT INTO chat.attachments (session_id, object_key, extracted_text, status) VALUES ($1, $2, $3, 'done')`,
		sessionID, objectKey, extracted)
	if err != nil {
		log.Warn(ctx, "insert attachment", logging.KV{"error", err})
	}
	// Отправка извлечённого текста в LLM и ответ пользователю ответом модели (не сырым текстом)
	userMsg := "Обработай следующий текст из вложения и ответь по существу:\n\n" + extracted
	if strings.TrimSpace(extracted) == "" {
		b.sendReply(ctx, chatID, "Файл обработан, текст не извлечён.")
		log.Info(ctx, "attachment processed (no text)", logging.KV{"chat_id", chatID}, logging.KV{"object_key", objectKey})
		return
	}
	key := fmt.Sprintf("user:%d", userID)
	if !b.perChatLimiter.Allow(key) {
		log.Warn(ctx, "per-user rate limit (attachment)", logging.KV{"user_id", userID})
		b.sendReply(ctx, chatID, "Слишком много запросов, подождите.")
		return
	}
	_ = b.appendMessage(ctx, sessionID, "user", userMsg)
	attachmentsText := b.getAttachmentsText(ctx, sessionID)
	contextText, err := b.buildContext(ctx, requestID, userMsg, attachmentsText, 4000, "default")
	if err != nil {
		log.Warn(ctx, "build_context for attachment failed", logging.KV{"error", err})
		contextText = ""
	}
	if err := b.llmLimiter.Acquire(ctx); err != nil {
		log.Warn(ctx, "llm queue full for attachment", logging.KV{"error", err})
		b.sendReply(ctx, chatID, "Сервер занят, попробуйте позже.")
		return
	}
	defer b.llmLimiter.Release()
	reply, err := b.callLLM(ctx, requestID, contextText, userMsg)
	if err != nil {
		log.Warn(ctx, "llm call for attachment", logging.KV{"error", err})
		b.sendReply(ctx, chatID, "Не удалось получить ответ модели. Извлечённый текст сохранён в контексте чата.")
		return
	}
	_ = b.appendMessage(ctx, sessionID, "assistant", reply)
	// Показываем пользователю, что извлекли (фрагмент) и ответ модели. Лимит Telegram 4096.
	const previewLen = 350
	const maxReplyLen = 4000
	replyToUser := reply
	if len(extracted) > 0 {
		preview := extracted
		if len(preview) > previewLen {
			preview = preview[:previewLen] + "..."
		}
		replyToUser = "Извлечённый текст (начало):\n" + preview + "\n\nОтвет:\n" + reply
		if len(replyToUser) > maxReplyLen {
			replyToUser = replyToUser[:maxReplyLen-20] + "\n..."
		}
	}
	b.sendReply(ctx, chatID, replyToUser)
	log.Info(ctx, "attachment processed and LLM replied", logging.KV{"chat_id", chatID}, logging.KV{"object_key", objectKey})
}

// callExtract отправляет файл в extract-tool (POST /extract). Возвращает (текст, сообщениеДляПользователя, ошибка).
// При 422 сообщение для пользователя (например "Файл или архив нельзя обработать.") в userErr.
func (b *Bot) callExtract(ctx context.Context, data []byte, fileName string) (text, userErr string, err error) {
	if b.extractToolURL == "" {
		return "", "", fmt.Errorf("EXTRACT_TOOL_URL / OCR_SERVICE_URL not set")
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return "", "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", "", err
	}
	if err := w.Close(); err != nil {
		return "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.extractToolURL+"/extract", &buf)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 422 {
		var errBody struct {
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(bodyBytes, &errBody)
		msg := errBody.Detail
		if msg == "" {
			msg = "Файл или архив нельзя обработать."
		}
		return "", msg, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("extract %d: %s", resp.StatusCode, string(bodyBytes))
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return "", "", err
	}
	return out.Text, "", nil
}
