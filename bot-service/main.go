package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"regexp"
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

//go:embed prompts/query_extract.txt prompts/answer.txt
var promptFS embed.FS

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
		bot:            nil, // set below if token present
		pool:           pool,
		minio:          minioClient,
		queue:          rmq,
		mcpReadURL:     mcpReadURL,
		extractToolURL: extractToolURL,
		vllmBase:       vllmBase,
		llmModel:       llmModel,
		llmAPIKey:      llmAPIKey,
		llmLimiter:     llmLimiter,
		perChatLimiter: perChatLimiter,
		debounce:       time.Duration(deboounceMs) * time.Millisecond,
		chatMu:         make(map[int64]chan struct{}),
		chatMuGuard:    &sync.Mutex{},
	}
	// Промпты из файлов: A — формулировка запроса для Qdrant, B — ответ по контексту
	if raw, err := promptFS.ReadFile("prompts/query_extract.txt"); err == nil {
		app.promptA = strings.TrimSpace(string(raw))
	}
	if raw, err := promptFS.ReadFile("prompts/answer.txt"); err == nil {
		app.promptB = strings.TrimSpace(string(raw))
	}
	if app.promptA == "" {
		app.promptA = "Сформулируй короткий поисковый запрос по сообщению пользователя для поиска в базе. Только запрос, без пояснений."
	}
	if app.promptB == "" {
		app.promptB = "Ты помощник. Отвечай по контексту ниже. Кратко, на языке вопроса.\n\nКонтекст:"
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
	promptA        string // промпт для выделения поискового запроса (LLM → Qdrant)
	promptB        string // промпт для ответа по контексту (чанки + вопрос)
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
	_, _ = b.appendMessage(ctx, sessionID, "user", msg.Text)
	b.trimSessionMessagesIfNeeded(ctx, sessionID)

	var contextText string
	// Ответ на наше сообщение: берём сохранённый контекст, сразу промпт B (без запроса в Qdrant)
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.IsBot {
		if ctxStored, ok := b.getContextByTelegramMessageID(ctx, sessionID, msg.ReplyToMessage.MessageID); ok && ctxStored != "" {
			contextText = ctxStored
			log.Info(ctx, "using reply-to context", logging.KV{"chat_id", chatID})
		}
	}
	if contextText == "" {
		// Сначала LLM с промптом A: выделяем, что искать в Qdrant
		searchQuery, errExtract := b.extractSearchQuery(ctx, requestID, msg.Text)
		if errExtract != nil {
			log.Warn(ctx, "extract search query failed, using raw message", logging.KV{"error", errExtract})
			searchQuery = msg.Text
		}
		searchQuery = stripThink(searchQuery)
		if searchQuery == "" || strings.EqualFold(strings.TrimSpace(searchQuery), "null") {
			searchQuery = extractDateFromQuestion(msg.Text)
			if searchQuery == "" {
				searchQuery = msg.Text
			}
		}
		// Дебаг: показываем пользователю, что ушло на поиск в Qdrant (не удалять после ответа)
		b.sendReply(ctx, chatID, "🔍 На поиск в Qdrant отправлено:\n"+searchQuery)
		attachmentsText := b.getAttachmentsText(ctx, sessionID)
		var err error
		contextText, err = b.buildContext(ctx, requestID, searchQuery, attachmentsText, 4000, "default")
		if err != nil {
			if err.Error() == "chunk_not_found" {
				b.sendReply(ctx, chatID, "По подходящим данным в базе ничего не найдено.")
				_, _ = b.appendMessage(ctx, sessionID, "assistant", "По подходящим данным в базе ничего не найдено.")
				return
			}
			if err.Error() == "date_not_found" {
				b.sendReply(ctx, chatID, "Данные не найдены.")
				_, _ = b.appendMessage(ctx, sessionID, "assistant", "Данные не найдены.")
				return
			}
			log.Warn(ctx, "build_context failed, using empty context", logging.KV{"error", err})
			contextText = ""
		}
	}

	// Ответ по контексту с промптом B
	systemContent := b.promptB + "\n" + contextText
	reply, err := b.callLLM(ctx, requestID, systemContent, msg.Text)
	if err != nil {
		log.Error(ctx, "llm call", logging.KV{"error", err}, logging.KV{"vllm_base", b.vllmBase})
		hint := "Модель недоступна. Проверьте, что vLLM запущен (docker compose --profile vllm up -d) и в .env указан VLLM_OPENAI_BASE."
		if errStr := err.Error(); len(errStr) < 120 {
			hint = "Ошибка LLM: " + errStr
		}
		b.sendReply(ctx, chatID, hint)
		return
	}
	msgID, _ := b.appendMessage(ctx, sessionID, "assistant", reply)
	b.trimSessionMessagesIfNeeded(ctx, sessionID)
	telegramMsgID := b.sendReplyWithID(ctx, chatID, reply)
	if msgID != uuid.Nil && telegramMsgID > 0 {
		_ = b.updateMessageTelegramID(ctx, msgID, telegramMsgID)
		_ = b.saveAnswerContext(ctx, sessionID, msgID, contextText)
	}
}

