package modules

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/telegram-ai-assistant/root/pkg/logging"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var logTG = logging.New("tg-bot.telegram")

// Лимит Telegram Bot API на одно текстовое сообщение (символы = Unicode code points).
const maxTelegramMessageRunes = 4096

// SendReply sends a text message to chat (Markdown, fallback to plain).
func (b *Bot) SendReply(ctx context.Context, chatID int64, text string) {
	b.SendReplyWithID(ctx, chatID, text)
}

// SendReplyWithID sends message and returns Telegram MessageID (0 on error).
func (b *Bot) SendReplyWithID(ctx context.Context, chatID int64, text string) int {
	if b.Bot == nil {
		return 0
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	sent, err := b.Bot.Send(msg)
	if err != nil {
		msg.ParseMode = ""
		sent, err = b.Bot.Send(msg)
	}
	if err != nil {
		logTG.Warn(ctx, "send reply", logging.KV{"error", err}, logging.KV{"chat_id", chatID})
		return 0
	}
	return sent.MessageID
}

// SendReplyToWithID sends a message as reply to another message.
func (b *Bot) SendReplyToWithID(ctx context.Context, chatID int64, replyToMessageID int, text string) int {
	if b.Bot == nil {
		return 0
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = replyToMessageID
	msg.ParseMode = "Markdown"
	sent, err := b.Bot.Send(msg)
	if err != nil {
		msg.ParseMode = ""
		sent, err = b.Bot.Send(msg)
	}
	if err != nil {
		logTG.Warn(ctx, "send reply-to", logging.KV{"error", err}, logging.KV{"chat_id", chatID})
		return 0
	}
	return sent.MessageID
}

// SendReplyTyping sends "..." and returns its MessageID (to be edited to final reply).
func (b *Bot) SendReplyTyping(ctx context.Context, chatID int64) int {
	return b.SendReplyWithID(ctx, chatID, "...")
}

// DeleteChatMessage removes a message (e.g. typing placeholder).
func (b *Bot) DeleteChatMessage(ctx context.Context, chatID int64, messageID int) {
	if b.Bot == nil || messageID <= 0 {
		return
	}
	if _, err := b.Bot.Request(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
		logTG.Warn(ctx, "delete message", logging.KV{"error", err}, logging.KV{"chat_id", chatID})
	}
}

// SendLongReply шлёт ответ серией сообщений (лимит Telegram 4096 символов на сообщение), разбивая по границам слов.
// Без Markdown (иначе _, *, ` ломают разбор сущностей). Слова длиннее лимита (URL и т.п.) режутся по символам.
func (b *Bot) SendLongReply(ctx context.Context, chatID int64, typingMsgID int, text string) int {
	if b.Bot == nil {
		return 0
	}
	text = strings.TrimSpace(text)
	if text == "" {
		text = "Пустой ответ от сервиса. Повторите запрос."
	}
	chunks := splitTelegramMessageChunks(text, maxTelegramMessageRunes)
	if len(chunks) == 0 {
		return 0
	}
	var firstMsgID int
	for i, ch := range chunks {
		if i == 0 && typingMsgID > 0 {
			if b.editMessageTextPlain(ctx, chatID, typingMsgID, ch) {
				firstMsgID = typingMsgID
			} else {
				firstMsgID = b.sendPlainMessageWithID(ctx, chatID, ch)
			}
		} else {
			id := b.sendPlainMessageWithID(ctx, chatID, ch)
			if firstMsgID == 0 {
				firstMsgID = id
			}
		}
	}
	return firstMsgID
}

func (b *Bot) sendPlainMessageWithID(ctx context.Context, chatID int64, text string) int {
	if b.Bot == nil {
		return 0
	}
	msg := tgbotapi.NewMessage(chatID, text)
	sent, err := b.Bot.Send(msg)
	if err != nil {
		logTG.Warn(ctx, "send plain chunk", logging.KV{"error", err}, logging.KV{"chat_id", chatID}, logging.KV{"len", utf8.RuneCountInString(text)})
		return 0
	}
	return sent.MessageID
}

func (b *Bot) editMessageTextPlain(ctx context.Context, chatID int64, messageID int, text string) bool {
	if b.Bot == nil || messageID <= 0 {
		return false
	}
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	_, err := b.Bot.Send(edit)
	if err != nil {
		logTG.Warn(ctx, "edit message plain", logging.KV{"error", err}, logging.KV{"chat_id", chatID})
		return false
	}
	return true
}

// splitTelegramMessageChunks режет текст на части ≤ maxRunes (лимит Telegram), по возможности между словами
// (слово = непрерывный фрагмент без unicode.IsSpace). Переносы строк сохраняются; хвостовые пробел/таб у чанка срезаются.
// Если одно «слово» длиннее лимита (URL, текст без пробелов) — режется по рунам.
func splitTelegramMessageChunks(text string, maxRunes int) []string {
	if maxRunes < 256 {
		maxRunes = maxTelegramMessageRunes
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	rs := []rune(text)
	if len(rs) <= maxRunes {
		return []string{text}
	}

	var chunks []string
	var b strings.Builder
	b.Grow(min(maxRunes*5, 65536))
	n := 0

	flush := func() {
		s := strings.TrimRight(b.String(), " \t")
		if strings.TrimSpace(s) != "" {
			chunks = append(chunks, s)
		}
		b.Reset()
		n = 0
	}

	appendRunes := func(r []rune) {
		b.WriteString(string(r))
		n += len(r)
	}

	emitOversizedWord := func(word []rune) {
		for len(word) > 0 {
			if n > 0 {
				flush()
			}
			take := maxRunes
			if take > len(word) {
				take = len(word)
			}
			chunks = append(chunks, string(word[:take]))
			word = word[take:]
		}
	}

	i := 0
	for i < len(rs) {
		wsStart := i
		for i < len(rs) && unicode.IsSpace(rs[i]) {
			i++
		}
		ws := rs[wsStart:i]

		wordStart := i
		for i < len(rs) && !unicode.IsSpace(rs[i]) {
			i++
		}
		word := rs[wordStart:i]

		for len(ws) > 0 {
			if n >= maxRunes {
				flush()
			}
			avail := maxRunes - n
			if avail == 0 {
				continue
			}
			take := len(ws)
			if take > avail {
				take = avail
			}
			appendRunes(ws[:take])
			ws = ws[take:]
		}

		if len(word) == 0 {
			continue
		}
		wlen := len(word)
		if wlen > maxRunes {
			emitOversizedWord(word)
			continue
		}
		if n+wlen <= maxRunes {
			appendRunes(word)
			continue
		}
		flush()
		appendRunes(word)
	}

	if n > 0 {
		s := strings.TrimRight(b.String(), " \t")
		if strings.TrimSpace(s) != "" {
			chunks = append(chunks, s)
		}
	}
	return chunks
}

// EditMessageText edits an existing message (Markdown, fallback plain). Для коротких системных сообщений.
func (b *Bot) EditMessageText(ctx context.Context, chatID int64, messageID int, text string) bool {
	if b.Bot == nil || messageID <= 0 {
		return false
	}
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "Markdown"
	_, err := b.Bot.Send(edit)
	if err != nil {
		edit.ParseMode = ""
		_, err = b.Bot.Send(edit)
	}
	if err != nil {
		logTG.Warn(ctx, "edit message", logging.KV{"error", err}, logging.KV{"chat_id", chatID})
		return false
	}
	return true
}

// GetChatID returns chat ID from an update.
func GetChatID(u tgbotapi.Update) int64 {
	if u.Message != nil {
		return u.Message.Chat.ID
	}
	if u.CallbackQuery != nil {
		return u.CallbackQuery.Message.Chat.ID
	}
	return 0
}
