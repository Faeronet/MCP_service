package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var logHandler = logging.New("mcp-proxy")

// llmFailureUserHint — текст для пользователя при сбое OpenAI-compatible API (vLLM).
func llmFailureUserHint(s *Server, err error) string {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	detail := errStr
	if len(detail) > 300 {
		detail = detail[:300] + "…"
	}
	base := fmt.Sprintf("URL=%s, LLM_MODEL=%s. Поднимите vLLM: docker compose --profile vllm up -d. В .env: VLLM_OPENAI_BASE (или LLM_BINDING_HOST с подчёркиваниями), имя модели как в vLLM.",
		s.VllmBase, s.LlmModel)
	if detail == "" {
		return "Модель недоступна. " + base
	}
	return "Ошибка LLM: " + detail + " | " + base
}

// HandleChat processes POST /chat: appends user message, builds context, calls LLM, saves assistant message, returns reply.
func (s *Server) HandleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	requestID := req.RequestID
	if requestID == "" {
		requestID = uuid.New().String()
	}

	key := fmt.Sprintf("user:%d", req.UserID)
	if !s.PerChatLimiter.Allow(key) {
		logHandler.Warn(ctx, "per-user rate limit", logging.KV{"user_id", req.UserID})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ChatResponse{ReplyText: ""})
		return
	}
	if err := s.LlmLimiter.Acquire(ctx); err != nil {
		logHandler.Warn(ctx, "llm queue full", logging.KV{"error", err})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ChatResponse{ReplyText: "Сервер занят, попробуйте позже."})
		return
	}
	defer s.LlmLimiter.Release()

	var contextText string
	var replyToPrefix string // PREVIOUS_QUESTION / PREVIOUS_ANSWER без дублирования CONTEXT
	var savedContextRef string
	var useStoredReplyContext bool // вопрос по ответу бота: в LLM — тот же CONTEXT, что был у того ответа (full или чанк)
	var debugMessage string
	var reminderExtraText string
	var reply string
	var replyErr error
	var nameAllHandled bool

	_, _ = s.AppendMessageWithReply(ctx, req.SessionID, "user", req.MessageText, req.ReplyToTelegramMessageID)
	s.TrimSessionMessagesIfNeeded(ctx, req.SessionID)

	lowerMsg := strings.ToLower(strings.TrimSpace(req.MessageText))
	if IsListAllAngelsRequest(req.MessageText) {
		names, errList := s.GetAngelNamesList(ctx)
		if errList != nil {
			reply = "Ошибка при получении списка."
			nameAllHandled = true
		} else if len(names) == 0 {
			reply = "В базе пока нет имён ангелов-хранителей."
			nameAllHandled = true
		} else {
			var bld strings.Builder
			bld.WriteString("Всего ")
			bld.WriteString(strconv.Itoa(len(names)))
			bld.WriteString(":\n")
			for i, n := range names {
				if i > 0 {
					bld.WriteString("\n")
				}
				bld.WriteString(strconv.Itoa(i + 1))
				bld.WriteString(". ")
				bld.WriteString(n)
			}
			reply = bld.String()
			nameAllHandled = true
		}
	} else if IsAngelCountRequest(req.MessageText) {
		names, errList := s.GetAngelNamesList(ctx)
		if errList != nil {
			reply = "0"
			nameAllHandled = true
		} else {
			reply = strconv.Itoa(len(names))
			nameAllHandled = true
		}
	}

	// Ответ на сообщение бота: факты для уточнения — только из сохранённого CONTEXT того ответа.
	if !nameAllHandled && req.ReplyToTelegramMessageID != 0 {
		_, _, ctxForLLM, ctxRef, ok := s.GetReplyToContext(ctx, req.SessionID, req.ReplyToTelegramMessageID)
		if ok && ctxForLLM != "" {
			useStoredReplyContext = true
			contextText = ctxForLLM
			savedContextRef = ctxRef
			// Для уточняющего вопроса не добавляем в system предыдущий вопрос/ответ — только CONTEXT.
			logHandler.Info(ctx, "reply-to: using stored answer CONTEXT for LLM", logging.KV{"chat_id", req.ChatID}, logging.KV{"has_ref", ctxRef != ""})
		}
	}

	if !nameAllHandled && !useStoredReplyContext {
		userDateStr := ExtractDateFromQuestion(req.MessageText)
		var searchQuery string
		// Промпт A = второй round-trip к LLM; по умолчанию пропускаем, если в вопросе уже есть дата (LLM_QUERY_EXTRACT=no_date).
		if s.ShouldRunQueryExtractLLM(req.MessageText) {
			if q, err := s.ExtractSearchQuery(ctx, requestID, req.MessageText); err == nil && strings.TrimSpace(q) != "" {
				searchQuery = q
			} else {
				if err != nil {
					logHandler.Warn(ctx, "extract_search_query (prompt A) failed, using fallback text", logging.KV{"error", err})
				}
				searchQuery = userDateStr
				if searchQuery == "" {
					searchQuery = req.MessageText
				}
			}
		} else {
			if userDateStr != "" {
				searchQuery = userDateStr
			} else {
				searchQuery = req.MessageText
			}
		}

		if HasAngelWord(req.MessageText) && strings.Contains(lowerMsg, "дат") {
			searchQuery = "[date] list"
		}
		if userDateStr != "" {
			_, _, modelHasDate := ParseDayMonthFromQuery(searchQuery)
			trimmed := strings.TrimSpace(searchQuery)
			if !modelHasDate || IsOnlyMonth(trimmed) || IsOnlyDay(trimmed) {
				searchQuery = userDateStr
			}
		}
		searchQuery = TranslateMonthToRussian(searchQuery)
		if userDateStr != "" {
			userDay, userMon, userOk := ParseDayMonthFromQuery(userDateStr)
			modelDay, modelMon, modelOk := ParseDayMonthFromQuery(searchQuery)
			if userOk && modelOk && (userDay != modelDay || userMon != modelMon) {
				searchQuery = userDateStr
			}
		}

		if hh, mm, ok := ParseReminderLine(searchQuery); ok {
			if err := s.UpsertReminderSubscriber(ctx, req.UserID, req.ChatID, hh, mm); err != nil {
				logHandler.Warn(ctx, "reminder_subscribe", logging.KV{"error", err})
			} else {
				reminderExtraText = s.BuildTodayAngelReminderText(ctx, requestID)
			}
			searchQuery = s.FallbackSearchQueryAfterReminder(req.MessageText)
		}

		day, month, hasDate := ParseDayMonthFromQuery(searchQuery)
		if hasDate && (day < 1 || day > MaxDaysInMonth(month)) {
			contextText = "date not found"
		} else if IsMetaQuestionAboutBot(req.MessageText) {
			// Мета-вопрос смотрим по исходному сообщению; searchQuery от промпта A даёт ложные срабатывания («что ты знаешь…»).
			contextText = ""
		} else {
			attachmentsText := s.GetAttachmentsText(ctx, req.SessionID)
			var err error
			var buildChunkIDs []string
			var buildCollection string
			var buildCollectionsSearched []string
			var buildQueryForFilter string
			var buildContextKind string
			var buildContextRef string
			// tokenBudget управляет тем, сколько материала вернётся в CONTEXT (и как быстро пройдёт построение/усечение).
			// Для ускорения берём небольшой, но достаточный бюджет под вход LLM.
			tokenBudget := s.LlmContextLength - s.LlmMaxTokens
			if tokenBudget > 1200 {
				tokenBudget = 1200
			}
			if tokenBudget < 512 {
				tokenBudget = 512
			}
			contextText, buildChunkIDs, buildCollection, buildCollectionsSearched, buildQueryForFilter, buildContextKind, buildContextRef, err = s.BuildContext(ctx, requestID, searchQuery, attachmentsText, tokenBudget, "default")
			if err != nil {
				switch err.Error() {
				case "chunk_not_found":
					contextText = "По подходящим данным в базе ничего не найдено."
				case "date_not_found":
					contextText = "Данные не найдены."
				case "embed_limit":
					contextText = "Поиск временно перегружен (лимит эмбеддинга). Повторите вопрос через минуту."
				default:
					logHandler.Warn(ctx, "build_context failed", logging.KV{"error", err})
					contextText = ""
				}
			} else if len(buildChunkIDs) == 1 {
				// Один чанк: в LLM — полный document_context из Postgres (если есть), иначе текст от mcp-read; ref всегда сохраняем для уточнений.
				// Коллекции emocionalnoe/intellektualnye/astralnyi_duh — по-прежнему только чанки, без полного документа.
				skipFullContext := buildCollection == "emocionalnoe" || buildCollection == "intellektualnye" || buildCollection == "astralnyi_duh"
				cid := strings.TrimSpace(buildChunkIDs[0])
				if skipFullContext {
					buildContextKind = "chunks"
				} else if buildCollection != "" && cid != "" {
					ref := buildCollection + ":" + cid
					savedContextRef = ref
					if fullCtx, ok := s.ResolveFullContextFromRef(ctx, ref); ok && strings.TrimSpace(fullCtx) != "" {
						contextText = strings.TrimSpace(fullCtx)
						buildContextKind = "full"
						buildContextRef = cid
					}
				} else if cid != "" {
					// Нет коллекции в ответе — хотя бы Postgres по chunk_id
					if fullCtx, ok := s.GetFullContextByChunkIDs(ctx, buildChunkIDs); ok && strings.TrimSpace(fullCtx) != "" {
						contextText = strings.TrimSpace(fullCtx)
						buildContextKind = "full"
						buildContextRef = cid
					}
				}
			}
			if s.DebugMode == 1 {
				debugMessage = "🔍 В Qdrant отправлено (поисковый запрос):\n" + searchQuery
				if buildQueryForFilter != "" {
					debugMessage += "\n\nПосле вырезания триггеров (на поиск по словам): " + buildQueryForFilter
				}
				if len(buildCollectionsSearched) > 0 {
					debugMessage += "\n\nИскало в: " + strings.Join(buildCollectionsSearched, ", ")
				}
				if buildCollection != "" {
					debugMessage += "\n\nНайдено в: " + buildCollection
				}
				if len(buildChunkIDs) > 0 {
					debugMessage += "\n\nChunk ID: " + strings.Join(buildChunkIDs, ", ")
				}
			}
			// savedContextRef для одного чанка уже выставлен выше; здесь — случаи full от mcp-read при нескольких чанках / без раннего ref.
			if savedContextRef == "" && buildContextKind == "full" && buildContextRef != "" && buildCollection != "" {
				savedContextRef = buildCollection + ":" + buildContextRef
			}
		}
	} else if !nameAllHandled && useStoredReplyContext && s.DebugMode == 1 {
		debugMessage = "↩️ Уточнение по ответу бота: CONTEXT тот же, что использовался для того ответа (без нового поиска)."
	}

	if !nameAllHandled {
		systemContent := s.ComposeAnswerSystem(s.PromptB, replyToPrefix, contextText, len(req.MessageText))
		reply, replyErr = s.CallLLM(ctx, requestID, systemContent, req.MessageText)
		if s.DebugMode == 0 {
			reply = StripThink(reply)
		}
	}
	if replyErr != nil {
		logHandler.Error(ctx, "llm call", logging.KV{"error", replyErr}, logging.KV{"vllm_base", s.VllmBase}, logging.KV{"llm_model", s.LlmModel})
		hint := llmFailureUserHint(s, replyErr)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ChatResponse{ReplyText: hint})
		return
	}

	msgID, _ := s.AppendMessageWithReply(ctx, req.SessionID, "assistant", reply, 0)
	s.TrimSessionMessagesIfNeeded(ctx, req.SessionID)
	_ = s.SaveAnswerContext(ctx, req.SessionID, msgID, contextText, savedContextRef)

	resp := ChatResponse{ReplyText: reply, MessageID: msgID.String(), DebugMessage: debugMessage, ReminderExtraText: reminderExtraText}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleRemindersTick POST /reminders/tick — подготовка джобов и список уведомлений на текущую минуту (tg-bot рассылает).
func (s *Server) HandleRemindersTick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	items, err := s.TickReminders(ctx)
	if err != nil {
		logHandler.Warn(ctx, "reminders_tick", logging.KV{"error", err})
		http.Error(w, `{"error":"tick"}`, http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []ReminderNotify{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"notifications": items})
}
