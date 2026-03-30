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
  return solution.name ?? solution.username
}

export function GameResultsPage() {
  const { id } = useParams<{ id: string }>()
  const gameId = Number(id)

  const [game, setGame] = useState<Game | null>(null)
  const [solutions, setSolutions] = useState<GameSolution[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Selected problem + user for code viewer
  const [selectedProblemId, setSelectedProblemId] = useState<string | null>(null)
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null)

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

  // When problem changes, auto-select first solver
  useEffect(() => {
    if (!selectedProblemId) return
    const first = solutions.find((s) => s.problem_id === selectedProblemId)
    setSelectedUserId(first?.user_id ?? null)
  }, [selectedProblemId, solutions])

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

  const problemSolvers = (problemId: string) =>
    solutions.filter((s) => s.problem_id === problemId)

  const activeSolution = solutions.find(
    (s) => s.problem_id === selectedProblemId && s.user_id === selectedUserId,
  ) ?? null

  return (
    <div className="flex flex-col gap-6 p-6 max-w-6xl mx-auto w-full">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Link to={`/games/${gameId}`} className="text-sm text-muted-foreground hover:text-foreground transition-colors">
          ← Игра #{gameId}
        </Link>
        <span className="text-muted-foreground/40">/</span>
        <span className="text-sm font-medium">Решения</span>
      </div>

      <div className="flex gap-6 min-h-0">
        {/* Left: problem list */}
        <div className="w-56 flex-shrink-0 flex flex-col gap-2">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-1">Задачи</p>
          {game.problem_ids.map((pid, idx) => {
            const solvers = problemSolvers(pid)
            const isSelected = pid === selectedProblemId
            return (
              <button
                key={pid}
                onClick={() => setSelectedProblemId(pid)}
                className={`w-full text-left rounded-lg px-3 py-2.5 text-sm transition-colors border ${
                  isSelected
                    ? 'bg-primary/10 border-primary/30 text-foreground'
                    : 'border-border/50 hover:bg-muted/50 text-muted-foreground hover:text-foreground'
                }`}
              >
                <div className="font-medium">Задача {idx + 1}</div>
                <div className="text-xs mt-0.5 opacity-70">
                  {solvers.length === 0
                    ? 'Никто не решил'
                    : `${solvers.length} ${solvers.length === 1 ? 'решение' : solvers.length < 5 ? 'решения' : 'решений'}`}
                </div>
              </button>
            )
          })}
        </div>

        {/* Right: solutions */}
        <div className="flex-1 flex flex-col gap-4 min-w-0">
          {selectedProblemId && (
            <>
              {/* Participant tabs */}
              {(() => {
                const solvers = problemSolvers(selectedProblemId)
                if (solvers.length === 0) {
                  return (
                    <div className="flex items-center justify-center h-32 rounded-lg border border-border/50 text-sm text-muted-foreground">
                      Никто не решил эту задачу
                    </div>
                  )
                }
                return (
                  <>
                    <div className="flex gap-2 overflow-x-auto pb-1 scrollbar-thin scrollbar-thumb-border scrollbar-track-transparent">
                      {solvers.map((s) => (
                        <button
                          key={s.user_id}
                          onClick={() => setSelectedUserId(s.user_id)}
                          className={`shrink-0 px-3 py-1.5 rounded-md text-sm transition-colors border ${
                            s.user_id === selectedUserId
                              ? 'bg-primary text-primary-foreground border-primary'
                              : 'border-border/60 hover:bg-muted/50 text-muted-foreground hover:text-foreground'
                          }`}
                        >
                          {participantLabel(s)}
                          <span className="ml-2 text-xs opacity-60">{s.language}</span>
                        </button>
                      ))}
                    </div>

                    {activeSolution && (
                      <div className="flex-1 rounded-lg border border-border overflow-hidden min-h-[480px]">
                        <Editor
                          height="480px"
                          language={MONACO_LANG[activeSolution.language] ?? 'plaintext'}
                          value={activeSolution.code}
                          theme="vs-dark"
                          options={{
                            readOnly: true,
                            readOnlyMessage: { value: '' },
                            minimap: { enabled: false },
                            fontSize: 14,
                            lineNumbers: 'on',
                            scrollBeyondLastLine: false,
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
