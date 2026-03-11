import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import Editor from '@monaco-editor/react'
import { getGame, startGame, cancelGame, leaveGame, joinGame, type Game, type GameParticipant } from '@/api/games'
import { getProblem, type Problem } from '@/api/problems'
import { runCode } from '@/api/execute'
import { ApiError } from '@/api/client'
import { errorMessage } from '@/lib/errors'
import { useAuth } from '@/context/AuthContext'
import { Button } from '@/components/ui/button'
import { ProblemDescription } from '@/components/ProblemDescription'

const LANGUAGES = [
  { value: 'python', label: 'Python', monaco: 'python' },
  { value: 'go', label: 'Go', monaco: 'go' },
  { value: 'cpp', label: 'C++', monaco: 'cpp' },
  { value: 'java', label: 'Java', monaco: 'java' },
] as const

type LangValue = (typeof LANGUAGES)[number]['value']

const DEFAULT_CODE: Record<LangValue, string> = {
  python: '\n',
  go: 'package main\n\nimport "fmt"\n\nfunc main() {\n\t\n}\n',
  cpp: '#include <bits/stdc++.h>\nusing namespace std;\n\nint main() {\n\t\n\treturn 0;\n}\n',
  java: 'public class Main {\n    public static void main(String[] args) {\n        \n    }\n}\n',
}


interface SubmissionResult {
  accepted: boolean
  stdout: string
  stderr: string
  failed_test?: number
  user_id: string
}

interface GameFinished {
  winner_id: string
  code?: string
  language?: string
}

interface PlayerAdvanced {
  user_id: string
  problem_id: string
  problem_index: number
  progress: Record<string, number>
}

interface PlayerState {
  problem_id: string
  problem_index: number
  progress: Record<string, number>
}

const participantLabel = (p: GameParticipant) => p.name ?? p.id.slice(0, 8)

