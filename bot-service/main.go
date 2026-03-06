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
	"strconv"
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
	"github.com/jdkato/prose/v2"
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
	botDebug := config.LoadInt("BOT_DEBUG", 0)

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
		debugMode:      botDebug,
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
	debugMode      int    // 0 = вырезать think в финальном ответе, не показывать «на поиск в Qdrant»; 1 = оставить think в ответе и показывать отладочное сообщение
}

const welcomeMessage = "Привет! Я ИИ-ассистент, отвечаю на вопросы по книге «Книга ангелов»."
const chatAlreadyStartedMessage = "Чат уже запущен."
const chatResetMessage = "Чат сброшен. Отправьте /start для начала."

func (b *Bot) handleStart(ctx context.Context, chatID, userID int64, username string) {
	var count int64
	err := b.pool.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM chat.messages m WHERE m.session_id = s.id)
		FROM chat.sessions s WHERE s.telegram_id = $1 AND s.chat_id = $2
	`, userID, chatID).Scan(&count)
	if err == nil && count > 0 {
		b.sendReply(ctx, chatID, chatAlreadyStartedMessage)
		return
	}
	_, _ = b.ensureSession(ctx, chatID, userID, username)
	b.sendReply(ctx, chatID, welcomeMessage)
}

func (b *Bot) handleRestart(ctx context.Context, chatID, userID int64) {
	_, err := b.pool.Exec(ctx, `
		WITH sid AS (
			SELECT id FROM chat.sessions WHERE telegram_id = $1 AND chat_id = $2
		)
		DELETE FROM chat.answer_context WHERE session_id IN (SELECT id FROM sid);
		DELETE FROM chat.messages WHERE session_id IN (SELECT id FROM sid);
		DELETE FROM chat.attachments WHERE session_id IN (SELECT id FROM sid);
		DELETE FROM chat.sessions WHERE telegram_id = $1 AND chat_id = $2
	`, userID, chatID)
	if err != nil {
		log.Warn(ctx, "handleRestart delete failed", logging.KV{"error", err})
	}
	b.sendReply(ctx, chatID, chatResetMessage)
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
	replyToTgID := 0
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.IsBot {
		replyToTgID = msg.ReplyToMessage.MessageID
	}
	_, _ = b.appendMessageWithReply(ctx, sessionID, "user", msg.Text, replyToTgID)
	b.trimSessionMessagesIfNeeded(ctx, sessionID)

	// Сообщение об ожидании — как можно раньше после получения сообщения пользователя
	typingMsgID := b.sendReplyTyping(ctx, chatID)

	var contextText string
	// Ответ на наше сообщение: берём сохранённый контекст, сразу промпт B (без запроса в Qdrant)
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.IsBot {
		if ctxStored, ok := b.getContextByTelegramMessageID(ctx, sessionID, msg.ReplyToMessage.MessageID); ok && ctxStored != "" {
			contextText = ctxStored
			log.Info(ctx, "using reply-to context", logging.KV{"chat_id", chatID})
		}
	}
	if contextText == "" {
		var searchQuery string
		// Поисковый запрос без LLM: извлекаем сущности (даты, части тела, национальности, знаки зодиака, фразы)
		searchQuery = extractSearchEntitiesFromQuestion(msg.Text)
		if searchQuery == "" {
			searchQuery = extractDateFromQuestion(msg.Text)
			if searchQuery == "" {
				searchQuery = msg.Text
			}
		}
		// Спецзапросы по ключевым словам в исходном сообщении (с упоминанием ангелов)
		lowerMsg := strings.ToLower(strings.TrimSpace(msg.Text))
		if hasAngelWord(msg.Text) {
			// [name] all — только при явном запросе «все ангелы» / «всех ангелов», а не при «какой ангел может растворить все долги»
			if strings.Contains(lowerMsg, "все ангел") || strings.Contains(lowerMsg, "всех ангел") ||
				strings.Contains(lowerMsg, "ангел вс") || strings.Contains(lowerMsg, "ангелов вс") ||
				strings.Contains(lowerMsg, "перечисли всех") || strings.Contains(lowerMsg, "список всех") {
				searchQuery = "[name] all"
			} else if strings.Contains(lowerMsg, "дат") {
				searchQuery = "[date] list"
			}
		}
		userDateStr := extractDateFromQuestion(msg.Text)
		if userDateStr != "" {
			_, _, modelHasDate := parseDayMonthFromQuery(searchQuery)
			trimmed := strings.TrimSpace(searchQuery)
			if !modelHasDate || isOnlyMonth(trimmed) || isOnlyDay(trimmed) {
				searchQuery = userDateStr
			}
		}
		searchQuery = translateMonthToRussian(searchQuery)
		if userDateStr != "" {
			userDay, userMon, userOk := parseDayMonthFromQuery(userDateStr)
			modelDay, modelMon, modelOk := parseDayMonthFromQuery(searchQuery)
			if userOk && modelOk && (userDay != modelDay || userMon != modelMon) {
				searchQuery = userDateStr
			}
		}
		day, month, hasDate := parseDayMonthFromQuery(searchQuery)
		if hasDate && (day < 1 || day > maxDaysInMonth(month)) {
			contextText = "date not found"
		} else {
			normalizedQuery := strings.TrimSpace(strings.ToLower(searchQuery))
			switch normalizedQuery {
		case "[name] all":
			if b.debugMode == 1 {
				b.sendReply(ctx, chatID, "🔍 На поиск в Qdrant отправлено:\n"+searchQuery)
			}
			var errNames error
			contextText, errNames = b.getAllNames(ctx, requestID)
			if errNames != nil {
				log.Warn(ctx, "getAllNames failed", logging.KV{"error", errNames})
				contextText = ""
			}
		case "[date] list":
			// Не ищем в Qdrant — сразу в LLM с промптом B и пустым контекстом
			contextText = ""
		default:
			attachmentsText := b.getAttachmentsText(ctx, sessionID)
			var err error
			var buildChunkIDs []string
			var buildCollection string
			var buildCollectionsSearched []string
			contextText, buildChunkIDs, buildCollection, buildCollectionsSearched, err = b.buildContext(ctx, requestID, searchQuery, attachmentsText, 4000, "default")
			if err != nil {
				if err.Error() == "chunk_not_found" {
					contextText = "По подходящим данным в базе ничего не найдено."
				} else if err.Error() == "date_not_found" {
					contextText = "Данные не найдены."
				} else {
					log.Warn(ctx, "build_context failed, using empty context", logging.KV{"error", err})
					contextText = ""
				}
			}
			if b.debugMode == 1 {
				msg := "🔍 На поиск в Qdrant отправлено:\n" + searchQuery
				if len(buildCollectionsSearched) > 0 {
					msg += "\n\nИскало в: " + strings.Join(buildCollectionsSearched, ", ")
				}
				if buildCollection != "" {
					msg += "\n\nНайдено в: " + buildCollection
				}
				if len(buildChunkIDs) > 0 {
					msg += "\n\nChunk ID: " + strings.Join(buildChunkIDs, ", ")
				}
				b.sendReply(ctx, chatID, msg)
			}
		}
		}
	}

	// Ответ по контексту с промптом B (typingMsgID уже отправлен выше)
	systemContent := b.promptB + "\n" + contextText
	reply, err := b.callLLM(ctx, requestID, systemContent, msg.Text)
	// В финальном ответе вырезать think только когда не дебаг; в дебаге оставить для просмотра
	if b.debugMode == 0 {
		reply = stripThink(reply)
	}
	if err != nil {
		log.Error(ctx, "llm call", logging.KV{"error", err}, logging.KV{"vllm_base", b.vllmBase})
		hint := "Модель недоступна. Проверьте, что vLLM запущен (docker compose --profile vllm up -d) и в .env указан VLLM_OPENAI_BASE."
		if errStr := err.Error(); len(errStr) < 120 {
			hint = "Ошибка LLM: " + errStr
		}
		if typingMsgID > 0 {
			b.editMessageText(ctx, chatID, typingMsgID, hint)
		} else {
			b.sendReply(ctx, chatID, hint)
		}
		return
	}
	msgID, _ := b.appendMessage(ctx, sessionID, "assistant", reply)
	b.trimSessionMessagesIfNeeded(ctx, sessionID)
	var telegramMsgID int
	if typingMsgID > 0 {
		if b.editMessageText(ctx, chatID, typingMsgID, reply) {
			telegramMsgID = typingMsgID
		} else {
			telegramMsgID = b.sendReplyWithID(ctx, chatID, reply)
		}
	} else {
		telegramMsgID = b.sendReplyWithID(ctx, chatID, reply)
	}
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
	return b.appendMessageWithReply(ctx, sessionID, role, content, 0)
}

// appendMessageWithReply сохраняет сообщение; для role=user replyToTelegramID — ID сообщения в Telegram, на которое отвечаем (0 = не ответ).
func (b *Bot) appendMessageWithReply(ctx context.Context, sessionID uuid.UUID, role, content string, replyToTelegramID int) (uuid.UUID, error) {
	var id uuid.UUID
	if replyToTelegramID != 0 && role == "user" {
		err := b.pool.QueryRow(ctx, `INSERT INTO chat.messages (session_id, role, content, reply_to_telegram_message_id) VALUES ($1, $2, $3, $4) RETURNING id`,
			sessionID, role, content, replyToTelegramID).Scan(&id)
		return id, err
	}
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

func (b *Bot) buildContext(ctx context.Context, requestID, query, attachmentsText string, tokenBudget int, mode string) (context string, chunkIDs []string, searchCollection string, collectionsSearched []string, err error) {
	body := map[string]interface{}{
		"query_text":       query,
		"acl_token":        "placeholder",
		"token_budget":     tokenBudget,
		"mode":             mode,
		"attachments_text": attachmentsText,
	}
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, b.mcpReadURL+"/mcp/build_context", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", requestID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bb, _ := io.ReadAll(resp.Body)
		return "", nil, "", fmt.Errorf("mcp-read %d: %s", resp.StatusCode, string(bb))
	}
	var out struct {
		Context            string   `json:"context"`
		ChunkIDs           []string `json:"chunk_ids"`
		SearchCollection   string   `json:"search_collection"`
		CollectionsSearched []string `json:"collections_searched"`
		Error              string   `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", nil, "", nil, err
	}
	if out.Error != "" {
		return "", out.ChunkIDs, out.SearchCollection, out.CollectionsSearched, fmt.Errorf("%s", out.Error)
	}
	return out.Context, out.ChunkIDs, out.SearchCollection, out.CollectionsSearched, nil
}

