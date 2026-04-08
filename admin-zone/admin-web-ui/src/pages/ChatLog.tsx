import { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { listChats, getChatMessages, getChatLogStats, type ChatListItem, type ChatMessage, type ChatLogStatsPoint } from '../api'
import { useToast } from '../context/ToastContext'

const CHAT_MESSAGES_POLL_MS = 5000
const CHAT_STATS_POLL_MS = 10000

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

function clamp(n: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, n))
}

function compressToBuckets(src: ChatLogStatsPoint[], buckets: number): ChatLogStatsPoint[] {
  if (src.length <= buckets || buckets <= 0) return src
  const out: ChatLogStatsPoint[] = []
  const step = src.length / buckets
  for (let i = 0; i < buckets; i++) {
    const from = Math.floor(i * step)
    const to = Math.min(src.length, Math.floor((i + 1) * step))
    let maxCount = 0
    let ts = src[from]?.ts || src[src.length - 1]?.ts || new Date().toISOString()
    for (let j = from; j < to; j++) {
      const p = src[j]
      if (p.count >= maxCount) {
        maxCount = p.count
        ts = p.ts
      }
    }
    out.push({ ts, count: maxCount })
  }
  return out
}

export function ChatLog() {
  const [chats, setChats] = useState<ChatListItem[]>([])
  const [totalChats, setTotalChats] = useState<number>(0)
  const [llmInflight, setLlmInflight] = useState<number>(0)
  const [llmSeries24h, setLlmSeries24h] = useState<ChatLogStatsPoint[]>([])
  const [showGraph, setShowGraph] = useState(false)
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

  const loadStats = useCallback(async () => {
    try {
      const s = await getChatLogStats()
      setTotalChats(typeof s.total_chats === 'number' ? s.total_chats : 0)
      setLlmInflight(typeof s.llm_inflight === 'number' ? s.llm_inflight : 0)
      setLlmSeries24h(Array.isArray(s.series_24h) ? s.series_24h : [])
    } catch {
      // not fatal for chat table
    }
  }, [])

  useEffect(() => {
    loadChats()
    void loadStats()
  }, [])

  useEffect(() => {
    const t = setInterval(() => {
      void loadStats()
    }, CHAT_STATS_POLL_MS)
    return () => clearInterval(t)
  }, [loadStats])

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

  const chartData = useMemo(() => compressToBuckets(llmSeries24h, 96), [llmSeries24h])
  const chartPeakRaw = useMemo(() => chartData.reduce((m, p) => Math.max(m, p.count), 0), [chartData])
  const chartYMax = useMemo(() => clamp(chartPeakRaw, 5, 100), [chartPeakRaw])

  return (
    <div className="page-layout">
      <div className="page-header">
        <h1 className="page-title">Chat Log</h1>
        <p className="text-muted">Чаты из PostgreSQL (chat.sessions, chat.messages).</p>
        <div className="input-line">
          <span className="text-muted">Всего чатов: <b>{totalChats || chats.length}</b></span>
          <span className="text-muted">Сейчас в LLM: <b>{llmInflight}</b></span>
          <button type="button" className="btn-monitor-inactive" onClick={() => setShowGraph(v => !v)}>
            {showGraph ? 'Скрыть график' : 'График 24ч'}
          </button>
        </div>
        {showGraph && (
          <div style={{ marginTop: 8, border: '1px solid #2f3441', borderRadius: 8, padding: 8 }}>
            <div className="text-muted" style={{ marginBottom: 6 }}>
              Одновременные запросы к LLM за 24 часа · пик: {chartPeakRaw} · шкала: 0..{chartYMax}
            </div>
            <div style={{ height: 120, display: 'flex', alignItems: 'flex-end', gap: 1 }}>
              {chartData.length === 0 ? (
                <span className="text-muted">Нет данных</span>
              ) : (
                chartData.map((p, i) => {
                  const h = Math.max(2, Math.round((p.count / chartYMax) * 100))
                  return (
                    <div
                      key={`${p.ts}-${i}`}
                      title={`${formatTime(p.ts)}: ${p.count}`}
                      style={{ width: 4, height: `${h}%`, background: '#7c8cf8', borderRadius: 2 }}
                    />
                  )
                })
              )}
            </div>
          </div>
        )}
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
                        <td style={{ width: '33%' }}>{c.telegram_id === -1 ? 'admin' : (c.username || '—')}</td>
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
                {modalSession.telegram_id === -1 ? 'admin' : (modalSession.username || '—')} · Chat {modalSession.chat_id}
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
                              {msg.role === 'user'
                                ? (modalSession.telegram_id === -1 ? 'Админ' : 'Пользователь')
                                : 'Ассистент'}
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
