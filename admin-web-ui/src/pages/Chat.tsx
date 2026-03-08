import { useState, useRef, useEffect } from 'react'
import { sendChatMessage } from '../api'
import { useToast } from '../context/ToastContext'
import { Send, Bot, User, Reply } from 'lucide-react'

type Message = {
  role: 'user' | 'assistant'
  content: string
  debug?: string
  /** ID ответа бота (для ответа по контексту) */
  telegram_message_id?: number
}

function truncateReply(text: string, maxLen: number = 80): string {
  const t = text.replace(/\s+/g, ' ').trim()
  return t.length <= maxLen ? t : t.slice(0, maxLen) + '…'
}

export function Chat() {
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [replyingTo, setReplyingTo] = useState<{ telegram_message_id: number; content: string } | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const toast = useToast()

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  useEffect(() => {
    scrollToBottom()
  }, [messages])

  const replyToMessageId = replyingTo ? replyingTo.telegram_message_id : undefined

  const send = async () => {
    const text = input.trim()
    if (!text || sending) return
    setInput('')
    setReplyingTo(null)
    setMessages(prev => [...prev, { role: 'user', content: text }])
    setSending(true)
    try {
      const { reply_text, debug_message, telegram_message_id } = await sendChatMessage(text, replyToMessageId)
      setMessages(prev => [
        ...prev,
        {
          role: 'assistant',
          content: reply_text || '(пустой ответ)',
          debug: debug_message,
          telegram_message_id: telegram_message_id ?? undefined,
        },
      ])
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка отправки')
      setMessages(prev => prev.slice(0, -1))
    } finally {
      setSending(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      send()
    }
  }

  const handleReply = (msg: Message) => {
    if (msg.role === 'assistant' && msg.telegram_message_id != null && msg.telegram_message_id > 0) {
      setReplyingTo({ telegram_message_id: msg.telegram_message_id, content: msg.content })
    } else {
      setReplyingTo({ telegram_message_id: 0, content: msg.content })
    }
  }

  return (
    <div className="chat-page">
      <h1 className="page-title">Чат с LLM</h1>
      <div className="chat-container">
        <div className="chat-messages">
          {messages.length === 0 && (
            <div className="chat-placeholder">
              <Bot size={48} strokeWidth={1.5} />
              <p>Напишите сообщение — ответ придёт через mcp-proxy. Можно ответить на сообщение по контексту.</p>
            </div>
          )}
          {messages.map((msg, i) => (
            <div key={i} className={`chat-bubble chat-bubble--${msg.role}`}>
              <span className="chat-bubble-icon" aria-hidden>
                {msg.role === 'user' ? <User size={20} /> : <Bot size={20} />}
              </span>
              <div className="chat-bubble-content">
                <div className="chat-bubble-text">{msg.content}</div>
                {msg.debug && (
                  <details className="chat-bubble-debug">
                    <summary>Debug</summary>
                    <pre>{msg.debug}</pre>
                  </details>
                )}
                <button
                  type="button"
                  className="chat-bubble-reply"
                  onClick={() => handleReply(msg)}
                  title="Ответить по контексту"
                >
                  <Reply size={14} />
                  Ответить
                </button>
              </div>
            </div>
          ))}
          <div ref={messagesEndRef} />
        </div>
        <div className="chat-input-area">
          {replyingTo && (
            <div className="chat-reply-preview">
              <span className="chat-reply-preview-label">Ответ на:</span>
              <span className="chat-reply-preview-text">{truncateReply(replyingTo.content)}</span>
              <button
                type="button"
                className="chat-reply-preview-cancel"
                onClick={() => setReplyingTo(null)}
                aria-label="Отменить ответ"
              >
                ×
              </button>
            </div>
          )}
          <div className="chat-input-wrap">
            <textarea
              className="chat-input"
              placeholder="Сообщение..."
              value={input}
              onChange={e => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              rows={1}
              disabled={sending}
            />
            <button
              type="button"
              className="chat-send"
              onClick={send}
              disabled={sending || !input.trim()}
              title="Отправить"
            >
              <Send size={20} />
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
