import { useState, useEffect } from 'react'
import { listChats, getChatMessages, type ChatListItem, type ChatMessage } from '../api'
import { useToast } from '../context/ToastContext'

function formatTime(iso: string): string {
  try {
    const d = new Date(iso)
    return d.toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })
  } catch {
    return iso
  }
}

export function ChatLog() {
  const [chats, setChats] = useState<ChatListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [modalSession, setModalSession] = useState<ChatListItem | null>(null)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [messagesLoading, setMessagesLoading] = useState(false)
  const toast = useToast()

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
    getChatMessages(modalSession.session_id)
      .then(({ messages: m }) => setMessages(Array.isArray(m) ? m : []))
      .catch(e => toast.error(e instanceof Error ? e.message : 'Failed to load messages'))
      .finally(() => setMessagesLoading(false))
  }, [modalSession?.session_id])

  const closeModal = () => setModalSession(null)

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) closeModal()
  }

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
            <div className="chat-modal-body">
              {messagesLoading ? (
                <p className="text-muted">Загрузка…</p>
              ) : messages.length === 0 ? (
                <p className="text-muted">Нет сообщений</p>
              ) : (
                <div className="chat-messages">
                  {messages.map((msg, i) => (
                    <div
                      key={i}
                      className={`chat-msg chat-msg--${msg.role === 'user' ? 'user' : 'assistant'}`}
                    >
                      <div className="chat-msg-bubble">
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
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
