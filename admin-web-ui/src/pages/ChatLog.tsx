import { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { listChats, getChatMessages, type ChatListItem, type ChatMessage } from '../api'
import { useToast } from '../context/ToastContext'

const CHAT_MESSAGES_POLL_MS = 5000

function formatTime(iso: string): string {
  try {
    const d = new Date(iso)
    return d.toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })
  } catch {
    return iso
  }
}

function truncateForReply(text: string, maxLen: number = 120): string {
  const t = text.replace(/\s+/g, ' ').trim()
  return t.length <= maxLen ? t : t.slice(0, maxLen) + '…'
}

export function ChatLog() {
  const [chats, setChats] = useState<ChatListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [modalSession, setModalSession] = useState<ChatListItem | null>(null)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [messagesLoading, setMessagesLoading] = useState(false)
  const modalBodyRef = useRef<HTMLDivElement | null>(null)
  const toast = useToast()

  const fetchMessages = useCallback(
    (sessionId: string) => {
      return getChatMessages(sessionId)
        .then(({ messages: m }) => setMessages(Array.isArray(m) ? m : []))
        .catch(e => toast.error(e instanceof Error ? e.message : 'Failed to load messages'))
    },
    [toast]
  )

  const didInitialScrollRef = useRef(false)
  useEffect(() => {
    if (!modalSession) {
      didInitialScrollRef.current = false
      return
    }
    if (!messagesLoading && messages.length > 0 && modalBodyRef.current && !didInitialScrollRef.current) {
      didInitialScrollRef.current = true
      const el = modalBodyRef.current
      el.scrollTop = el.scrollHeight
      requestAnimationFrame(() => { el.scrollTop = el.scrollHeight })
    }
  }, [modalSession, messagesLoading, messages.length])

  const loadChats = async () => {
    setLoading(true)
    try {
      const { chats: list } = await listChats()
      setChats(Array.isArray(list) ? list : [])
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to load chats')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadChats()
  }, [])

  useEffect(() => {
    if (!modalSession) {
      setMessages([])
      return
    }
    setMessagesLoading(true)
    fetchMessages(modalSession.session_id).finally(() => setMessagesLoading(false))
  }, [modalSession?.session_id, fetchMessages])

  useEffect(() => {
    if (!modalSession) return
    const t = setInterval(() => {
      fetchMessages(modalSession.session_id)
    }, CHAT_MESSAGES_POLL_MS)
    return () => clearInterval(t)
  }, [modalSession?.session_id, fetchMessages])

  const closeModal = () => setModalSession(null)

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) closeModal()
  }

  const scrollToTelegramMessageId = (telegramMessageId: number) => {
    const container = modalBodyRef.current
    if (!container) return
    const target = container.querySelector(`[data-telegram-message-id="${telegramMessageId}"]`) as HTMLElement | null
    if (!target) return
    const containerRect = container.getBoundingClientRect()
    const targetRect = target.getBoundingClientRect()
    const scrollTop = container.scrollTop + (targetRect.top - containerRect.top) - containerRect.height / 2 + targetRect.height / 2
    container.scrollTo({ top: Math.max(0, scrollTop), behavior: 'smooth' })
  }

  const messageByTelegramId = useMemo(() => {
    const map = new Map<number, ChatMessage>()
    for (const m of messages) {
      if (m.telegram_message_id != null) map.set(m.telegram_message_id, m)
    }
    return map
  }, [messages])

  return (
    <div className="page-layout">
      <div className="page-header">
        <h1 className="page-title">Chat Log</h1>
        <p className="text-muted">Чаты из PostgreSQL (chat.sessions, chat.messages).</p>
      </div>
      <div className="content-panel">
        {loading ? (
          <p className="text-muted">Loading…</p>
        ) : (
          <div className="table-wrap table-wrap-logs">
            <div className="table-header-wrap">
              <table className="data-table data-table-header">
                <thead>
                  <tr>
                    <th style={{ width: '33%' }}>Username</th>
                    <th style={{ width: '33%' }}>Chat ID</th>
                    <th style={{ width: '34%' }}>Сообщений</th>
                  </tr>
                </thead>
              </table>
            </div>
            <div className="table-body-wrap">
              {chats.length === 0 ? (
                <div className="table-empty">
                  <p className="table-empty-msg">Чатов пока нет</p>
                  <p className="text-muted table-empty-hint">Данные из chat.sessions и core.users.</p>
                </div>
              ) : (
                <table className="data-table data-table-body">
                  <tbody>
                    {chats.map((c) => (
                      <tr
                        key={c.session_id}
                        className="chat-log-row"
                        onClick={() => setModalSession(c)}
                        role="button"
                        tabIndex={0}
                        onKeyDown={(e) => e.key === 'Enter' && setModalSession(c)}
                      >
                        <td style={{ width: '33%' }}>{c.username || '—'}</td>
                        <td style={{ width: '33%' }} className="mono">{String(c.chat_id)}</td>
                        <td style={{ width: '34%' }}>{c.message_count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        )}
      </div>

      {modalSession && (
        <div className="chat-modal-overlay" onClick={handleBackdropClick} role="presentation">
          <div className="chat-modal" onClick={(e) => e.stopPropagation()}>
            <div className="chat-modal-header">
              <h2 className="chat-modal-title">
                {modalSession.username || '—'} · Chat {modalSession.chat_id}
              </h2>
              <button
                type="button"
                className="chat-modal-close"
                onClick={closeModal}
                aria-label="Закрыть"
              >
                ×
              </button>
            </div>
            <div className="chat-modal-body" ref={modalBodyRef}>
              {messagesLoading && messages.length === 0 ? (
                <p className="text-muted">Загрузка…</p>
              ) : messages.length === 0 ? (
                <p className="text-muted">Нет сообщений</p>
              ) : (
                <div className="chat-messages">
                  {messages.map((msg, i) => {
                    const replyToMsg = msg.reply_to_telegram_message_id != null
                      ? messageByTelegramId.get(msg.reply_to_telegram_message_id)
                      : undefined
                    return (
                      <div
                        key={msg.id ?? i}
                        className={`chat-msg chat-msg--${msg.role === 'user' ? 'user' : 'assistant'}`}
                        data-message-id={msg.id ?? undefined}
                        data-telegram-message-id={msg.telegram_message_id ?? undefined}
                      >
                        <div className="chat-msg-bubble">
                          {replyToMsg != null && (
                            <button
                              type="button"
                              className="chat-msg-reply-to"
                              onClick={() => scrollToTelegramMessageId(msg.reply_to_telegram_message_id!)}
                              title="Перейти к сообщению, на которое отвечали"
                            >
                              <span className="chat-msg-reply-to-who">
                                {replyToMsg.role === 'assistant' ? 'Ассистент' : 'Пользователь'}
                              </span>
                              <span className="chat-msg-reply-to-text">
                                {truncateForReply(replyToMsg.content)}
                              </span>
                            </button>
                          )}
                          <div className="chat-msg-meta">
                            <span className="chat-msg-who">
                              {msg.role === 'user' ? 'Пользователь' : 'Ассистент'}
                            </span>
                            <span className="chat-msg-time">{formatTime(msg.created_at)}</span>
                            {msg.role === 'assistant' && msg.response_time_sec != null && (
                              <span className="chat-msg-response-time">
                                Ответ за {msg.response_time_sec} с
                              </span>
                            )}
                          </div>
                          <div className="chat-msg-content">{msg.content}</div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
