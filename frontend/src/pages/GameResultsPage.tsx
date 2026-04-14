import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import Editor from '@monaco-editor/react'
import { getGame, getGameSolutions, type Game, type GameSolution } from '@/api/games'
import { ApiError } from '@/api/client'
import { errorMessage } from '@/lib/errors'

const MONACO_LANG: Record<string, string> = {
  python: 'python',
  go: 'go',
  cpp: 'cpp',
  java: 'java',
}

function participantLabel(solution: GameSolution): string {
  return solution.name ?? solution.user_id.slice(0, 8)
}

export function GameResultsPage() {
  const { id } = useParams<{ id: string }>()
  const gameId = Number(id)

  const [game, setGame] = useState<Game | null>(null)
  const [solutions, setSolutions] = useState<GameSolution[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [selectedProblemId, setSelectedProblemId] = useState<string | null>(null)
  const [manualUserId, setManualUserId] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([getGame(gameId), getGameSolutions(gameId)])
      .then(([gameRes, solRes]) => {
        setGame(gameRes.game)
        setSolutions(solRes.solutions)
        if (gameRes.game.problem_ids.length > 0) {
          setSelectedProblemId(gameRes.game.problem_ids[0])
        }
      })
      .catch((err) => {
        setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : 'Не удалось загрузить результаты')
      })
      .finally(() => setLoading(false))
  }, [gameId])

  // Auto-select first solver; null means "auto" (manual override stored in manualUserId)
  const effectiveUserId =
    manualUserId ?? solutions.find((s) => s.problem_id === selectedProblemId)?.user_id ?? null

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64 text-muted-foreground text-sm">
        Загрузка...
      </div>
    )
  }

  if (error || !game) {
    return (
      <div className="flex flex-col items-center justify-center h-64 gap-3">
        <p className="text-sm text-muted-foreground">{error ?? 'Игра не найдена'}</p>
        <Link to="/games" className="text-sm text-primary hover:underline">← К играм</Link>
      </div>
    )
  }

  const isSolo = game.is_solo
  const solvedCount = new Set(solutions.map((s) => s.problem_id)).size
  const totalCount = game.problem_ids.length

  const problemSolvers = (problemId: string) =>
    solutions.filter((s) => s.problem_id === problemId)

  const activeSolution = solutions.find(
    (s) => s.problem_id === selectedProblemId && s.user_id === effectiveUserId,
  ) ?? null

  return (
    <div className="flex flex-col gap-6 p-6 max-w-6xl mx-auto w-full">
      <div className="flex items-center gap-3">
        <Link to="/games" className="text-sm text-muted-foreground hover:text-foreground transition-colors">
          ← Игры
        </Link>
        <span className="text-muted-foreground/40">/</span>
        <span className="text-sm font-medium">
          {isSolo ? `Соло #${gameId}` : `Игра #${gameId}`} — {isSolo ? 'Результаты' : 'Решения'}
        </span>
      </div>

      {isSolo && (
        <div className={`rounded-lg border px-5 py-4 flex items-center gap-4 ${
          game.winner_id
            ? 'border-green-500/30 bg-green-500/5'
            : 'border-orange-500/30 bg-orange-500/5'
        }`}>
          <div className={`w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0 ${
            game.winner_id ? 'bg-green-500/20' : 'bg-orange-500/20'
          }`}>
            {game.winner_id ? (
              <svg className="w-4 h-4 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
              </svg>
            ) : (
              <svg className="w-4 h-4 text-orange-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            )}
          </div>
          <div>
            <p className={`text-sm font-medium ${game.winner_id ? 'text-green-400' : 'text-orange-400'}`}>
              {game.winner_id ? 'Завершено' : 'Время вышло'}
            </p>
            <p className="text-xs text-muted-foreground mt-0.5">
              Решено задач: {solvedCount} / {totalCount}
            </p>
          </div>
        </div>
      )}

      <div className="flex gap-6 min-h-0">
        <div className="w-56 flex-shrink-0 flex flex-col gap-2">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-1">Задачи</p>
          {game.problem_ids.map((pid, idx) => {
            const solvers = problemSolvers(pid)
            const isSelected = pid === selectedProblemId
            return (
              <button
                key={pid}
                onClick={() => { setSelectedProblemId(pid); setManualUserId(null) }}
                className={`w-full text-left rounded-lg px-3 py-2.5 text-sm transition-colors border ${
                  isSelected
                    ? 'bg-primary/10 border-primary/30 text-foreground'
                    : 'border-border/50 hover:bg-muted/50 text-muted-foreground hover:text-foreground'
                }`}
              >
                <div className="font-medium">Задача {idx + 1}</div>
                <div className="text-xs mt-0.5 opacity-70">
                  {solvers.length === 0
                    ? 'Не решена'
                    : isSolo
                      ? 'Решена'
                      : `${solvers.length} ${solvers.length === 1 ? 'решение' : solvers.length < 5 ? 'решения' : 'решений'}`}
                </div>
              </button>
            )
          })}
        </div>

        <div className="flex-1 flex flex-col gap-4 min-w-0">
          {selectedProblemId && (
            <>
              {(() => {
                const solvers = problemSolvers(selectedProblemId)
                if (solvers.length === 0) {
                  return (
                    <div className="flex items-center justify-center h-32 rounded-lg border border-border/50 text-sm text-muted-foreground">
                      Задача не решена
                    </div>
                  )
                }
                return (
                  <>
                    {!isSolo && (
                      <div className="flex gap-2 overflow-x-auto pb-1 scrollbar-thin scrollbar-thumb-border scrollbar-track-transparent">
                        {solvers.map((s) => (
                          <button
                            key={s.user_id}
                            onClick={() => setManualUserId(s.user_id)}
                            className={`shrink-0 flex items-center gap-2 max-w-[11rem] px-3 py-1.5 rounded-md text-sm transition-colors border ${
                              s.user_id === effectiveUserId
                                ? 'bg-primary text-primary-foreground border-primary'
                                : 'border-border/60 hover:bg-muted/50 text-muted-foreground hover:text-foreground'
                            }`}
                          >
                            <span className="truncate">{participantLabel(s)}</span>
                            <span className="text-xs opacity-60 shrink-0">{s.language}</span>
                          </button>
                        ))}
                      </div>
                    )}

                    {activeSolution && (
                      <div className="flex-1 rounded-lg border border-border overflow-hidden min-h-[480px]">
                        <Editor
                          height="480px"
                          language={MONACO_LANG[activeSolution.language] ?? 'plaintext'}
                          value={activeSolution.code}
                          theme="vs-dark"
                          options={{
                            readOnly: true,
                            domReadOnly: true,
                            mouseStyle: 'default',
                            minimap: { enabled: false },
                            fontSize: 14,
                            lineNumbers: 'on',
                            scrollBeyondLastLine: false,
                            renderLineHighlight: 'none',
                            padding: { top: 12 },
                          }}
                        />
                      </div>
                    )}
                  </>
                )
              })()}
            </>
          )}
        </div>
      </div>
    </div>
  )
}