// getAllNames возвращает контекст из всех уникальных name в Qdrant (для запроса "[name] all").
func (b *Bot) getAllNames(ctx context.Context, requestID string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, b.mcpReadURL+"/mcp/all_names", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", requestID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bb, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("mcp-read all_names %d: %s", resp.StatusCode, string(bb))
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

// Синонимы «ангел» и т.п. для определения [name] all / [date] list (не для вырезания).
var angelSynonymsForDetection = []string{
	"ангелы-хранители", "ангелов-хранителей", "ангел-хранитель", "ангела-хранителя",
	"ангелы", "ангелов", "ангелам", "ангелами", "ангелах",
	"ангел", "ангела", "ангелу", "ангелом", "ангеле",
	"хранители", "хранителей", "хранитель", "хранителя", "хранителю", "хранителем", "хранителе",
}

// hasAngelWord возвращает true, если в тексте есть одно из слов про ангелов (для определения [name] all / [date] list).
func hasAngelWord(s string) bool {
	lower := strings.ToLower(s)
	for _, phrase := range angelSynonymsForDetection {
		if phrase == "" {
			continue
		}
		re := regexp.MustCompile(`(?i)(^|[^\p{L}])` + regexp.QuoteMeta(phrase) + `([^\p{L}]|$)`)
		if re.MatchString(lower) {
			return true
		}
	}
	return false
}

// Извлечение сущностей только через Prose: Entities() + фразы после триггеров по токенам. Без фиксированных словарей — X может быть любым (брак, Турция, ноги, бессонница и т.д.).
var monthVariantsForTokens = []string{
	"января", "янвря", "янаря", "январь", "февраля", "феврля", "феварля", "февраль",
	"марта", "матра", "мрта", "март", "апреля", "апереля", "апрелья", "апрель",
	"мая", "май", "июня", "июна", "июнь", "июля", "июль", "августа", "авгста", "август",
	"сентября", "сентябрь", "сентебря", "сентябра", "октября", "октбря", "октябрья", "октябрь",
	"ноября", "ноебря", "оября", "ноядбоя", "ноябрь", "ноябра", "декабря", "декабрля", "декбаря", "декабрь",
}

var monthNamesForDate = makeSet(monthVariantsForTokens)

func makeSet(words []string) map[string]struct{} {
	m := make(map[string]struct{}, len(words))
	for _, w := range words {
		m[strings.ToLower(w)] = struct{}{}
	}
	return m
}

// Триггеры для извлечения фразы после них по токенам Prose: ключ — слово (нижний регистр), значение — сколько токенов пропустить перед фразой.
var phraseTriggerSkip = map[string]int{
	"исцелить": 0, "исцеляют": 0, "исцеляющих": 0, "исцеляющего": 0,
	"способных": 0, "способны": 0, "могут": 0,
	"влияет": 1, "влияние": 1, "влияют": 1,
	"властвует": 1, "властвуют": 1, "влавствует": 1, // опечатка «влавствует»
	"помогает": 1, "помогают": 1, "помощь": 1,
	"при": 1, // по умолчанию «при X»; для «при проблемах с» — динамически skip 3
	"от": 0, "для": 0,
	"на": 0, "над": 0, // «влияет на X», «властвует над X» — X из предлога при любом глаголе
	"с": 0, "в": 0, "о": 0, "об": 0, "по": 0, "к": 0, "у": 0,
	"из": 0, "за": 0, "про": 0, "под": 0, "до": 0, "после": 0, "между": 0, "перед": 0, "через": 0,
	// с/в/о/по/к/у/из/за/про/под/до/после/между/перед/через X — ангел из сферы, за мир, расскажи про X, под знаком, до рассвета и т.д.
}

// extractSearchEntitiesFromQuestion извлекает сущности только через Prose: Entities() + фразы после триггеров по Tokens(); без фиксированных словарей — подходит для любых тем.
func extractSearchEntitiesFromQuestion(question string) string {
	s := strings.TrimSpace(question)
	if s == "" {
		return ""
	}
	doc, err := prose.NewDocument(s)
	if err != nil {
		return ""
	}
	seen := make(map[string]struct{})
	var parts []string
	add := func(t string) {
		t = strings.TrimSpace(t)
		if t == "" {
			return
		}
		t = toNounNominativePhrase(t)
		t = strings.TrimSpace(t)
		if t == "" {
			return
		}
		tl := strings.ToLower(t)
		if _, ok := seen[tl]; ok {
			return
		}
		seen[tl] = struct{}{}
		parts = append(parts, t)
	}

	tokens := doc.Tokens()
	for _, ent := range doc.Entities() {
		add(ent.Text)
	}

	// Дата по токенам: число 1–31 + следующий токен — месяц
	for i := 0; i < len(tokens)-1; i++ {
		day, errDay := strconv.Atoi(tokens[i].Text)
		if errDay != nil || day < 1 || day > 31 {
			continue
		}
		monLower := strings.ToLower(tokens[i+1].Text)
		if _, ok := monthNamesForDate[monLower]; ok {
			add(tokens[i].Text + " " + tokens[i+1].Text)
			break
		}
	}

	// Фразы после триггеров: X из «исцелить X», «влияет на X», «властвует над X», «помогает при X» / «помогает браку», «от X», «для X» — без фиксированных списков, X любой.
	for i := 0; i < len(tokens); i++ {
		tl := strings.ToLower(tokens[i].Text)
		skip, isTrigger := phraseTriggerSkip[tl]
		if !isTrigger {
			continue
		}
		// «помогает браку»: после «помогает» нет «при» — берём следующий токен; «помогает при бессоннице» — пропускаем «при».
		if (tl == "помогает" || tl == "помогают" || tl == "помощь") && i+1 < len(tokens) && strings.ToLower(tokens[i+1].Text) != "при" {
			skip = 0
		}
		// «при проблемах с бессонницей» — пропустить 3 токена; иначе «при бессоннице» — пропустить 1.
		if tl == "при" && i+2 < len(tokens) {
			next1 := strings.ToLower(tokens[i+1].Text)
			next2 := strings.ToLower(tokens[i+2].Text)
			if next1 == "проблемах" && next2 == "с" {
				skip = 3
			}
		}
		start := i + 1 + skip
		if start >= len(tokens) {
			continue
		}
		var phrase []string
		for j := start; j < len(tokens); j++ {
			t := tokens[j].Text
			if len(t) == 1 && (t == "." || t == "," || t == "!" || t == "?" || t == ";") {
				break
			}
			phrase = append(phrase, t)
		}
		if len(phrase) > 0 {
			phraseStr := strings.TrimRight(strings.Join(phrase, " "), ".,?!;")
			add(phraseStr)
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(collapseSpaces(strings.Join(parts, " ")))
}

func collapseSpaces(s string) string {
	var b strings.Builder
	prevSpace := true
	for _, r := range strings.TrimSpace(s) {
		if r == ' ' || r == '\t' {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return b.String()
}

// Правила приведения русского слова к форме существительного (именительный падеж): браком->брак, ногами->ноги, турцией->турция.
// Порядок: от длинных суффиксов к коротким.
var nounNominativeSuffixes = []struct{ suffix, repl string }{
	{"ами", "и"},   // ногами->ноги, руками->руки
	{"ями", "и"},   // евреями->евреи
	{"нице", "ница"}, // бессоннице->бессонница
	{"ах", "и"},    // ногах->ноги
	{"ом", ""},     // браком->брак, домом->дом
	{"ой", "а"},    // ногой->нога, рукой->рука
	{"ей", "я"},    // турцией->турция
	{"ке", "ка"},   // руке->рука
	{"ям", "и"},    // евреям->евреи
	{"ев", "и"},    // евреев->евреи
	{"ов", ""},     // столов->стол, ангелов->ангел
	{"у", ""},      // браку->брак, слону->слон
	{"ю", ""},      // слону->слон (вариант)
}

func toNounNominative(word string) string {
	w := strings.TrimSpace(word)
	if w == "" {
		return w
	}
	lower := strings.ToLower(w)
	for _, r := range nounNominativeSuffixes {
		if len(lower) <= len(r.suffix) {
			continue
		}
		if strings.HasSuffix(lower, r.suffix) {
			base := w[:len(w)-len(r.suffix)]
			return base + r.repl
		}
	}
	return w
}

// toNounNominativePhrase приводит каждое слово во фразе к форме существительного (для поиска: браком->брак, ногами->ноги).
func toNounNominativePhrase(phrase string) string {
	parts := strings.Fields(phrase)
	for i, p := range parts {
		parts[i] = toNounNominative(p)
	}
	return strings.Join(parts, " ")
}

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
var reDateDot = regexp.MustCompile(`(\d{1,2})\.(\d{1,2})(?:\.\d{2,4})?`)

// monthNameToNum — маппинг вариантов названия месяца (р.п.) на номер 1–12.
var monthNameToNum = func() map[string]int {
	groups := [][]string{
		{"января", "янвря", "янаря", "январь"},
		{"февраля", "феврля", "феварля", "февраль"},
		{"марта", "матра", "мрта", "март"},
		{"апреля", "апереля", "апрелья", "апрель"},
		{"мая", "май"},
		{"июня", "июна", "июнь"},
		{"июля", "июль"},
		{"августа", "авгста", "август"},
		{"сентября", "сентябрь", "сентебря", "сентябра"},
		{"октября", "октбря", "октябрья", "октябрь"},
		{"ноября", "ноебря", "оября", "ноядбоя", "ноябрь", "ноябра"},
		{"декабря", "декабрля", "декбаря", "декабрь"},
	}
	m := make(map[string]int)
	for i, variants := range groups {
		for _, v := range variants {
			m[strings.ToLower(v)] = i + 1
		}
	}
	return m
}()

// maxDaysInMonth — макс. число дней в месяце (февраль высокосный = 29).
func maxDaysInMonth(month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		return 29
	default:
		return 0
	}
}

// parseDayMonthFromQuery парсит из строки запроса день и месяц. Возвращает (day, month 1–12, ok).
// День может быть 0 (для проверки day < 1) или до 31; валидация по maxDaysInMonth делается снаружи.
func parseDayMonthFromQuery(query string) (day, month int, ok bool) {
	q := strings.TrimSpace(query)
	if q == "" {
		return 0, 0, false
	}
	if m := reDateMonthRu.FindStringSubmatch(q); len(m) >= 3 {
		d, err := strconv.Atoi(m[1])
		if err != nil || d < 0 || d > 31 {
			return 0, 0, false
		}
		monName := strings.ToLower(strings.TrimSpace(m[2]))
		if mon, has := monthNameToNum[monName]; has {
			return d, mon, true
		}
		return d, 0, false
	}
	if m := reDateDot.FindStringSubmatch(q); len(m) >= 3 {
		d, err1 := strconv.Atoi(m[1])
		mon, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil || mon < 1 || mon > 12 || d < 0 || d > 31 {
			return 0, 0, false
		}
		return d, mon, true
	}
	return 0, 0, false
}

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

// isOnlyMonth возвращает true, если строка — только название месяца (рус. или англ.), без числа.
func isOnlyMonth(s string) bool {
	t := strings.TrimSpace(strings.ToLower(s))
	if t == "" {
		return false
	}
	if _, has := monthNameToNum[t]; has {
		return true
	}
	_, has := enMonthToRuLower[t]
	return has
}

// isOnlyDay возвращает true, если строка — только число 1–31 (день месяца).
func isOnlyDay(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	n, err := strconv.Atoi(t)
	return err == nil && n >= 1 && n <= 31
}

// Английские названия месяцев (р.п. или им.п.) → русские в р.п. (для замены в ответе модели).
var enMonthToRu = map[string]string{
	"January": "января", "February": "февраля", "March": "марта", "April": "апреля",
	"May": "мая", "June": "июня", "July": "июля", "August": "августа",
	"September": "сентября", "October": "октября", "November": "ноября", "December": "декабря",
	"Jan": "января", "Feb": "февраля", "Mar": "марта", "Apr": "апреля",
	"Jun": "июня", "Jul": "июля", "Aug": "августа", "Sep": "сентября",
	"Oct": "октября", "Nov": "ноября", "Dec": "декабря",
}
var enMonthToRuLower = func() map[string]string {
	m := make(map[string]string)
	for k, v := range enMonthToRu {
		m[strings.ToLower(k)] = v
	}
	return m
}()

// translateMonthToRussian заменяет в строке английские названия месяцев на русские (р.п.).
func translateMonthToRussian(s string) string {
	out := s
	for en, ru := range enMonthToRu {
		out = strings.ReplaceAll(out, en, ru)
		out = strings.ReplaceAll(out, strings.ToLower(en), ru)
	}
	return out
}

func stripThink(s string) string {
	return strings.TrimSpace(reThinkBlock.ReplaceAllString(s, ""))
}

func isStartCommand(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	return t == "/start" || strings.HasPrefix(t, "/start ")
}

func isRestartCommand(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	return t == "/restart" || t == "/reset" || strings.HasPrefix(t, "/restart ") || strings.HasPrefix(t, "/reset ")
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

// sendReplyWithID отправляет сообщение и возвращает MessageID в Telegram (0 при ошибке). Применяется Markdown.
func (b *Bot) sendReplyWithID(ctx context.Context, chatID int64, text string) int {
	if b.bot == nil {
		return 0
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	sent, err := b.bot.Send(msg)
	if err != nil {
		msg.ParseMode = ""
		sent, err = b.bot.Send(msg)
	}
	if err != nil {
		log.Warn(ctx, "send reply", logging.KV{"error", err}, logging.KV{"chat_id", chatID})
		return 0
	}
	return sent.MessageID
}

// sendReplyTyping отправляет «...» и возвращает MessageID (0 при ошибке). Потом его редактируют в итоговый ответ.
func (b *Bot) sendReplyTyping(ctx context.Context, chatID int64) int {
	return b.sendReplyWithID(ctx, chatID, "...")
}

// editMessageText редактирует ранее отправленное сообщение. При ошибке возвращает false.
func (b *Bot) editMessageText(ctx context.Context, chatID int64, messageID int, text string) bool {
	if b.bot == nil || messageID <= 0 {
		return false
	}
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "Markdown"
	_, err := b.bot.Send(edit)
	if err != nil {
		edit.ParseMode = ""
		_, err = b.bot.Send(edit)
	}
	if err != nil {
		log.Warn(ctx, "edit message", logging.KV{"error", err}, logging.KV{"chat_id", chatID})
		return false
	}
	return true
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
	// Сообщение об ожидании — как можно раньше после получения вложения
	typingMsgID := b.sendReplyTyping(ctx, chatID)

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
		if typingMsgID > 0 {
			b.editMessageText(ctx, chatID, typingMsgID, "Не удалось получить файл.")
		} else {
			b.sendReply(ctx, chatID, "Не удалось получить файл.")
		}
		return
	}
	downloadURL := "https://api.telegram.org/file/bot" + b.bot.Token + "/" + file.FilePath
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Warn(ctx, "download file from telegram", logging.KV{"error", err})
		if typingMsgID > 0 {
			b.editMessageText(ctx, chatID, typingMsgID, "Не удалось скачать файл.")
		} else {
			b.sendReply(ctx, chatID, "Не удалось скачать файл.")
		}
		return
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Warn(ctx, "read telegram file body", logging.KV{"error", err})
		if typingMsgID > 0 {
			b.editMessageText(ctx, chatID, typingMsgID, "Ошибка чтения файла.")
		} else {
			b.sendReply(ctx, chatID, "Ошибка чтения файла.")
		}
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
		if typingMsgID > 0 {
			b.editMessageText(ctx, chatID, typingMsgID, "Не удалось обработать файл.")
		} else {
			b.sendReply(ctx, chatID, "Не удалось обработать файл.")
		}
		return
	}
	if userErr != "" {
		if typingMsgID > 0 {
			b.editMessageText(ctx, chatID, typingMsgID, userErr)
		} else {
			b.sendReply(ctx, chatID, userErr)
		}
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
		if typingMsgID > 0 {
			b.editMessageText(ctx, chatID, typingMsgID, "Слишком много запросов, подождите.")
		} else {
			b.sendReply(ctx, chatID, "Слишком много запросов, подождите.")
		}
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
		var searchQuery string
		searchQuery = extractSearchEntitiesFromQuestion(userMsg)
		if searchQuery == "" {
			searchQuery = extractDateFromQuestion(userMsg)
			if searchQuery == "" {
				searchQuery = userMsg
			}
		}
		lowerMsg := strings.ToLower(strings.TrimSpace(userMsg))
		if hasAngelWord(userMsg) {
			if strings.Contains(lowerMsg, "все ангел") || strings.Contains(lowerMsg, "всех ангел") ||
				strings.Contains(lowerMsg, "ангел вс") || strings.Contains(lowerMsg, "ангелов вс") ||
				strings.Contains(lowerMsg, "перечисли всех") || strings.Contains(lowerMsg, "список всех") {
				searchQuery = "[name] all"
			} else if strings.Contains(lowerMsg, "дат") {
				searchQuery = "[date] list"
			}
		}
		userDateStr := extractDateFromQuestion(userMsg)
		if userDateStr != "" {
			_, _, modelHasDate := parseDayMonthFromQuery(searchQuery)
			trimmed := strings.TrimSpace(searchQuery)
			if !modelHasDate || isOnlyMonth(trimmed) || isOnlyDay(trimmed) {
				searchQuery = userDateStr
			}
		}
		searchQuery = translateMonthToRussian(searchQuery)
		if userDateStr != "" {
			userDay, userMon, userOk := parseDayMonthFromQuery(userDateStr)
			modelDay, modelMon, modelOk := parseDayMonthFromQuery(searchQuery)
			if userOk && modelOk && (userDay != modelDay || userMon != modelMon) {
				searchQuery = userDateStr
			}
		}
		day, month, hasDate := parseDayMonthFromQuery(searchQuery)
		if hasDate && (day < 1 || day > maxDaysInMonth(month)) {
			contextText = "date not found"
		} else {
			normalizedQuery := strings.TrimSpace(strings.ToLower(searchQuery))
			switch normalizedQuery {
			case "[name] all":
				if b.debugMode == 1 {
					b.sendReply(ctx, chatID, "🔍 На поиск в Qdrant отправлено:\n"+searchQuery)
				}
				var errNames error
				contextText, errNames = b.getAllNames(ctx, requestID)
				if errNames != nil {
					log.Warn(ctx, "getAllNames for attachment failed", logging.KV{"error", errNames})
					contextText = ""
				}
			case "[date] list":
				contextText = ""
			default:
				attachmentsText := b.getAttachmentsText(ctx, sessionID)
				var err error
				var buildChunkIDs []string
				var buildCollection string
				var buildCollectionsSearched []string
				contextText, buildChunkIDs, buildCollection, buildCollectionsSearched, err = b.buildContext(ctx, requestID, searchQuery, attachmentsText, 4000, "default")
				if err != nil {
					if err.Error() == "chunk_not_found" {
						contextText = "По подходящим данным в базе ничего не найдено."
					} else if err.Error() == "date_not_found" {
						contextText = "Данные не найдены."
					} else {
						log.Warn(ctx, "build_context for attachment failed", logging.KV{"error", err})
						contextText = ""
					}
				}
				if b.debugMode == 1 {
					msg := "🔍 На поиск в Qdrant отправлено:\n" + searchQuery
					if len(buildCollectionsSearched) > 0 {
						msg += "\n\nИскало в: " + strings.Join(buildCollectionsSearched, ", ")
					}
					if buildCollection != "" {
						msg += "\n\nНайдено в: " + buildCollection
					}
					if len(buildChunkIDs) > 0 {
						msg += "\n\nChunk ID: " + strings.Join(buildChunkIDs, ", ")
					}
					b.sendReply(ctx, chatID, msg)
				}
			}
		}
	}
	if err := b.llmLimiter.Acquire(ctx); err != nil {
		log.Warn(ctx, "llm queue full for attachment", logging.KV{"error", err})
		b.sendReply(ctx, chatID, "Сервер занят, попробуйте позже.")
		return
	}
	defer b.llmLimiter.Release()
	// typingMsgID уже отправлен в начале handleAttachment
	systemContent := b.promptB + "\n" + contextText
	reply, err := b.callLLM(ctx, requestID, systemContent, userMsg)
	// В финальном ответе вырезать think только когда не дебаг; в дебаге оставить для просмотра
	if b.debugMode == 0 {
		reply = stripThink(reply)
	}
	if err != nil {
		log.Warn(ctx, "llm call for attachment", logging.KV{"error", err})
		hint := "Не удалось получить ответ модели. Извлечённый текст сохранён в контексте чата."
		if typingMsgID > 0 {
			b.editMessageText(ctx, chatID, typingMsgID, hint)
		} else {
			b.sendReply(ctx, chatID, hint)
		}
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
	var telegramMsgID int
	if typingMsgID > 0 {
		if b.editMessageText(ctx, chatID, typingMsgID, replyToUser) {
			telegramMsgID = typingMsgID
		} else {
			telegramMsgID = b.sendReplyWithID(ctx, chatID, replyToUser)
		}
	} else {
		telegramMsgID = b.sendReplyWithID(ctx, chatID, replyToUser)
	}
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
