import { apiFetch } from './client'

export interface ExecuteResponse {
  stdout: string
  stderr: string
  exit_code: number
  time_used_ms: number
}

export const runCode = (code: string, language: string, input: string) =>
  apiFetch<ExecuteResponse>('/execute', {
    method: 'POST',
    body: JSON.stringify({ code, language, input }),
  })
