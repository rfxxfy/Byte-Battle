import { apiFetch } from './client'

export interface TokenResponse {
  token: string
  expires_at: string
}

export interface MeResponse {
  user_id: string
}

export const enter = (email: string) =>
  apiFetch<{ status: string }>('/auth/enter', {
    method: 'POST',
    body: JSON.stringify({ email }),
  })

export const confirm = (email: string, code: string) =>
  apiFetch<TokenResponse>('/auth/confirm', {
    method: 'POST',
    body: JSON.stringify({ email, code }),
  })

export const me = () => apiFetch<MeResponse>('/auth/me')

export const logout = () =>
  apiFetch<{ status: string }>('/auth/logout', { method: 'POST' })
