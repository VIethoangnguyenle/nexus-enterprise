import { useAuthStore } from '../stores/auth.store'

const API_BASE = '/api'

/** Authenticated fetch wrapper. Injects Bearer token and handles JSON. */
export async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = useAuthStore.getState().token
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(options.headers || {}),
  }

  const res = await fetch(`${API_BASE}${path}`, { ...options, headers })

  if (res.status === 401) {
    useAuthStore.getState().logout()
    throw new ApiError('Unauthorized', 401)
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError(body.error || body.message || res.statusText, res.status, body)
  }

  if (res.status === 204) return {} as T
  return res.json()
}

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public body?: unknown,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}