export function GamePage() {
  const { id } = useParams<{ id: string }>()
  const gameId = Number(id)
  const { token, userId } = useAuth()
  const navigate = useNavigate()

  const [game, setGame] = useState<Game | null>(null)
  const [problem, setProblem] = useState<Problem | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')

  const [language, setLanguage] = useState<LangValue>('python')
  const languageRef = useRef<LangValue>('python')
  const storageKey = `bb_code_${gameId}`
  const [codePerLang, setCodePerLang] = useState<Record<LangValue, string>>(() => {
    try {
      const saved = localStorage.getItem(storageKey)
      if (saved) return { ...DEFAULT_CODE, ...JSON.parse(saved) }
    } catch {}
    return { ...DEFAULT_CODE }
  })

  const code = codePerLang[language]
  const setCode = (val: string) => setCodePerLang((prev) => {
    const next = { ...prev, [language]: val }
    try { localStorage.setItem(storageKey, JSON.stringify(next)) } catch {}
    return next
  })
  const [stdin, setStdin] = useState('')

  const [running, setRunning] = useState(false)
  const [runOutput, setRunOutput] = useState<{ stdout: string; stderr: string } | null>(null)

  const [submitting, setSubmitting] = useState(false)
  const [submissionResult, setSubmissionResult] = useState<SubmissionResult | null>(null)
  const [winner, setWinner] = useState<GameFinished | null>(null)
  const [playerProgress, setPlayerProgress] = useState<Record<string, number>>({})
  const [solvedTransition, setSolvedTransition] = useState(false)
  const [playerSolutions, setPlayerSolutions] = useState<Record<string, { code: string; language: LangValue }>>({})
  const [viewingUserId, setViewingUserId] = useState<string | null>(null)

  const wsRef = useRef<WebSocket | null>(null)
  const userIdRef = useRef<string | null>(userId)

  useEffect(() => {
    languageRef.current = language
  }, [language])

  useEffect(() => {
    userIdRef.current = userId
  }, [userId])

  const fetchGame = useCallback(async () => {
    try {
      const res = await getGame(gameId)
      setGame(res.game)
      const pRes = await getProblem(res.game.problem_ids[0])
      setProblem((prev) => {
        if (prev !== null && prev.id !== pRes.problem.id) {
          const fresh = { ...DEFAULT_CODE }
          try { localStorage.removeItem(storageKey) } catch {}
          setCodePerLang(fresh)
        }
        return pRes.problem
      })
    } catch (err) {
      setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    } finally {
      setLoading(false)
    }
  }, [gameId])

  // Initial load
  useEffect(() => {
    fetchGame()
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Poll while pending so we catch participant joins and game start
  useEffect(() => {
    if (game?.status !== 'pending') return
    const timer = setInterval(fetchGame, 2000)
    return () => clearInterval(timer)
  }, [game?.status, fetchGame])

  // Connect WebSocket when game is active, reconnect on unexpected close
  useEffect(() => {
    if (game?.status !== 'active' || !token) return

    let stopped = false
    let retryTimeout: ReturnType<typeof setTimeout> | null = null

    const connect = () => {
      if (stopped) return

      const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
      const ws = new WebSocket(`${proto}//${location.host}/api/games/${gameId}/ws`, [token])
      wsRef.current = ws

      ws.onmessage = (e) => {
        try {
          const msg = JSON.parse(e.data) as { type: string } & SubmissionResult & GameFinished & PlayerAdvanced & PlayerState
          if (msg.type === 'submission_result') {
            setSubmissionResult(msg)
            setSubmitting(false)
          } else if (msg.type === 'player_state') {
            setPlayerProgress(msg.progress ?? {})
            getProblem(msg.problem_id).then((res) => {
              setProblem(res.problem)
            }).catch(() => {})
          } else if (msg.type === 'player_advanced') {
            setPlayerProgress(msg.progress ?? {})
            if (msg.user_id === userIdRef.current) {
              setSolvedTransition(true)
              ;(async () => {
                try {
                  const [, res] = await Promise.all([
                    new Promise<void>((resolve) => setTimeout(resolve, 1000)),
                    getProblem(msg.problem_id),
                  ])
                  setProblem(res.problem)
                  const fresh = { ...DEFAULT_CODE }
                  try { localStorage.removeItem(storageKey) } catch {}
                  setCodePerLang(fresh)
                  setSubmissionResult(null)
                } catch {
                  setActionError('Не удалось загрузить следующую задачу')
                } finally {
                  setSolvedTransition(false)
                }
              })()
            }
          } else if (msg.type === 'game_finished') {
            if (msg.code && msg.language) {
              setPlayerSolutions((prev) => ({
                ...prev,
                [msg.winner_id]: { code: msg.code!, language: msg.language as LangValue },
              }))
              setViewingUserId(msg.winner_id)
            }
            setWinner({ winner_id: msg.winner_id })
            setGame((prev) => (prev ? { ...prev, status: 'finished' } : prev))
            setSubmitting(false)
          }
        } catch {
          // ignore malformed messages
        }
      }

      ws.onerror = () => setActionError('Ошибка WebSocket-соединения')
      ws.onclose = () => {
        wsRef.current = null
        setGame((prev) => {
          if (prev?.status === 'finished') return prev
          if (!stopped) {
            setActionError('Переподключение...')
            retryTimeout = setTimeout(connect, 3000)
          }
          return prev
        })
      }
    }

    connect()

    return () => {
      stopped = true
      if (retryTimeout) clearTimeout(retryTimeout)
      wsRef.current?.close()
    }
  }, [game?.status, token, gameId])

  const handleStart = async () => {
    setActionError('')
    try {
      const res = await startGame(gameId)
      setGame(res.game)
    } catch (err) {
      setActionError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    }
  }

  const handleCancel = async () => {
    setActionError('')
    try {
      await cancelGame(gameId)
      navigate('/games')
    } catch (err) {
      setActionError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    }
  }

  const handleLeave = async () => {
    setActionError('')
    try {
      await leaveGame(gameId)
      navigate('/games')
    } catch (err) {
      setActionError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    }
  }

  const handleJoin = async () => {
    setActionError('')
    try {
      const res = await joinGame(gameId)
      setGame(res.game)
    } catch (err) {
      setActionError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    }
  }

  const handleCopyLink = () => {
    navigator.clipboard.writeText(window.location.href)
  }

  const handleRun = async () => {
    setRunning(true)
    setRunOutput(null)
    setActionError('')
    try {
      const res = await runCode(code, language, stdin)
      setRunOutput({ stdout: res.stdout, stderr: res.stderr })
    } catch (err) {
      setActionError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    } finally {
      setRunning(false)
    }
  }

  const handleSubmit = () => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      setActionError('WebSocket не подключён')
      return
    }
    setSubmitting(true)
    setSubmissionResult(null)
    setActionError('')
    ws.send(JSON.stringify({ type: 'submit', code, language }))
  }

  const handleLangChange = (lang: LangValue) => {
    setLanguage(lang)
  }

  if (loading) return <p className="text-sm text-muted-foreground">Загрузка...</p>
  if (error) return <p className="text-sm text-destructive">{error}</p>
  if (!game || !problem) return null

  const isCreator = userId != null && game.creator_id === userId
  const isParticipant = userId != null && game.participants.some((p) => p.id === userId)
  const isActive = game.status === 'active'
  const isFinished = game.status === 'finished' || game.status === 'cancelled'
  const canStart = isCreator && game.participants.length >= 2
  const totalProblems = game.problem_ids.length
  const myProgress = playerProgress[userId ?? ''] ?? 0

  // Lobby (pending)
  if (game.status === 'pending') {
    return (
      <div className="flex flex-col items-center justify-center min-h-[calc(100vh-80px)] py-8">
        <div className="w-full max-w-md flex flex-col gap-4">
          {/* Back link */}
          <Link
            to="/games"
            className="text-sm text-muted-foreground hover:text-foreground transition-colors self-start"
          >
            ← Игры
          </Link>

          {/* Problem card */}
          <div className="rounded-lg border border-border/60 bg-card/50 p-5">
            <h2 className="text-base font-semibold mb-1">{problem.title}</h2>
            <div className="flex items-center gap-2 flex-wrap">
              <span className="px-2 py-0.5 rounded-full text-xs text-muted-foreground bg-muted/50 border border-border/40">
                {problem.time_limit_ms} мс
              </span>
              <span className="px-2 py-0.5 rounded-full text-xs text-muted-foreground bg-muted/50 border border-border/40">
                {problem.memory_limit_mb} МБ
              </span>
            </div>
          </div>

          {/* Participants card */}
          <div className="rounded-lg border border-border/60 bg-card/50 p-5">
            <p className="text-xs font-medium text-muted-foreground mb-3">
              Участники · {game.participants.length}
            </p>
            <div className="flex flex-col gap-2">
              {game.participants.map((p) => (
                <div key={p.id} className="flex items-center gap-2 text-sm">
                  <span className="w-2 h-2 rounded-full bg-primary/60 flex-shrink-0" />
                  <span className={p.id === userId ? 'text-primary font-medium' : ''}>
                    {participantLabel(p)}
                    {p.id === userId && ' (ты)'}
                    {p.id === game.creator_id && (
                      <span className="ml-1.5 text-xs text-muted-foreground">создатель</span>
                    )}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {actionError && (
            <p className="text-sm text-destructive">{actionError}</p>
          )}

          {/* Actions */}
          {isCreator ? (
            <div className="flex flex-col gap-2">
              <Button onClick={handleStart} disabled={!canStart} className="w-full">
                Начать игру
              </Button>
              {!canStart && (
                <p className="text-xs text-center text-muted-foreground">
                  Нужен хотя бы 1 соперник
                </p>
              )}
              <Button variant="outline" onClick={handleCopyLink} className="w-full">
                Скопировать ссылку
              </Button>
              <Button variant="outline" onClick={handleCancel} className="w-full">
                Отменить игру
              </Button>
            </div>
          ) : isParticipant ? (
            <div className="flex flex-col gap-2">
              <p className="text-sm text-center text-muted-foreground">
                Ждём пока создатель начнёт игру...
              </p>
              <Button variant="outline" onClick={handleLeave} className="w-full">
                Покинуть игру
              </Button>
            </div>
          ) : (
            <Button onClick={handleJoin} className="w-full">
              Вступить в игру
            </Button>
          )}
        </div>
      </div>
    )
  }

  // Game / Finished
  const viewedSolution = viewingUserId !== null ? playerSolutions[viewingUserId] : null
  const editorCode = viewingUserId !== null ? (viewedSolution?.code ?? '') : code
  const editorLanguage = viewedSolution?.language ?? language
  const monacoLang = LANGUAGES.find((l) => l.value === editorLanguage)?.monaco ?? 'python'
  const winnerParticipant = winner
    ? game.participants.find((p) => p.id === winner.winner_id)
    : null

  return (
    <div className="flex flex-col gap-4" style={{ height: 'calc(100vh - 80px)' }}>
      {/* Header */}
      <div className="flex items-center justify-between flex-shrink-0">
        <div className="flex items-center gap-3">
          <Link
            to="/games"
            className="text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            ← Игры
          </Link>
          <span className="text-muted-foreground/40">|</span>
          <span className="text-sm font-medium">{problem.title}</span>
          <span className="text-muted-foreground/40">|</span>
          <span className="text-xs text-muted-foreground">
            {myProgress + 1}/{totalProblems}
          </span>
        </div>
        <div className="flex items-center gap-3">
          {isActive && (
            <div className="flex items-center gap-1.5 text-sm text-green-400">
              <span className="w-2 h-2 rounded-full bg-green-400 animate-pulse" />
              Идёт
            </div>
          )}
          {isFinished && (
            <span className="text-sm text-muted-foreground">Завершена</span>
          )}
        </div>
      </div>

      {actionError && (
        <p className="text-sm text-destructive flex-shrink-0">{actionError}</p>
      )}

      {/* Main layout */}
      <div className="flex gap-4 flex-1 min-h-0">
        {/* Left: problem + participants */}
        <div className="w-2/5 flex flex-col gap-3 overflow-y-auto">
          <div className="rounded-lg border border-border bg-card p-5 flex-shrink-0 shadow-sm">
            <h2 className="text-base font-semibold mb-3">{problem.title}</h2>
            <div className="flex items-center gap-2 flex-wrap mb-4">
              <span className="px-2 py-0.5 rounded-full text-xs text-muted-foreground bg-muted border border-border">
                {problem.time_limit_ms} мс
              </span>
              <span className="px-2 py-0.5 rounded-full text-xs text-muted-foreground bg-muted border border-border">
                {problem.memory_limit_mb} МБ
              </span>
            </div>
            <ProblemDescription content={problem.description} />
          </div>

          <div className="rounded-lg border border-border bg-card p-4 flex-shrink-0 shadow-sm">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-3">Участники</p>
            <div className="flex flex-col divide-y divide-border">
              {game.participants.map((p) => (
                <div
                  key={p.id}
                  className={`flex items-center gap-2 text-sm py-2 first:pt-0 last:pb-0 rounded-md px-1 -mx-1 transition-colors ${
                    isFinished
                      ? `cursor-pointer hover:bg-muted/40 ${viewingUserId === p.id ? 'bg-muted/40 ring-1 ring-border/60' : ''}`
                      : ''
                  }`}
                  onClick={() => {
                    if (isFinished) setViewingUserId((prev) => (prev === p.id ? null : p.id))
                  }}
                >
                  <span className="w-2 h-2 rounded-full bg-primary/60 flex-shrink-0" />
                  <span className={p.id === userId ? 'text-primary font-medium' : ''}>
                    {participantLabel(p)}
                    {p.id === userId && ' (ты)'}
                  </span>
                  <div className="ml-auto flex items-center gap-2">
                    <span className="text-xs font-mono text-muted-foreground">
                      {playerProgress[p.id] ?? 0}/{totalProblems}
                    </span>
                    {game.winner_id === p.id && (
                      <span className="text-xs text-yellow-400 font-medium">Победитель</span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Right: editor + controls */}
        <div className="flex-1 flex flex-col gap-2 min-h-0">
          {/* Toolbar */}
          <div className="flex items-center gap-2 flex-shrink-0">
            <select
              value={editorLanguage}
              onChange={(e) => handleLangChange(e.target.value as LangValue)}
              disabled={isFinished}
              className="rounded-md border border-input bg-background px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"
            >
              {LANGUAGES.map((l) => (
                <option key={l.value} value={l.value} className="bg-background">
                  {l.label}
                </option>
              ))}
            </select>
            {isFinished && (
              <span className="text-xs text-muted-foreground bg-muted/40 border border-border/40 px-2 py-1 rounded-md">
                {viewingUserId
                  ? `Решение: ${participantLabel(game.participants.find((p) => p.id === viewingUserId) ?? { id: viewingUserId, name: null })}`
                  : 'Завершена — только чтение'}
              </span>
            )}
            <div className="ml-auto flex items-center gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={handleRun}
                disabled={!isActive || running}
              >
                {running ? 'Запуск...' : 'Запустить'}
              </Button>
              <Button
                size="sm"
                onClick={handleSubmit}
                disabled={!isActive || submitting}
              >
                {submitting ? 'Проверяем...' : 'Отправить решение'}
              </Button>
            </div>
          </div>

          {/* Monaco editor */}
          <div className="flex-1 rounded-lg border border-border overflow-hidden min-h-0 shadow-sm">
            <Editor
              height="100%"
              language={monacoLang}
              value={editorCode}
              onChange={(val) => { if (!isFinished && viewingUserId === null) setCode(val ?? '') }}
              theme="vs-dark"
              options={{
                minimap: { enabled: false },
                fontSize: 14,
                lineNumbers: 'on',
                scrollBeyondLastLine: false,
                readOnly: isFinished || viewingUserId !== null,
                readOnlyMessage: { value: '' },
                padding: { top: 12 },
              }}
            />
          </div>

          {/* Stdin */}
          <textarea
            placeholder="stdin для кнопки «Запустить»"
            value={stdin}
            onChange={(e) => setStdin(e.target.value)}
            disabled={!isActive}
            rows={2}
            className="flex-shrink-0 w-full rounded-md border border-input bg-background px-3 py-2 text-xs font-mono text-foreground resize-none focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50 placeholder:text-muted-foreground"
          />

          {/* Run output */}
          {runOutput && (
            <div className="flex-shrink-0 rounded-md border border-border bg-muted/30 p-3 text-xs font-mono max-h-32 overflow-y-auto">
              {runOutput.stdout && (
                <pre className="whitespace-pre-wrap">{runOutput.stdout}</pre>
              )}
              {runOutput.stderr && (
                <pre className="whitespace-pre-wrap text-destructive">{runOutput.stderr}</pre>
              )}
              {!runOutput.stdout && !runOutput.stderr && (
                <span className="text-muted-foreground">(нет вывода)</span>
              )}
            </div>
          )}

          {/* Submission result */}
          {submissionResult && (
            <div
              className={`flex-shrink-0 rounded-md border px-4 py-2.5 text-sm font-medium ${
                submissionResult.accepted
                  ? 'border-green-500/30 bg-green-500/10 text-green-400'
                  : 'border-red-500/30 bg-red-500/10 text-red-400'
              }`}
            >
              {submissionResult.accepted ? (
                'Принято!'
              ) : (
                <>
                  Неверно
                  {submissionResult.failed_test != null &&
                    ` — тест #${submissionResult.failed_test + 1}`}
                  {submissionResult.stderr && (
                    <pre className="mt-1 text-xs font-mono opacity-80 whitespace-pre-wrap">
                      {submissionResult.stderr}
                    </pre>
                  )}
                </>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Solved transition overlay */}
      {solvedTransition && (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className="animate-in fade-in zoom-in-95 duration-300 flex flex-col items-center gap-4">
            <div className="w-20 h-20 rounded-full bg-green-500/20 border-2 border-green-500/60 flex items-center justify-center">
              <svg className="w-10 h-10 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
              </svg>
            </div>
            <p className="text-lg font-semibold text-green-400">Задача решена!</p>
            <p className="text-sm text-muted-foreground">Загружаем следующую...</p>
          </div>
        </div>
      )}

      {/* Winner modal */}
      {winner && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-card border border-border/60 rounded-xl p-8 w-full max-w-sm shadow-2xl shadow-black/40 mx-4 text-center">
            <h2 className="text-xl font-semibold mb-1">
              {winner.winner_id === userId ? 'Победа!' : 'Игра завершена'}
            </h2>
            <p className="text-sm text-muted-foreground mb-5">
              {winner.winner_id === userId
                ? 'Ты решил все задачи первым!'
                : `Победил ${winnerParticipant ? participantLabel(winnerParticipant) : winner.winner_id.slice(0, 8)}`}
            </p>
            <Button className="w-full" onClick={() => setWinner(null)}>
              Закрыть
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
