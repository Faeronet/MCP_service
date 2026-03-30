// Пустая строка = тот же хост (nginx проксирует /api на backend). Для доступа с другого устройства используем origin.
const API_URL = (() => {
  const env = import.meta.env.VITE_API_URL
  if (env && typeof env === 'string' && env.trim() !== '') return env.trim()
  if (typeof window !== 'undefined') return window.location.origin
  return ''
})()

function getToken(): string {
  return localStorage.getItem('token') || ''
}

function checkAuth(r: Response): void {
  if (r.status === 401) {
    localStorage.removeItem('token')
    window.dispatchEvent(new Event('auth-change'))
  }
}

export interface LoginResult {
  token: string
  reminder_debug?: boolean
}

export async function login(username: string, password: string): Promise<LoginResult> {
  let r: Response
  try {
    r = await fetch(`${API_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })
  } catch (e) {
    throw new Error('CONNECTION_ERROR')
  }
  if (r.status === 401) throw new Error('INVALID_CREDENTIALS')
  if (!r.ok) throw new Error(`SERVER_ERROR:${r.status}`)
  const data = await r.json()
  if (!data || typeof data.token !== 'string') throw new Error('INVALID_RESPONSE')
  if (data.reminder_debug === true) {
    localStorage.setItem('reminder_debug', '1')
  } else {
    localStorage.removeItem('reminder_debug')
  }
  return data
}

export async function getRemindersConfig(): Promise<{ disabled: boolean; simulated_at: string | null }> {
  const r = await fetch(`${API_URL}/api/reminders/config`, { headers: { Authorization: `Bearer ${getToken()}` } })
  checkAuth(r)
  if (!r.ok) throw new Error('Failed to load reminders config')
  return r.json()
}

export async function setRemindersDisabled(disabled: boolean): Promise<void> {
  const r = await fetch(`${API_URL}/api/reminders/toggle`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
    body: JSON.stringify({ disabled }),
  })
  checkAuth(r)
  if (!r.ok) throw new Error('Failed to toggle reminders')
}

export async function setRemindersDebugClock(body: { simulated_iso?: string; clear?: boolean }): Promise<void> {
  const r = await fetch(`${API_URL}/api/reminders/debug-clock`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
    body: JSON.stringify(body),
  })
  checkAuth(r)
  if (!r.ok) throw new Error('Failed to set debug clock')
}

export interface ReminderSubscriber {
  telegram_id: number
  chat_id: number
  username: string
  reminder_hh: number
  reminder_mm: number
  enabled: boolean
  updated_at: string
}

export async function getRemindersSubscribers(): Promise<{ subscribers: ReminderSubscriber[] }> {
  const r = await fetch(`${API_URL}/api/reminders/subscribers`, {
    headers: { Authorization: `Bearer ${getToken()}` },
  })
  checkAuth(r)
  if (!r.ok) throw new Error('Failed to load reminders subscribers')
  return r.json()
}

export async function resetRemindersForUser(telegramId: number): Promise<void> {
  const r = await fetch(`${API_URL}/api/reminders/reset-user`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
    body: JSON.stringify({ telegram_id: telegramId }),
  })
  checkAuth(r)
  if (!r.ok) throw new Error('Failed to reset reminders for user')
}

export async function uploadFile(file: File, name?: string): Promise<{ job_id: string; doc_id: string; status: string }> {
  const token = getToken()
  if (!token) throw new Error('NOT_LOGGED_IN')
  const form = new FormData()
  form.append('file', file)
  if (name) form.append('name', name)
  const r = await fetch(`${API_URL}/api/upload`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: form,
  })
  checkAuth(r)
  if (!r.ok) {
    const text = await r.text()
    try {
      const body = JSON.parse(text) as { error?: string }
      if (body?.error === 'token_expired') throw new Error('Сессия истекла. Войдите снова.')
    } catch (e) {
      if (e instanceof Error && e.message.startsWith('Сессия истекла')) throw e
    }
    throw new Error(text || 'Upload failed')
  }
  return r.json()
}

export async function listDocs(): Promise<{ docs: Array<{ id: string; name: string; created_at: string; versions: unknown }> }> {
  const r = await fetch(`${API_URL}/api/docs`, { headers: { Authorization: `Bearer ${getToken()}` } })
  if (!r.ok) throw new Error('Failed to load docs')
  return r.json()
}

export async function deleteDoc(docId: string): Promise<{ status: string; doc_id: string }> {
  const r = await fetch(`${API_URL}/api/docs/${docId}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${getToken()}` },
  })
  checkAuth(r)
  if (r.status === 404) throw new Error('Document not found')
  if (!r.ok) throw new Error(await r.text() || 'Failed to delete document')
  return r.json()
}

export async function listJobs(limit?: number): Promise<{ jobs: Array<Record<string, unknown>> }> {
  const u = new URL(`${API_URL}/api/jobs`)
  if (limit) u.searchParams.set('limit', String(limit))
  const r = await fetch(u.toString(), { headers: { Authorization: `Bearer ${getToken()}` } })
  if (!r.ok) throw new Error('Failed to load jobs')
  return r.json()
}

export async function getJob(id: string): Promise<Record<string, unknown>> {
  const r = await fetch(`${API_URL}/api/jobs/${id}`, { headers: { Authorization: `Bearer ${getToken()}` } })
  if (!r.ok) throw new Error('Failed to load job')
  return r.json()
}

export async function searchLogs(params: { service?: string; request_id?: string; level?: string; limit?: number; offset?: number }): Promise<{ logs: Array<Record<string, unknown>>; total: number }> {
  const u = new URL(`${API_URL}/api/logs/search`)
  if (params.service) u.searchParams.set('service', params.service)
  if (params.request_id) u.searchParams.set('request_id', params.request_id)
  if (params.level) u.searchParams.set('level', params.level)
  if (params.limit != null) u.searchParams.set('limit', String(params.limit))
  if (params.offset != null) u.searchParams.set('offset', String(params.offset))
  const r = await fetch(u.toString(), { headers: { Authorization: `Bearer ${getToken()}` } })
  checkAuth(r)
  if (!r.ok) throw new Error('Failed to search logs')
  return r.json()
}

export interface ChatListItem {
  session_id: string
  telegram_id: number
  chat_id: number
  username: string
  message_count: number
}

export async function listChats(): Promise<{ chats: ChatListItem[] }> {
  const r = await fetch(`${API_URL}/api/chats`, { headers: { Authorization: `Bearer ${getToken()}` } })
  checkAuth(r)
  if (!r.ok) throw new Error('Failed to load chats')
  return r.json()
}

export interface ChatMessage {
  id?: string
  role: string
  content: string
  created_at: string
  response_time_sec?: number
  /** ID сообщения в Telegram (есть у ответов бота) */
  telegram_message_id?: number
  /** Ответ пользователя по контексту этого сообщения (telegram_message_id сообщения бота) */
  reply_to_telegram_message_id?: number
}

export async function getChatMessages(sessionId: string): Promise<{ messages: ChatMessage[] }> {
  const r = await fetch(`${API_URL}/api/chats/${encodeURIComponent(sessionId)}/messages`, {
    headers: { Authorization: `Bearer ${getToken()}` },
  })
  checkAuth(r)
  if (!r.ok) throw new Error('Failed to load messages')
  return r.json()
}

export interface SendChatMessageResult {
  reply_text: string
  debug_message?: string
  telegram_message_id?: number
}

/** Тестовый чат с LLM через admin-backend → mcp-proxy (сессия admin в БД). */
export async function sendChatMessage(
  messageText: string,
  replyToTelegramMessageId?: number
): Promise<SendChatMessageResult> {
  const r = await fetch(`${API_URL}/api/chat/llm`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getToken()}` },
    body: JSON.stringify({
      message_text: messageText,
      reply_to_telegram_message_id: replyToTelegramMessageId ?? 0,
    }),
  })
  checkAuth(r)
  if (!r.ok) {
    const t = await r.text()
    throw new Error(t || `chat ${r.status}`)
  }
  return r.json()
}

