import { apiFetch } from './client'

export interface TokenResponse {
  token: string
  expires_at: string
  name?: string
}

export interface MeResponse {
  user_id: string
  name?: string
}

export const enter = (email: string) =>
  apiFetch<{ status: string }>('/api/auth/enter', {
    method: 'POST',
    body: JSON.stringify({ email }),
  })

export const confirm = (email: string, code: string) =>
  apiFetch<TokenResponse>('/api/auth/confirm', {
    method: 'POST',
    body: JSON.stringify({ email, code }),
  })

export const updateMe = (name: string) =>
  apiFetch<MeResponse>('/api/auth/me', {
    method: 'PATCH',
    body: JSON.stringify({ name }),
  })

export const me = () => apiFetch<MeResponse>('/api/auth/me')

export const logout = () =>
  apiFetch<{ status: string }>('/api/auth/logout', { method: 'POST' })
