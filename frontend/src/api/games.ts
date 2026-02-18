import { apiFetch } from './client'

export interface GameParticipant {
  id: string
  name?: string | null
}

export interface Game {
  id: number
  problem_ids: string[]
  current_problem_index: number
  creator_id: string
  status: 'pending' | 'active' | 'finished' | 'cancelled'
  participants: GameParticipant[]
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

export const createGame = (problemIds: string[]) =>
  apiFetch<{ game: Game }>('/games', {
    method: 'POST',
    body: JSON.stringify({
      problem_ids: problemIds.slice(0, 20),
    }),
  })

export const joinGame = (id: number) =>
  apiFetch<{ game: Game }>(`/games/${id}/join`, { method: 'POST' })

export const leaveGame = (id: number) =>
  apiFetch<{ game: Game }>(`/games/${id}/leave`, { method: 'POST' })

export const startGame = (id: number) =>
  apiFetch<{ game: Game }>(`/games/${id}/start`, { method: 'POST' })

export const cancelGame = (id: number) =>
  apiFetch<{ game: Game }>(`/games/${id}/cancel`, { method: 'POST' })
