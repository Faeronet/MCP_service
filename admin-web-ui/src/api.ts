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

export async function login(username: string, password: string): Promise<{ token: string }> {
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
  return data
}

export async function uploadFile(file: File, name?: string): Promise<{ job_id: string; doc_id: string; status: string }> {
  const form = new FormData()
  form.append('file', file)
  if (name) form.append('name', name)
  const r = await fetch(`${API_URL}/api/upload`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${getToken()}` },
    body: form,
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

export async function listDocs(): Promise<{ docs: Array<{ id: string; name: string; created_at: string; versions: unknown }> }> {
  const r = await fetch(`${API_URL}/api/docs`, { headers: { Authorization: `Bearer ${getToken()}` } })
  if (!r.ok) throw new Error('Failed to load docs')
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
