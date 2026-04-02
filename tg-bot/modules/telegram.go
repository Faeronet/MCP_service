package modules

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/telegram-ai-assistant/root/pkg/logging"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var logTG = logging.New("tg-bot.telegram")

const maxTelegramMessageLen = 4000

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

// SendLongReply sends text in one or more chunks; if typingMsgID > 0, edits it to first chunk. Returns first message ID.
func (b *Bot) SendLongReply(ctx context.Context, chatID int64, typingMsgID int, text string) int {
	if b.Bot == nil {
		return 0
	}
	text = strings.TrimSpace(text)
	if text == "" {
		text = "Пустой ответ от сервиса. Повторите запрос."
	}
	chunks := splitMessageChunks(text, maxTelegramMessageLen)
	if len(chunks) == 0 {
		return 0
	}
	var firstMsgID int
	for i, ch := range chunks {
		if i == 0 && typingMsgID > 0 {
			if b.EditMessageText(ctx, chatID, typingMsgID, ch) {
				firstMsgID = typingMsgID
			} else {
				firstMsgID = b.SendReplyWithID(ctx, chatID, ch)
			}
		} else {
			id := b.SendReplyWithID(ctx, chatID, ch)
			if firstMsgID == 0 {
				firstMsgID = id
			}
		}
	}
	return firstMsgID
}

func splitMessageChunks(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}
	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}
		cut := text[:maxLen]
		for len(cut) > 0 && !utf8.RuneStart(cut[len(cut)-1]) {
			cut = cut[:len(cut)-1]
		}
		lastNewline := strings.LastIndex(cut, "\n")
		if lastNewline > maxLen/2 {
			cut = text[:lastNewline+1]
		}
		chunks = append(chunks, strings.TrimSpace(cut))
		text = text[len(cut):]
		for len(text) > 0 && !utf8.RuneStart(text[0]) {
			text = text[1:]
		}
		text = strings.TrimLeft(text, " \n")
	}
	return chunks
}

// EditMessageText edits an existing message. Returns false on error.
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
