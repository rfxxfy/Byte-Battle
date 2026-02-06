import { apiFetch } from './client'

export interface Game {
  id: number
  problem_id: string
  status: 'pending' | 'active' | 'finished' | 'cancelled'
  participant_ids: string[]
  winner_id?: string | null
  created_at: string
  updated_at: string
}

export const listGames = (limit = 10, offset = 0) =>
  apiFetch<{ games: Game[]; total: number }>(
    `/games?limit=${limit}&offset=${offset}`,
  )

export const getGame = (id: number) =>
  apiFetch<{ game: Game }>(`/games/${id}`)

export const createGame = (problemId: string) =>
  apiFetch<{ game: Game }>('/games', {
    method: 'POST',
    body: JSON.stringify({ problem_id: problemId }),
  })

export const joinGame = (id: number) =>
  apiFetch<{ game: Game }>(`/games/${id}/join`, { method: 'POST' })

export const startGame = (id: number) =>
  apiFetch<{ game: Game }>(`/games/${id}/start`, { method: 'POST' })
