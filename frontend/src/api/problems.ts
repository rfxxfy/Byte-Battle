import { apiFetch } from './client'

export interface Problem {
  id: string
  title: string
  description: string
  difficulty: 'easy' | 'medium' | 'hard'
  time_limit_ms: number
  memory_limit_mb: number
  test_count?: number
}

export const listProblems = () =>
  apiFetch<{ problems: Problem[] }>('/problems')

export const getProblem = (id: string) =>
  apiFetch<{ problem: Problem }>(`/problems/${id}`)
