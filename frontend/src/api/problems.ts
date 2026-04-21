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

export interface MyProblem {
  id: string
  title: string
  visibility: 'public' | 'unlisted' | 'private'
  status: 'published' | 'archived'
  version?: number | null
}

export interface UploadResult {
  slug: string
  title: string
  version: number
}

export type UploadResponse = UploadResult | { problems: UploadResult[] }

export const listProblems = (q = '', limit = 50, offset = 0) =>
  apiFetch<{ problems: Problem[]; total: number }>(
    `/problems?q=${encodeURIComponent(q)}&limit=${limit}&offset=${offset}`,
  )

export const listMyProblems = (q = '') =>
  apiFetch<{ problems: MyProblem[] }>(`/problems/mine?q=${encodeURIComponent(q)}`)

export const getProblem = (id: string) =>
  apiFetch<{ problem: Problem }>(`/problems/${id}`)

export const patchProblem = (id: string, visibility: string) =>
  apiFetch<{ slug: string; visibility: string }>(`/problems/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ visibility }),
  })

export const uploadProblem = (file: File, visibility: string): Promise<UploadResponse> => {
  const form = new FormData()
  form.append('file', file)
  form.append('visibility', visibility)
  return apiFetch<UploadResponse>('/problems', { method: 'POST', body: form })
}

export const uploadProblemVersion = (slug: string, file: File): Promise<UploadResult> => {
  const form = new FormData()
  form.append('file', file)
  return apiFetch<UploadResult>(`/problems/${slug}/versions`, { method: 'POST', body: form })
}