func (b *Bot) ensureSession(ctx context.Context, chatID, userID int64, username string) (uuid.UUID, error) {
	// Сохраняем username в core.users для отображения в памяти чата (Chat Log)
	if username != "" {
		_, _ = b.pool.Exec(ctx, `
			INSERT INTO core.users (telegram_id, username) VALUES ($1, $2)
			ON CONFLICT (telegram_id) DO UPDATE SET username = EXCLUDED.username
		`, userID, username)
	}
	var id uuid.UUID
	err := b.pool.QueryRow(ctx, `
		INSERT INTO chat.sessions (telegram_id, chat_id, last_active)
		VALUES ($1, $2, NOW())
		ON CONFLICT (telegram_id, chat_id) DO UPDATE SET last_active = NOW()
		RETURNING id
	`, userID, chatID).Scan(&id)
	return id, err
}

func (b *Bot) appendMessage(ctx context.Context, sessionID uuid.UUID, role, content string) (uuid.UUID, error) {
	var id uuid.UUID
	err := b.pool.QueryRow(ctx, `INSERT INTO chat.messages (session_id, role, content) VALUES ($1, $2, $3) RETURNING id`, sessionID, role, content).Scan(&id)
	return id, err
}

func (b *Bot) updateMessageTelegramID(ctx context.Context, messageID uuid.UUID, telegramMessageID int) error {
	_, err := b.pool.Exec(ctx, `UPDATE chat.messages SET telegram_message_id = $1 WHERE id = $2`, telegramMessageID, messageID)
	return err
}

func (b *Bot) saveAnswerContext(ctx context.Context, sessionID, messageID uuid.UUID, contextText string) error {
	_, err := b.pool.Exec(ctx, `INSERT INTO chat.answer_context (session_id, message_id, context_text) VALUES ($1, $2, $3)`, sessionID, messageID, contextText)
	return err
}

func (b *Bot) getContextByTelegramMessageID(ctx context.Context, sessionID uuid.UUID, telegramMessageID int) (string, bool) {
	var contextText string
	err := b.pool.QueryRow(ctx, `
		SELECT ac.context_text FROM chat.answer_context ac
		JOIN chat.messages m ON m.id = ac.message_id
		WHERE m.session_id = $1 AND m.telegram_message_id = $2
		LIMIT 1
	`, sessionID, telegramMessageID).Scan(&contextText)
	return contextText, err == nil
}

const maxMessagesBeforeTrim = 30
const keepMessagesAfterTrim = 20