export function grafanaUrl(): string {
  const base = `${API_URL}/api/grafana/`
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : ''
  return token ? `${base}?token=${encodeURIComponent(token)}` : base
}

export interface GPUMetrics {
  name?: string
  gpu_pct: number
  vram_pct: number
  vram_used_gb?: number
  vram_total_gb?: number
}

export interface ContainerHistoryPoint {
  ts: string
  cpu: number
  ram: number
}

export interface ContainerMetrics {
  name: string
  cpu_pct: number
  ram_pct: number
  ram_used_gb?: number
  ram_limit_gb?: number
  history?: ContainerHistoryPoint[]
}

export interface MonitorMetricsResponse {
  system: {
    cpu_pct: number
    ram_pct: number
    ram_used_gb?: number
    ram_total_gb?: number
    disk_io_k: number
  }
  gpu?: { gpu_pct: number; vram_pct: number }
  gpus: GPUMetrics[]
  containers?: ContainerMetrics[]
  uptime_sec?: number
  history: Array<{
    ts: string
    cpu: number
    ram: number
    disk_io: number
    gpu: number
    vram: number
    gpus?: Array<{ gpu_pct: number; vram_pct: number }>
  }>
}

export async function getMonitorMetrics(): Promise<MonitorMetricsResponse> {
  const r = await fetch(`${API_URL}/api/monitor/metrics`, {
    headers: { Authorization: `Bearer ${getToken()}` },
  })
  checkAuth(r)
  if (!r.ok) throw new Error('Failed to load monitor metrics')
  return r.json()
}
