package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var logHandler = logging.New("mcp-proxy")

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

	_, _ = s.AppendMessageWithReply(ctx, req.SessionID, "user", req.MessageText, req.ReplyToTelegramMessageID)
	s.TrimSessionMessagesIfNeeded(ctx, req.SessionID)

	var contextText string
	var savedContextRef string
	var debugMessage string
	var reply string
	var replyErr error
	var nameAllHandled bool

	if req.ReplyToTelegramMessageID != 0 {
		if userQ, botA, ctxStored, ok := s.GetReplyToContext(ctx, req.SessionID, req.ReplyToTelegramMessageID); ok && (userQ != "" || botA != "" || ctxStored != "") {
			var bld strings.Builder
			if userQ != "" {
				bld.WriteString("Предыдущий вопрос: ")
				bld.WriteString(userQ)
				bld.WriteString("\n")
			}
			if botA != "" {
				bld.WriteString("Ответ: ")
				bld.WriteString(botA)
				bld.WriteString("\n\n")
			}
			if ctxStored != "" {
				bld.WriteString("Контекст:\n")
				bld.WriteString(ctxStored)
			}
			contextText = bld.String()
			logHandler.Info(ctx, "using reply-to context", logging.KV{"chat_id", req.ChatID})
		}
	}

	if contextText == "" {
		var searchQuery string
		if q, err := s.ExtractSearchQuery(ctx, requestID, req.MessageText); err == nil && strings.TrimSpace(q) != "" {
			searchQuery = q
		} else {
			searchQuery = ExtractDateFromQuestion(req.MessageText)
			if searchQuery == "" {
				searchQuery = req.MessageText
			}
		}
		lowerMsg := strings.ToLower(strings.TrimSpace(req.MessageText))
		if HasAngelWord(req.MessageText) {
			if strings.Contains(lowerMsg, "все ангел") || strings.Contains(lowerMsg, "всех ангел") ||
				strings.Contains(lowerMsg, "ангел вс") || strings.Contains(lowerMsg, "ангелов вс") ||
				strings.Contains(lowerMsg, "перечисли всех") || strings.Contains(lowerMsg, "список всех") {
				searchQuery = "[name] all"
			} else if strings.Contains(lowerMsg, "сколько имен") || strings.Contains(lowerMsg, "количество имен") ||
				strings.Contains(lowerMsg, "число имен") || strings.Contains(lowerMsg, "сколько ангел") || strings.Contains(lowerMsg, "количество ангел") {
				searchQuery = "name number"
			} else if strings.Contains(lowerMsg, "дат") {
				searchQuery = "[date] list"
			}
		}
		userDateStr := ExtractDateFromQuestion(req.MessageText)
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
		day, month, hasDate := ParseDayMonthFromQuery(searchQuery)
		if hasDate && (day < 1 || day > MaxDaysInMonth(month)) {
			contextText = "date not found"
		} else if IsMetaQuestionAboutBot(searchQuery) {
			contextText = ""
		} else {
			normalizedQuery := strings.TrimSpace(strings.ToLower(searchQuery))
			nq := strings.ReplaceAll(strings.ReplaceAll(normalizedQuery, "[", ""), "]", "")
			nq = strings.TrimSpace(nq)
			if IsNameNumberQuery(nq) {
				// name number: тот же контекст, что при name all — в LLM уходит полный контекст из Postgres + вопрос пользователя.
				if s.DebugMode == 1 {
					debugMessage = "🔍 Список имён из Postgres (name number)"
				}
				var errNames error
				contextText, errNames = s.GetAngelNamesFromPostgres(ctx)
				if errNames != nil {
					logHandler.Warn(ctx, "getAngelNamesFromPostgres failed", logging.KV{"error", errNames})
					contextText = ""
				}
			} else if IsNameAllQuery(nq) || normalizedQuery == "[name] all" {
				// name all: берём имена списком, просим LLM дополнить в стиле «я пока знаю только эти имена», склеиваем список + ответ LLM.
				if s.DebugMode == 1 {
					debugMessage = "🔍 Список имён + дополнение LLM (name all)"
				}
				names, errList := s.GetAngelNamesList(ctx)
				if errList != nil {
					contextText = ""
				} else if len(names) == 0 {
					reply = "В базе пока нет имён ангелов-хранителей."
					nameAllHandled = true
				} else {
					var bld strings.Builder
					for i, n := range names {
						if i > 0 {
							bld.WriteString("\n")
						}
						bld.WriteString(n)
					}
					listStr := bld.String()
					sys := s.PromptB + "\n\nТы дополняешь список имён ангелов-хранителей. Отвечай только именами, в том же стиле, без пояснений."
					userPrompt := "Я пока знаю только эти имена ангелов-хранителей:\n" + listStr + "\n\nДополни этот список в том же стиле: только имена, без пояснений."
					llmReply, errLLM := s.CallLLM(ctx, requestID, sys, userPrompt)
					if errLLM == nil {
						reply = listStr + "\n" + strings.TrimSpace(StripThink(llmReply))
						nameAllHandled = true
					}
				}
			} else if normalizedQuery == "[date] list" {
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
				contextText, buildChunkIDs, buildCollection, buildCollectionsSearched, buildQueryForFilter, buildContextKind, buildContextRef, err = s.BuildContext(ctx, requestID, searchQuery, attachmentsText, 4000, "default")
				if err != nil {
					if err.Error() == "chunk_not_found" {
						contextText = "По подходящим данным в базе ничего не найдено."
					} else if err.Error() == "date_not_found" {
						contextText = "Данные не найдены."
					} else {
						logHandler.Warn(ctx, "build_context failed", logging.KV{"error", err})
						contextText = ""
					}
				} else if len(buildChunkIDs) <= 2 && len(buildChunkIDs) > 0 {
					// При 1–2 чанках подставляем полный контекст из Postgres (core.document_context)
					if fullCtx, ok := s.GetFullContextByChunkIDs(ctx, buildChunkIDs); ok && fullCtx != "" {
						contextText = fullCtx
						buildContextKind = "full"
						if len(buildChunkIDs) > 0 {
							buildContextRef = buildChunkIDs[0]
						}
					}
				}
				if s.DebugMode == 1 {
					debugMessage = "🔍 В Qdrant отправлено (ответ модели на промпт A):\n" + searchQuery
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
				if buildContextKind == "full" && buildContextRef != "" && buildCollection != "" {
					savedContextRef = buildCollection + ":" + buildContextRef
				}
			}
		}
	}

	if !nameAllHandled {
		systemContent := s.PromptB + "\n" + contextText
		reply, replyErr = s.CallLLM(ctx, requestID, systemContent, req.MessageText)
		if s.DebugMode == 0 {
			reply = StripThink(reply)
		}
	}
	if replyErr != nil {
		logHandler.Error(ctx, "llm call", logging.KV{"error", replyErr})
		hint := "Модель недоступна. Проверьте, что vLLM запущен и в .env указан VLLM_OPENAI_BASE."
		if errStr := replyErr.Error(); len(errStr) < 120 {
			hint = "Ошибка LLM: " + errStr
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ChatResponse{ReplyText: hint})
		return
	}

	msgID, _ := s.AppendMessageWithReply(ctx, req.SessionID, "assistant", reply, 0)
	s.TrimSessionMessagesIfNeeded(ctx, req.SessionID)
	_ = s.SaveAnswerContext(ctx, req.SessionID, msgID, contextText, savedContextRef)

	resp := ChatResponse{ReplyText: reply, MessageID: msgID.String(), DebugMessage: debugMessage}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
