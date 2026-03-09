package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/telegram-ai-assistant/root/pkg/logging"
)

var logHandler = logging.New("mcp-proxy")

// parseNumberedList разбирает текст вида "1. Имя\n2. Имя\n..." в слайс имён (индекс 0 = номер 1).
func parseNumberedList(text string) []string {
	var names []string
	re := regexp.MustCompile(`(?m)^\s*\d+[.)]\s*(.+)$`)
	for _, m := range re.FindAllStringSubmatch(strings.TrimSpace(text), -1) {
		if len(m) >= 2 {
			n := strings.TrimSpace(m[1])
			if n != "" {
				names = append(names, n)
			}
		}
	}
	return names
}

// parseNumbersFromReply извлекает числа из ответа LLM (например "25" или "3, 5, 7" или "3 0 5"). Возвращает 1-based номера; если только 0 — возвращает nil. Ноль включается в результат, если есть и другие числа.
func parseNumbersFromReply(reply string) []int {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return nil
	}
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(reply, -1)
	if len(matches) == 0 {
		return nil
	}
	if len(matches) == 1 {
		n, _ := strconv.Atoi(matches[0])
		if n == 0 {
			return nil // единственный 0 — никого нет
		}
		return []int{n}
	}
	var nums []int
	seen := make(map[int]struct{})
	for _, m := range matches {
		n, _ := strconv.Atoi(m)
		if n < 0 {
			continue
		}
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			nums = append(nums, n)
		}
	}
	return nums
}

// containsZero возвращает true, если в слайсе есть 0.
func containsZero(nums []int) bool {
	for _, n := range nums {
		if n == 0 {
			return true
		}
	}
	return false
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

	_, _ = s.AppendMessageWithReply(ctx, req.SessionID, "user", req.MessageText, req.ReplyToTelegramMessageID)
	s.TrimSessionMessagesIfNeeded(ctx, req.SessionID)

	var contextText string
	var savedContextRef string
	var debugMessage string
	var reply string
	var replyErr error
	var nameAllHandled bool

	// Контекст списка name all: ответ на список или следующий вопрос после списка (последнее сообщение ассистента — нумерованный список)
	var botA string
	var userQ, ctxStored string
	if req.ReplyToTelegramMessageID != 0 {
		userQ, botA, ctxStored, _ = s.GetReplyToContext(ctx, req.SessionID, req.ReplyToTelegramMessageID)
		if botA == "" {
			if lastMsg, lastOk := s.GetLastAssistantMessage(ctx, req.SessionID); lastOk && lastMsg != "" {
				botA = lastMsg
			}
		}
	} else {
		// Пользователь не нажал «Ответить», но последнее сообщение в сессии — список имён → считаем вопрос по контексту списка
		if lastMsg, lastOk := s.GetLastAssistantMessage(ctx, req.SessionID); lastOk && lastMsg != "" && len(parseNumberedList(lastMsg)) >= 3 {
			botA = lastMsg
		}
	}
	if botA != "" {
		namesFromList := parseNumberedList(botA)
		// Ответ по контексту списка (name all): промпт C → номер(а) → контекст из Postgres → промпт B
		if len(namesFromList) >= 3 {
				systemC := s.PromptC + "\n\nСписок:\n" + botA
				replyC, errC := s.CallLLM(ctx, requestID, systemC, req.MessageText)
				if errC == nil {
					replyC = strings.TrimSpace(StripThink(replyC))
					numbers := parseNumbersFromReply(replyC)
					if numbers != nil && len(numbers) > 1 && containsZero(numbers) {
						reply = "Один или несколько из указанных номеров не существуют в списке. Проверьте номера и попробуйте снова."
						nameAllHandled = true
					} else if numbers == nil {
						// Число 0 или пусто — симулируем ответ ИИ
						reply = "По этому списку никого не найдено. Уточните, о ком или о чём вы спрашиваете."
						nameAllHandled = true
					} else {
						var fullContextParts []string
						for _, num := range numbers {
							if num == 0 || num < 1 || num > len(namesFromList) {
								continue
							}
							name := namesFromList[num-1]
							if ctxPart, ok := s.GetFullContextByAngelName(ctx, name); ok && ctxPart != "" {
								fullContextParts = append(fullContextParts, ctxPart)
							}
						}
						if len(fullContextParts) > 0 {
							combinedContext := strings.Join(fullContextParts, "\n\n")
							systemB := s.PromptB + "\n\nСписок (для ориентира):\n" + botA + "\n\nКонтекст:\n" + combinedContext
							reply, replyErr = s.CallLLM(ctx, requestID, systemB, req.MessageText)
							if replyErr == nil {
								reply = StripThink(reply)
								nameAllHandled = true
							}
						} else {
							reply = "По этому списку никого не найдено. Уточните, о ком или о чём вы спрашиваете."
							nameAllHandled = true
						}
					}
				}
			}
			if !nameAllHandled && (userQ != "" || botA != "" || ctxStored != "") {
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
				// name all: только нумерованный список из БД, без дополнения от LLM (LLM может придумывать имена).
				if s.DebugMode == 1 {
					debugMessage = "🔍 Список имён из Postgres (name all)"
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
						bld.WriteString(strconv.Itoa(i + 1))
						bld.WriteString(". ")
						bld.WriteString(n)
					}
					reply = bld.String()
					nameAllHandled = true
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
