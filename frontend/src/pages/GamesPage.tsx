import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { listGames, createGame, type Game, type GameParticipant } from '@/api/games'
import { listProblems, type Problem } from '@/api/problems'
import { ApiError } from '@/api/client'
import { errorMessage } from '@/lib/errors'
import { formatTimer } from '@/lib/utils'
import { Button } from '@/components/ui/button'

function avatarInitials(p: GameParticipant): string {
  if (p.name) return p.name.slice(0, 2).toUpperCase()
  return p.id.slice(0, 2).toUpperCase()
}

const statusLabel: Record<Game['status'], string> = {
  pending: 'Ожидание',
  active: 'Идёт',
  finished: 'Завершена',
  cancelled: 'Отменена',
}

const statusClass: Record<Game['status'], string> = {
  pending: 'text-yellow-400 bg-yellow-400/10',
  active: 'text-green-400 bg-green-400/10',
  finished: 'text-blue-400 bg-blue-400/10',
  cancelled: 'text-muted-foreground bg-muted/40',
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

const MAX_DOTS = 6

function ProblemDots({ game }: { game: Game }) {
  const total = game.problem_ids.length
  const visible = Math.min(total, MAX_DOTS)
  const overflow = total - MAX_DOTS

  return (
    <div className="flex items-center gap-1.5">
      {Array.from({ length: visible }).map((_, i) => (
        <span
          key={i}
          className={
            game.status === 'finished'
              ? 'w-2 h-2 rounded-full bg-primary/70'
              : game.status === 'active'
                ? 'w-2 h-2 rounded-full bg-muted-foreground/40 animate-pulse'
                : 'w-2 h-2 rounded-full bg-muted-foreground/25'
          }
        />
      ))}
      {overflow > 0 && (
        <span className="text-xs text-muted-foreground/50 leading-none">+{overflow}</span>
      )}
    </div>
  )
}

type TabId = 'multiplayer' | 'solo'

type TimerOption = 15 | 30 | 60 | 90 | 120 | 'custom' | null

export function GamesPage() {
  const navigate = useNavigate()

  const [games, setGames] = useState<Game[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const [activeTab, setActiveTab] = useState<TabId>('multiplayer')

  const [modalOpen, setModalOpen] = useState(false)
  const [problems, setProblems] = useState<Problem[]>([])
  const [selectedProblemIds, setSelectedProblemIds] = useState<string[]>([])
  const [isPublic, setIsPublic] = useState(true)
  const [isSolo, setIsSolo] = useState(false)
  const [timerOption, setTimerOption] = useState<TimerOption>(null)
  const [customMinutes, setCustomMinutes] = useState('')
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    listGames(50, 0)
      .then((res) => setGames(res.games))
      .catch((err) =>
        setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err)),
      )
      .finally(() => setLoading(false))
  }, [])

  const closeModal = () => {
    setModalOpen(false)
    setActionError('')
    setIsPublic(true)
    setIsSolo(false)
    setTimerOption(null)
    setCustomMinutes('')
  }

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

  const resolvedTimeLimitMinutes = (): number | null => {
    if (!isSolo || timerOption === null) return null
    if (timerOption === 'custom') {
      const n = parseInt(customMinutes, 10)
      return isNaN(n) ? null : n
    }
    return timerOption
  }

  const customMinutesValid =
    timerOption !== 'custom' ||
    (() => {
      const n = parseInt(customMinutes, 10)
      return !isNaN(n) && n >= 1 && n <= 300
    })()

  const handleCreateGame = async () => {
    if (selectedProblemIds.length === 0) return
    if (selectedProblemIds.length > 20) {
      setActionError('Можно выбрать не более 20 задач')
      return
    }
    if (!customMinutesValid) return
    setCreating(true)
    setActionError('')
    try {
      const timeLimitMinutes = resolvedTimeLimitMinutes()
      const res = await createGame(selectedProblemIds, isSolo ? false : isPublic, isSolo, timeLimitMinutes)
      closeModal()
      if (isSolo) {
        navigate(`/games/${res.game.id}/lobby`)
      } else {
        navigate(`/games/join/${res.game.invite_token}`)
      }
    } catch (err) {
      setActionError(
        err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err),
      )
      setCreating(false)
    }
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

  const multiplayerGames = games.filter((g) => !g.is_solo)
  const soloGames = games.filter((g) => g.is_solo)
  const visibleGames = activeTab === 'multiplayer' ? multiplayerGames : soloGames

  if (loading) return <p className="text-sm text-muted-foreground">Загрузка...</p>
  if (error) return <p className="text-sm text-destructive">{error}</p>

  return (
    <div className="flex flex-col gap-6 py-8">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Игры</h1>
        <Button onClick={openModal}>Создать игру</Button>
      </div>

      <div className="flex gap-1 border-b border-border/60">
        {(['multiplayer', 'solo'] as TabId[]).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px ${
              activeTab === tab
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            }`}
          >
            {tab === 'multiplayer' ? 'Мультиплеер' : 'Соло'}
          </button>
        ))}
      </div>

      {actionError && <p className="text-sm text-destructive">{actionError}</p>}

      <div className="rounded-lg border border-border/60 overflow-hidden">
        <table className="w-full text-sm table-fixed">
          <thead>
            <tr className="border-b border-border/60 bg-muted/30 text-xs font-medium text-muted-foreground">
              <th className="px-4 py-3 text-left w-12">#</th>
              <th className="px-4 py-3 text-left w-24">Задачи</th>
              <th className="px-4 py-3 text-left w-36">Статус</th>
              <th className="px-4 py-3 text-left">
                {activeTab === 'multiplayer' ? 'Участники' : 'Таймер'}
              </th>
              <th className="px-4 py-3 text-left w-40">Дата</th>
            </tr>
          </thead>
          <tbody>
            {visibleGames.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-10 text-center text-sm text-muted-foreground">
                  {activeTab === 'multiplayer' ? 'Пока нет игр' : 'Пока нет соло-практик'}
                </td>
              </tr>
            ) : (
              visibleGames.map((g, idx) => (
                <tr
                  key={g.id}
                  onClick={() => {
                    if (g.status === 'finished') navigate(`/games/${g.id}/results`)
                    else if (g.status === 'pending' && g.is_solo) navigate(`/games/${g.id}/lobby`)
                    else if (g.status === 'pending' && g.invite_token) navigate(`/games/join/${g.invite_token}`)
                    else navigate(`/games/${g.id}`)
                  }}
                  className="border-b border-border/40 last:border-0 even:bg-muted/20 hover:bg-muted/10 cursor-pointer transition-colors"
                >
                  <td className="px-4 py-4 text-xs font-mono text-muted-foreground/40">{idx + 1}</td>
                  <td className="px-4 py-4"><ProblemDots game={g} /></td>
                  <td className="px-4 py-4">
                    <span className={`px-2.5 py-0.5 rounded-full text-xs font-medium ${statusClass[g.status]}`}>
                      {statusLabel[g.status]}
                    </span>
                  </td>
                  {activeTab === 'multiplayer' ? (
                    <td className="px-4 py-4">
                      <div className="flex items-center gap-1">
                        {g.participants.slice(0, 4).map((p, idx) => (
                          <div
                            key={p.id}
                            title={p.name ?? undefined}
                            style={{ zIndex: g.participants.length - idx, marginLeft: idx === 0 ? 0 : '-6px' }}
                            className="w-7 h-7 rounded-full bg-muted border-2 border-card flex items-center justify-center text-[10px] font-semibold text-muted-foreground"
                          >
                            {avatarInitials(p)}
                          </div>
                        ))}
                        {g.participants.length > 4 && (
                          <span className="text-xs text-muted-foreground ml-2">
                            +{g.participants.length - 4}
                          </span>
                        )}
                        {g.participants.length === 0 && (
                          <span className="text-xs text-muted-foreground/40">—</span>
                        )}
                      </div>
                    </td>
                  ) : (
                    <td className="px-4 py-4 text-xs text-muted-foreground">
                      {formatTimer(g.time_limit_minutes)}
                    </td>
                  )}
                  <td className="px-4 py-4 text-left text-xs text-muted-foreground/60">
                    {formatDate(g.created_at)}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {modalOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
          onClick={closeModal}
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
                        </label>
                      )
                    })}
                  </div>
                </div>
              )}
            </div>

            <label className="flex items-center gap-2 text-sm mt-4 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={isSolo}
                onChange={(e) => {
                  setIsSolo(e.target.checked)
                  setTimerOption(null)
                  setCustomMinutes('')
                }}
              />
              <span>Одиночная игра</span>
            </label>

            {isSolo ? (
              <div className="flex flex-col gap-2 mt-4">
                <label className="text-sm text-muted-foreground">Таймер</label>
                <div className="flex flex-wrap gap-2">
                  {([15, 30, 60, 90, 120] as const).map((m) => (
                    <button
                      key={m}
                      type="button"
                      onClick={() => setTimerOption(m)}
                      className={`px-3 py-1.5 rounded-md text-xs border transition-colors ${
                        timerOption === m
                          ? 'border-primary bg-primary/10 text-foreground'
                          : 'border-border text-muted-foreground hover:text-foreground'
                      }`}
                    >
                      {m} мин
                    </button>
                  ))}
                  <button
                    type="button"
                    onClick={() => setTimerOption('custom')}
                    className={`px-3 py-1.5 rounded-md text-xs border transition-colors ${
                      timerOption === 'custom'
                        ? 'border-primary bg-primary/10 text-foreground'
                        : 'border-border text-muted-foreground hover:text-foreground'
                    }`}
                  >
                    Своё
                  </button>
                  <button
                    type="button"
                    onClick={() => setTimerOption(null)}
                    className={`px-3 py-1.5 rounded-md text-xs border transition-colors ${
                      timerOption === null
                        ? 'border-primary bg-primary/10 text-foreground'
                        : 'border-border text-muted-foreground hover:text-foreground'
                    }`}
                  >
                    Без таймера
                  </button>
                </div>
                {timerOption === 'custom' && (
                  <div className="flex items-center gap-2 mt-1">
                    <input
                      type="number"
                      min={1}
                      max={300}
                      value={customMinutes}
                      onChange={(e) => setCustomMinutes(e.target.value)}
                      placeholder="1–300"
                      className="w-24 rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                    />
                    <span className="text-xs text-muted-foreground">минут (1–300)</span>
                  </div>
                )}
              </div>
            ) : (
              <label className="flex items-center gap-2 text-sm mt-4 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={isPublic}
                  onChange={(e) => setIsPublic(e.target.checked)}
                />
                <span>Публичная игра</span>
                {!isPublic && (
                  <span className="text-xs text-muted-foreground">(только по ссылке)</span>
                )}
              </label>
            )}

            <p className="text-xs text-muted-foreground/60 mt-4">
              После создания вы попадёте в лобби
            </p>

            {actionError && <p className="text-sm text-destructive mt-3">{actionError}</p>}

            <div className="flex gap-2 mt-4">
              <Button
                variant="outline"
                className="flex-1"
                onClick={closeModal}
              >
                Отмена
              </Button>
              <Button
                className="flex-1"
                onClick={handleCreateGame}
                disabled={creating || selectedProblemIds.length === 0 || !customMinutesValid}
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
