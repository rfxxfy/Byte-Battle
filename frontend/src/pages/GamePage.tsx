import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import Editor from '@monaco-editor/react'
import { getGame, timeoutGame, type Game, type GameParticipant } from '@/api/games'
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
  python: 'print("Hello, ByteBattle!")\n',
  go: 'package main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Println("Hello, ByteBattle!")\n}\n',
  cpp: '#include <bits/stdc++.h>\nusing namespace std;\n\nint main() {\n\tcout << "Hello, ByteBattle!" << endl;\n\treturn 0;\n}\n',
  java: 'public class Main {\n    public static void main(String[] args) {\n        System.out.println("Hello, ByteBattle!");\n    }\n}\n',
}

interface SubmissionResult {
  accepted: boolean
  stdout: string
  stderr: string
  failed_test?: number
  user_id: string
}

interface GameFinished {
  winner_id: string | null
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

function formatSeconds(totalSeconds: number): string {
  const m = Math.floor(totalSeconds / 60)
  const s = totalSeconds % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

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
    } catch { /* ignore */ }
    return { ...DEFAULT_CODE }
  })

  const code = codePerLang[language]
  const setCode = (val: string) => setCodePerLang((prev) => {
    const next = { ...prev, [language]: val }
    try { localStorage.setItem(storageKey, JSON.stringify(next)) } catch { /* ignore */ }
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
  const [notification, setNotification] = useState<string | null>(null)

  const [soloTimeDisplay, setSoloTimeDisplay] = useState<string | null>(null)
  const [soloRemainingSeconds, setSoloRemainingSeconds] = useState<number | null>(null)
  const [timedOut, setTimedOut] = useState(false)

  const wsRef = useRef<WebSocket | null>(null)
  const userIdRef = useRef<string | null>(userId)
  const gameRef = useRef<Game | null>(null)
  const notifTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const timeoutCalledRef = useRef(false)

  useEffect(() => { languageRef.current = language }, [language])
  useEffect(() => { userIdRef.current = userId }, [userId])
  useEffect(() => { gameRef.current = game }, [game])

  const fetchGame = useCallback(async () => {
    try {
      const res = await getGame(gameId)
      const g = res.game
      if (g.status === 'pending') {
        if (g.is_solo) {
          navigate(`/games/${gameId}/lobby`, { replace: true })
        } else {
          navigate(`/games/join/${g.invite_token}`, { replace: true })
        }
        return
      }
      if (g.status === 'finished') {
        navigate(`/games/${gameId}/results`, { replace: true })
        return
      }
      setGame(g)
      const pRes = await getProblem(g.problem_ids[0])
      setProblem((prev) => {
        if (prev !== null && prev.id !== pRes.problem.id) {
          try { localStorage.removeItem(storageKey) } catch { /* ignore */ }
          setCodePerLang({ ...DEFAULT_CODE })
        }
        return pRes.problem
      })
    } catch (err) {
      setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    } finally {
      setLoading(false)
    }
  }, [gameId, storageKey, navigate])

  useEffect(() => {
    fetchGame()
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Solo timer (countdown or stopwatch)
  useEffect(() => {
    if (!game?.is_solo || game.status !== 'active') return

    if (!game.started_at || game.time_limit_minutes == null) return
    const startedAt = new Date(game.started_at).getTime()
    const timeLimitSeconds = game.time_limit_minutes * 60

    const tick = () => {
      const elapsed = Math.floor((Date.now() - startedAt) / 1000)
      const remaining = Math.max(0, timeLimitSeconds - elapsed)
      setSoloTimeDisplay(formatSeconds(remaining))
      setSoloRemainingSeconds(remaining)
      if (remaining === 0 && !timeoutCalledRef.current) {
        timeoutCalledRef.current = true
        setTimedOut(true)
        timeoutGame(gameId).catch(() => {})
        setTimeout(() => navigate(`/games/${gameId}/results`, { replace: true }), 2000)
      }
    }

    tick()
    const interval = setInterval(tick, 1000)
    return () => clearInterval(interval)
  }, [game?.is_solo, game?.status, game?.started_at, game?.time_limit_minutes, gameId, navigate])

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
            getProblem(msg.problem_id).then((res) => setProblem(res.problem)).catch(() => {})
          } else if (msg.type === 'player_advanced') {
            setPlayerProgress(msg.progress ?? {})
            if (msg.user_id !== userIdRef.current) {
              const p = gameRef.current?.participants.find((p) => p.id === msg.user_id)
              const label = p ? (p.name ?? p.id.slice(0, 8)) : msg.user_id.slice(0, 8)
              const text = `${label} решил задачу ${msg.problem_index}`
              if (notifTimerRef.current) clearTimeout(notifTimerRef.current)
              setNotification(text)
              notifTimerRef.current = setTimeout(() => setNotification(null), 3500)
            }
            if (msg.user_id === userIdRef.current) {
              setSolvedTransition(true)
              ;(async () => {
                try {
                  const [, res] = await Promise.all([
                    new Promise<void>((resolve) => setTimeout(resolve, 1000)),
                    getProblem(msg.problem_id),
                  ])
                  setProblem(res.problem)
                  try { localStorage.removeItem(storageKey) } catch { /* ignore */ }
                  setCodePerLang({ ...DEFAULT_CODE })
                  setSubmissionResult(null)
                } catch {
                  setActionError('Не удалось загрузить следующую задачу')
                } finally {
                  setSolvedTransition(false)
                }
              })()
            }
          } else if (msg.type === 'game_finished') {
            // For timeout, we already handle navigation via the timer effect
            if (!timeoutCalledRef.current) {
              setWinner({ winner_id: msg.winner_id })
            }
            setGame((prev) => (prev ? { ...prev, status: 'finished' } : prev))
            setSubmitting(false)
          }
        } catch { /* ignore malformed */ }
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
  }, [game?.status, token, gameId, storageKey])

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

  if (loading) return <p className="text-sm text-muted-foreground">Загрузка...</p>
  if (error) return <p className="text-sm text-destructive">{error}</p>
  if (!game || !problem) return null

  const isActive = game.status === 'active'
  const isFinished = game.status === 'finished' || game.status === 'cancelled'
  const blocked = isFinished || timedOut
  const totalProblems = game.problem_ids.length
  const myProgress = playerProgress[userId ?? ''] ?? 0
  const monacoLang = LANGUAGES.find((l) => l.value === language)?.monaco ?? 'python'
  const winnerParticipant = winner?.winner_id
    ? game.participants.find((p) => p.id === winner.winner_id)
    : null

  return (
    <div className="flex flex-col gap-4 flex-1 min-h-0 py-4">
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
          {soloTimeDisplay && isActive && (
            <div className={`flex items-center gap-1.5 text-sm font-mono tabular-nums ${
              game.time_limit_minutes != null && !timedOut
                ? soloRemainingSeconds !== null && soloRemainingSeconds < 300 ? 'text-red-400' : 'text-muted-foreground'
                : 'text-muted-foreground'
            }`}>
              {game.time_limit_minutes != null ? '⏱' : '⏱'} {soloTimeDisplay}
            </div>
          )}
          {isActive && !soloTimeDisplay && (
            <div className="flex items-center gap-1.5 text-sm text-green-400">
              <span className="w-2 h-2 rounded-full bg-green-400 animate-pulse" />
              Идёт
            </div>
          )}
          {isFinished && (
            <div className="flex items-center gap-3">
              <span className="text-sm font-medium text-amber-400/90">Завершена</span>
              <Button
                size="sm"
                variant="outline"
                className="border-amber-400/40 text-amber-400/90 hover:bg-amber-400/10 hover:text-amber-400"
                onClick={() => navigate(`/games/${gameId}/results`)}
              >
                {game.is_solo ? 'Смотреть результаты' : 'Смотреть решения участников'}
              </Button>
            </div>
          )}
        </div>
      </div>

      {actionError && (
        <p className="text-sm text-destructive flex-shrink-0">{actionError}</p>
      )}

      <div className="flex gap-4 flex-1 min-h-0">
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

          {!game.is_solo && (
            <div className="rounded-lg border border-border bg-card p-4 flex-shrink-0 shadow-sm">
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-3">Участники</p>
              <div className="flex flex-col divide-y divide-border">
                {game.participants.map((p) => (
                  <div
                    key={p.id}
                    className="flex items-center gap-2 text-sm py-2 first:pt-0 last:pb-0 rounded-md px-1 -mx-1"
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
          )}
        </div>

        <div className="flex-1 flex flex-col min-h-0 rounded-lg border border-border overflow-hidden shadow-sm">
          <div className="flex items-center gap-2 px-3 py-2 border-b border-border/60 bg-card flex-shrink-0">
            <select
              value={language}
              onChange={(e) => setLanguage(e.target.value as LangValue)}
              disabled={blocked}
              className="rounded-md border border-input bg-background px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"
            >
              {LANGUAGES.map((l) => (
                <option key={l.value} value={l.value} className="bg-background">
                  {l.label}
                </option>
              ))}
            </select>
            {blocked && (
              <span className="text-xs text-muted-foreground bg-muted/40 border border-border/40 px-2 py-1 rounded-md">
                Только чтение
              </span>
            )}
            <div className="ml-auto flex items-center gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={handleRun}
                disabled={!isActive || running || timedOut}
              >
                {running ? 'Запуск...' : (
                  <span className="flex items-center gap-1.5">
                    <svg viewBox="0 0 16 16" fill="currentColor" className="w-3.5 h-3.5 text-green-400">
                      <path d="M3 2.5l10 5.5-10 5.5V2.5z" />
                    </svg>
                    Запустить
                  </span>
                )}
              </Button>
              <Button
                size="sm"
                onClick={handleSubmit}
                disabled={!isActive || submitting || timedOut}
              >
                {submitting ? 'Проверяем...' : 'Отправить решение'}
              </Button>
            </div>
          </div>

          <div className="flex-1 min-h-0 relative">
            <div className="absolute inset-0">
              <Editor
                height="100%"
                language={monacoLang}
                value={code}
                onChange={(val) => { if (!blocked) setCode(val ?? '') }}
                theme="vs-dark"
                options={{
                  minimap: { enabled: false },
                  fontSize: 14,
                  lineNumbers: 'on',
                  scrollBeyondLastLine: false,
                  readOnly: blocked,
                  domReadOnly: blocked,
                  mouseStyle: blocked ? 'default' : 'text',
                  renderLineHighlight: blocked ? 'none' : 'line',
                  padding: { top: 12 },
                }}
              />
            </div>
          </div>

          <div className="flex-shrink-0 border-t border-border/60">
            <textarea
              placeholder="stdin для кнопки «Запустить»"
              value={stdin}
              onChange={(e) => setStdin(e.target.value)}
              disabled={!isActive || timedOut}
              rows={2}
              className="w-full border-b border-border/60 bg-background px-3 py-2 text-xs font-mono text-foreground resize-none focus:outline-none disabled:opacity-50 placeholder:text-muted-foreground block"
            />
            <div className="h-20 overflow-y-auto px-3 py-2 text-xs font-mono bg-background">
              {submissionResult ? (
                <div className={submissionResult.accepted ? 'text-green-400' : 'text-red-400'}>
                  {submissionResult.accepted ? (
                    'Принято!'
                  ) : (
                    <>
                      Неверно
                      {submissionResult.failed_test != null &&
                        ` — тест #${submissionResult.failed_test + 1}`}
                      {submissionResult.stderr && (
                        <pre className="mt-1 opacity-80 whitespace-pre-wrap">
                          {submissionResult.stderr}
                        </pre>
                      )}
                    </>
                  )}
                </div>
              ) : runOutput ? (
                <>
                  {runOutput.stdout && (
                    <pre className="whitespace-pre-wrap">{runOutput.stdout}</pre>
                  )}
                  {runOutput.stderr && (
                    <pre className="whitespace-pre-wrap text-destructive">{runOutput.stderr}</pre>
                  )}
                  {!runOutput.stdout && !runOutput.stderr && (
                    <span className="text-muted-foreground">(нет вывода)</span>
                  )}
                </>
              ) : (
                <span className="text-muted-foreground/40">Вывод появится здесь</span>
              )}
            </div>
          </div>
        </div>
      </div>

      {notification && (
        <div className="fixed bottom-6 right-6 z-50 px-4 py-3 rounded-lg border border-border bg-card shadow-lg text-sm animate-in fade-in slide-in-from-bottom-2 duration-200">
          <span className="text-muted-foreground">{notification}</span>
        </div>
      )}

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

      {timedOut && !winner && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
          <div className="text-center">
            <p className="text-4xl font-bold text-red-400">Время вышло</p>
            <p className="text-sm text-muted-foreground mt-3">Переходим к результатам...</p>
          </div>
        </div>
      )}

      {winner && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="bg-card border border-border/60 rounded-xl p-8 w-full max-w-sm shadow-2xl shadow-black/40 mx-4 text-center">
            {game.is_solo ? (
              <>
                <h2 className="text-xl font-semibold mb-1">
                  {winner.winner_id ? 'Готово!' : 'Время вышло'}
                </h2>
                <p className="text-sm text-muted-foreground mb-5">
                  {winner.winner_id
                    ? 'Все задачи решены!'
                    : `Решено ${myProgress}/${totalProblems} задач`}
                </p>
              </>
            ) : (
              <>
                <h2 className="text-xl font-semibold mb-1">
                  {winner.winner_id === userId ? 'Победа!' : 'Игра завершена'}
                </h2>
                <p className="text-sm text-muted-foreground mb-5">
                  {winner.winner_id === userId
                    ? 'Ты решил все задачи первым!'
                    : `Победил ${winnerParticipant ? participantLabel(winnerParticipant) : winner.winner_id?.slice(0, 8)}`}
                </p>
              </>
            )}
            <Button className="w-full" onClick={() => navigate(`/games/${gameId}/results`)}>
              {game.is_solo ? 'Смотреть результаты' : 'Смотреть решения участников'}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
