export type AuthResponse = { access_token: string; refresh_token: string }
export type User = { id: string; email: string }
export type BucketStatus = { limit: number; remaining: number; reset: number }
export type Limits = { ip: BucketStatus; user: BucketStatus }

const accessKey = 'api-gateway.access-token'
const refreshKey = 'api-gateway.refresh-token'

export const tokens = {
  get: () => ({ access: localStorage.getItem(accessKey), refresh: localStorage.getItem(refreshKey) }),
  set: (response: AuthResponse) => {
    localStorage.setItem(accessKey, response.access_token)
    localStorage.setItem(refreshKey, response.refresh_token)
  },
  clear: () => {
    localStorage.removeItem(accessKey)
    localStorage.removeItem(refreshKey)
  },
}

async function request<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
  const { access, refresh } = tokens.get()
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  if (init.body) headers.set('Content-Type', 'application/json')
  if (access) headers.set('Authorization', `Bearer ${access}`)

  const response = await fetch(path, { ...init, headers })
  if (response.status === 401 && retry && refresh && path !== '/api/v1/auth/refresh') {
    const refreshed = await fetch('/api/v1/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({ refresh_token: refresh }),
    })
    if (refreshed.ok) {
      tokens.set((await refreshed.json()) as AuthResponse)
      return request<T>(path, init, false)
    }
    tokens.clear()
  }

  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as { error?: string }
    throw new Error(body.error ?? `Request failed (${response.status})`)
  }
  return (await response.json()) as T
}

export const api = {
  login: (email: string, password: string) => request<AuthResponse>('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }),
  register: (email: string, password: string) => request<User>('/api/v1/auth/register', { method: 'POST', body: JSON.stringify({ email, password }) }),
  me: () => request<User>('/api/v1/users/me'),
  limits: () => request<Limits>('/api/v1/limits/status'),
  health: () => request<{ status: string }>('/healthz'),
  ready: () => request<{ status: string }>('/readyz'),
}

export function decodeClaims(token: string | null): Record<string, unknown> {
  if (!token) return {}
  try {
    const payload = token.split('.')[1]
    return JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/'))) as Record<string, unknown>
  } catch {
    return {}
  }
}