func (b *Bot) trimSessionMessagesIfNeeded(ctx context.Context, sessionID uuid.UUID) {
	var count int64
	if err := b.pool.QueryRow(ctx, `SELECT COUNT(*) FROM chat.messages WHERE session_id = $1`, sessionID).Scan(&count); err != nil || count < maxMessagesBeforeTrim {
		return
	}
	res, err := b.pool.Exec(ctx, `
		DELETE FROM chat.messages WHERE session_id = $1 AND id NOT IN (
			SELECT id FROM chat.messages WHERE session_id = $1 ORDER BY created_at DESC LIMIT $2
		)
	`, sessionID, keepMessagesAfterTrim)
	if err != nil {
		log.Warn(ctx, "trim session messages", logging.KV{"error", err}, logging.KV{"session_id", sessionID})
		return
	}
	if res.RowsAffected() > 0 {
		log.Info(ctx, "trimmed session messages", logging.KV{"session_id", sessionID}, logging.KV{"deleted", res.RowsAffected()})
	}
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

// stripThink убирает блоки think из ответа модели (для поискового запроса), чтобы в Qdrant уходил только сам запрос.
var reThinkBlock = regexp.MustCompile(`(?is)<think[^>]*>.*?` + "</think>")

// Варианты написания месяцев (с опечатками) для извлечения даты из вопроса при NULL от модели.
var monthVariants = []string{
	"января", "янвря", "янаря", "январь",
	"февраля", "феврля", "феварля", "февраль",
	"марта", "матра", "мрта", "март",
	"апреля", "апереля", "апрелья", "апрель",
	"мая", "май",
	"июня", "июна", "июнь",
	"июля", "июль", "июля",
	"августа", "авгста", "август",
	"сентября", "сентябрь", "сентебря", "сентябра",
	"октября", "октбря", "октябрья", "октябрь",
	"ноября", "ноебря", "оября", "ноядбоя", "ноябрь", "ноябра",
	"декабря", "декабрля", "декбаря", "декабрь",
}
var reDateMonthRu = regexp.MustCompile(`(\d{1,2})\s+(` + strings.Join(monthVariants, "|") + `)`)
var reDateDot = regexp.MustCompile(`\d{1,2}\.\d{1,2}(?:\.\d{2,4})?`)

// extractDateFromQuestion извлекает из текста дату (для поиска в Qdrant при NULL от модели). Учитывает опечатки в названиях месяцев.
func extractDateFromQuestion(question string) string {
	q := strings.TrimSpace(question)
	if q == "" {
		return ""
	}
	if m := reDateMonthRu.FindString(q); m != "" {
		return strings.TrimSpace(m)
	}
	if m := reDateDot.FindString(q); m != "" {
		return m
	}
	return ""
}

func stripThink(s string) string {
	return strings.TrimSpace(reThinkBlock.ReplaceAllString(s, ""))
}

// extractSearchQuery вызывает LLM с промптом A: по вопросу пользователя возвращает запрос для поиска в Qdrant.
func (b *Bot) extractSearchQuery(ctx context.Context, requestID, userQuestion string) (string, error) {
	reply, err := b.callLLM(ctx, requestID, b.promptA, userQuestion)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(reply), nil
}

// callLLM вызывает vLLM: systemContent — системный промпт (для ответа по контексту: промпт B + контекст).
func (b *Bot) callLLM(ctx context.Context, requestID, systemContent, userQuery string) (string, error) {
	// OpenAI-compatible chat completion to vLLM
	messages := []map[string]string{
		{"role": "system", "content": systemContent},
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
	b.sendReplyWithID(ctx, chatID, text)
}

// sendReplyWithID отправляет сообщение и возвращает MessageID в Telegram (0 при ошибке).
func (b *Bot) sendReplyWithID(ctx context.Context, chatID int64, text string) int {
	if b.bot == nil {
		return 0
	}
	msg := tgbotapi.NewMessage(chatID, text)
	sent, err := b.bot.Send(msg)
	if err != nil {
		log.Warn(ctx, "send reply", logging.KV{"error", err}, logging.KV{"chat_id", chatID})
		return 0
	}
	return sent.MessageID
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
	_, _ = b.appendMessage(ctx, sessionID, "user", userMsg)
	b.trimSessionMessagesIfNeeded(ctx, sessionID)

	var contextText string
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.IsBot {
		if ctxStored, ok := b.getContextByTelegramMessageID(ctx, sessionID, msg.ReplyToMessage.MessageID); ok && ctxStored != "" {
			contextText = ctxStored
			log.Info(ctx, "using reply-to context for attachment", logging.KV{"chat_id", chatID})
		}
	}
	if contextText == "" {
		searchQuery, errExtract := b.extractSearchQuery(ctx, requestID, userMsg)
		if errExtract != nil {
			log.Warn(ctx, "extract search query for attachment failed", logging.KV{"error", errExtract})
			searchQuery = userMsg
		}
		searchQuery = stripThink(searchQuery)
		if searchQuery == "" || strings.EqualFold(strings.TrimSpace(searchQuery), "null") {
			searchQuery = extractDateFromQuestion(userMsg)
			if searchQuery == "" {
				searchQuery = userMsg
			}
		}
		b.sendReply(ctx, chatID, "🔍 На поиск в Qdrant отправлено:\n"+searchQuery)
		attachmentsText := b.getAttachmentsText(ctx, sessionID)
		var err error
		contextText, err = b.buildContext(ctx, requestID, searchQuery, attachmentsText, 4000, "default")
		if err != nil {
			if err.Error() == "chunk_not_found" {
				b.sendReply(ctx, chatID, "По подходящим данным в базе ничего не найдено.")
				_, _ = b.appendMessage(ctx, sessionID, "assistant", "По подходящим данным в базе ничего не найдено.")
				return
			}
			if err.Error() == "date_not_found" {
				b.sendReply(ctx, chatID, "Данные не найдены.")
				_, _ = b.appendMessage(ctx, sessionID, "assistant", "Данные не найдены.")
				return
			}
			log.Warn(ctx, "build_context for attachment failed", logging.KV{"error", err})
			contextText = ""
		}
	}
	if err := b.llmLimiter.Acquire(ctx); err != nil {
		log.Warn(ctx, "llm queue full for attachment", logging.KV{"error", err})
		b.sendReply(ctx, chatID, "Сервер занят, попробуйте позже.")
		return
	}
	defer b.llmLimiter.Release()
	systemContent := b.promptB + "\n" + contextText
	reply, err := b.callLLM(ctx, requestID, systemContent, userMsg)
	if err != nil {
		log.Warn(ctx, "llm call for attachment", logging.KV{"error", err})
		b.sendReply(ctx, chatID, "Не удалось получить ответ модели. Извлечённый текст сохранён в контексте чата.")
		return
	}
	msgID, _ := b.appendMessage(ctx, sessionID, "assistant", reply)
	b.trimSessionMessagesIfNeeded(ctx, sessionID)
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
	telegramMsgID := b.sendReplyWithID(ctx, chatID, replyToUser)
	if msgID != uuid.Nil && telegramMsgID > 0 {
		_ = b.updateMessageTelegramID(ctx, msgID, telegramMsgID)
		_ = b.saveAnswerContext(ctx, sessionID, msgID, contextText)
	}
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
	// Первый запрос может долго ждать: загрузка моделей с HF (ASR) или медленный инференс на CPU (OCR). 10 мин.
	client := &http.Client{Timeout: 600 * time.Second}
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
