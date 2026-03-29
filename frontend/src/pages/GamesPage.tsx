import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { listGames, createGame, joinGame, type Game } from '@/api/games'
import { listProblems, type Problem } from '@/api/problems'
import { ApiError } from '@/api/client'
import { errorMessage } from '@/lib/errors'
import { useAuth } from '@/context/AuthContext'
import { Button } from '@/components/ui/button'

const statusLabel: Record<Game['status'], string> = {
  pending: 'Ожидание',
  active: 'Идёт',
  finished: 'Завершена',
  cancelled: 'Отменена',
}

const statusClass: Record<Game['status'], string> = {
  pending: 'text-yellow-400 bg-yellow-400/10',
  active: 'text-yellow-400 bg-yellow-400/10',
  finished: 'text-red-400 bg-red-400/10',
  cancelled: 'text-red-400 bg-red-400/10',
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function GamesPage() {
  const navigate = useNavigate()
  const { userId } = useAuth()

  const [games, setGames] = useState<Game[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')

  const [modalOpen, setModalOpen] = useState(false)
  const [problems, setProblems] = useState<Problem[]>([])
  const [selectedProblemIds, setSelectedProblemIds] = useState<string[]>([])
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    setLoading(true)
    listGames(50, 0)
      .then((res) => setGames(res.games))
      .catch((err) =>
        setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err)),
      )
      .finally(() => setLoading(false))
  }, [])

  const openModal = async () => {
    setModalOpen(true)
    setActionError('')
    if (problems.length === 0) {
      try {
        const res = await listProblems()
        setProblems(res.problems)
        if (res.problems.length > 0) setSelectedProblemIds([res.problems[0].id])
      } catch (err) {
        setActionError(
          err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err),
        )
      }
    }
  }

  const handleCreateGame = async () => {
    if (selectedProblemIds.length === 0) return
    if (selectedProblemIds.length > 20) {
      setActionError('Можно выбрать не более 20 задач')
      return
    }
    setCreating(true)
    setActionError('')
    try {
      const res = await createGame(selectedProblemIds)
      setModalOpen(false)
      navigate(`/games/${res.game.id}`)
    } catch (err) {
      setActionError(
        err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err),
      )
      setCreating(false)
    }
  }

  const handleJoin = async (id: number) => {
    setActionError('')
    try {
      await joinGame(id)
      navigate(`/games/${id}`)
    } catch (err) {
      setActionError(
        err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err),
      )
    }
  }

  const rowAction = (game: Game) => {
    const isParticipant = userId != null && game.participants.some((p) => p.id === userId)

    if (game.status === 'pending' && !isParticipant) {
      return (
        <Button
          size="sm"
          variant="outline"
          onClick={(e) => {
            e.stopPropagation()
            handleJoin(game.id)
          }}
        >
          Войти
        </Button>
      )
    }

    return null
  }

  const toggleProblem = (problemId: string, checked: boolean) => {
    setSelectedProblemIds((prev) => {
      if (checked) {
        if (prev.includes(problemId) || prev.length >= 20) return prev
        return [...prev, problemId]
      }
      return prev.filter((id) => id !== problemId)
    })
  }

  if (loading) return <p className="text-sm text-muted-foreground">Загрузка...</p>
  if (error) return <p className="text-sm text-destructive">{error}</p>

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Игры</h1>
        <Button onClick={openModal}>Создать игру</Button>
      </div>

      {actionError && <p className="text-sm text-destructive">{actionError}</p>}

      <div className="rounded-lg border border-border/60 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border/60 bg-muted/30">
              <th className="px-4 py-3 text-left font-medium text-muted-foreground w-16">#</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Задачи</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground w-32">Статус</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground w-28">Участники</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground w-40">Дата</th>
              <th className="px-4 py-3 w-28" />
            </tr>
          </thead>
          <tbody>
            {games.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-10 text-center text-sm text-muted-foreground">
                  Игр пока нет — создай первую
                </td>
              </tr>
            ) : (
              games.map((g) => (
                <tr
                  key={g.id}
                  onClick={() => navigate(`/games/${g.id}`)}
                  className="border-b border-border/40 last:border-0 hover:bg-muted/10 cursor-pointer transition-colors"
                >
                  <td className="px-4 py-3 text-xs font-mono text-muted-foreground">{g.id}</td>
                  <td className="px-4 py-3 text-xs font-mono">
                    {g.current_problem_index + 1}/{g.problem_ids.length} · {g.problem_ids[g.current_problem_index] ?? '—'}
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`px-2 py-0.5 rounded-full text-xs font-medium ${statusClass[g.status]}`}
                    >
                      {statusLabel[g.status]}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">{g.participants.length}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">
                    {formatDate(g.created_at)}
                  </td>
                  <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                    {rowAction(g)}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Create game modal */}
      {modalOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
          onClick={() => setModalOpen(false)}
        >
          <div
            className="bg-card border border-border/60 rounded-xl p-6 w-full max-w-sm shadow-2xl shadow-black/40 mx-4"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="text-lg font-semibold mb-5">Новая игра</h2>

            <div className="flex flex-col gap-2">
              <label className="text-sm text-muted-foreground">
                Задачи ({selectedProblemIds.length}/20)
              </label>
              {problems.length === 0 ? (
                <p className="text-sm text-muted-foreground">Загрузка задач...</p>
              ) : (
                <div className="max-h-52 overflow-y-auto rounded-md border border-input px-3 py-2">
                  <div className="flex flex-col gap-2">
                    {problems.map((p) => {
                      const checked = selectedProblemIds.includes(p.id)
                      const disabled = !checked && selectedProblemIds.length >= 20
                      return (
                        <label key={p.id} className="flex items-center gap-2 text-sm">
                          <input
                            type="checkbox"
                            checked={checked}
                            disabled={disabled}
                            onChange={(e) => toggleProblem(p.id, e.target.checked)}
                          />
                          <span>{p.title}</span>
                          <span className="text-xs text-muted-foreground">({p.id})</span>
                        </label>
                      )
                    })}
                  </div>
                </div>
              )}
            </div>

            {actionError && <p className="text-sm text-destructive mt-3">{actionError}</p>}

            <div className="flex gap-2 mt-6">
              <Button
                variant="outline"
                className="flex-1"
                onClick={() => {
                  setModalOpen(false)
                  setActionError('')
                }}
              >
                Отмена
              </Button>
              <Button
                className="flex-1"
                onClick={handleCreateGame}
                disabled={creating || selectedProblemIds.length === 0}
              >
                {creating ? 'Создаём...' : 'Создать →'}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
