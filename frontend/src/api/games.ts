import { apiFetch } from './client'

export interface GameParticipant {
  id: string
  name?: string | null
}

export interface Game {
  id: number
  problem_ids: string[]
  creator_id: string
  status: 'pending' | 'active' | 'finished' | 'cancelled'
  is_public: boolean
  is_solo: boolean
  invite_token?: string | null
  time_limit_minutes?: number | null
  participants: GameParticipant[]
  winner_id?: string | null
  started_at?: string | null
  created_at: string
  updated_at: string
}

export const listGames = (limit = 10, offset = 0) =>
  apiFetch<{ games: Game[]; total: number }>(
    `/games?limit=${limit}&offset=${offset}`,
  )

export const getGame = (id: number) =>
  apiFetch<{ game: Game }>(`/games/${id}`)

export const createGame = (
  problemIds: string[],
  isPublic = true,
  isSolo = false,
  timeLimitMinutes?: number | null,
) =>
  apiFetch<{ game: Game }>('/games', {
    method: 'POST',
    body: JSON.stringify({
      problem_ids: problemIds.slice(0, 20),
      is_public: isPublic,
      is_solo: isSolo,
      time_limit_minutes: timeLimitMinutes ?? undefined,
    }),
  })

export const timeoutGame = (id: number) =>
  apiFetch<{ game: Game }>(`/games/${id}/timeout`, { method: 'POST' })

export const getGameByToken = (token: string) =>
  apiFetch<{ game: Game }>(`/games/join/${token}`)

export const joinGameByToken = (token: string) =>
  apiFetch<{ game: Game }>(`/games/join/${token}`, { method: 'POST' })

export const leaveGame = (id: number) =>
  apiFetch<{ game: Game }>(`/games/${id}/leave`, { method: 'POST' })

export const startGame = (id: number) =>
  apiFetch<{ game: Game }>(`/games/${id}/start`, { method: 'POST' })

export const cancelGame = (id: number) =>
  apiFetch<{ game: Game }>(`/games/${id}/cancel`, { method: 'POST' })

export interface GameSolution {
  user_id: string
  name?: string | null
  problem_id: string
  code: string
  language: string
  solved_at: string
}

export const getGameSolutions = (id: number) =>
  apiFetch<{ solutions: GameSolution[] }>(`/games/${id}/solutions`)
