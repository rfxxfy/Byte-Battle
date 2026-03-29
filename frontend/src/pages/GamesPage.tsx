import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { listGames, type Game, type GameParticipant } from '@/api/games'
import { ApiError } from '@/api/client'
import { errorMessage } from '@/lib/errors'
import { formatTimer } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { CreateGameModal } from '@/components/CreateGameModal'

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

export function GamesPage() {
  const navigate = useNavigate()

  const [games, setGames] = useState<Game[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [activeTab, setActiveTab] = useState<TabId>('multiplayer')
  const [modalOpen, setModalOpen] = useState(false)

  useEffect(() => {
    listGames(50, 0)
      .then(res => setGames(res.games))
      .catch(err =>
        setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err)),
      )
      .finally(() => setLoading(false))
  }, [])

  const multiplayerGames = games.filter(g => !g.is_solo)
  const soloGames = games.filter(g => g.is_solo)
  const visibleGames = activeTab === 'multiplayer' ? multiplayerGames : soloGames

  if (loading) return <p className="text-sm text-muted-foreground">Загрузка...</p>
  if (error) return <p className="text-sm text-destructive">{error}</p>

  return (
    <div className="flex flex-col gap-6 py-8">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Игры</h1>
        <Button onClick={() => setModalOpen(true)}>Создать игру</Button>
      </div>

      <div className="flex gap-1 border-b border-border/60">
        {(['multiplayer', 'solo'] as TabId[]).map(tab => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px ${
              activeTab === tab
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            }`}
          >
            {tab === 'multiplayer' ? 'Мультиплеер' : 'Одиночные'}
            <span className={`ml-1.5 text-xs px-1.5 py-0.5 rounded-full tabular-nums ${
              activeTab === tab
                ? 'bg-primary/15 text-primary'
                : 'bg-muted text-muted-foreground'
            }`}>
              {tab === 'multiplayer' ? multiplayerGames.length : soloGames.length}
            </span>
          </button>
        ))}
      </div>

      <div className="rounded-lg border border-border/60 overflow-hidden">
        <table className="w-full text-sm table-fixed">
          <thead>
            <tr className="border-b border-border/60 bg-muted/30 text-xs font-medium text-muted-foreground">
              <th className="px-4 py-3 text-right w-14">#</th>
              <th className="px-4 py-3 text-left w-24">Задачи</th>
              <th className="px-4 py-3 text-left w-36">Статус</th>
              <th className="px-4 py-3 text-left">
                {activeTab === 'multiplayer' ? 'Участники' : 'Таймер'}
              </th>
              <th className="px-4 py-3 text-right w-40">Дата</th>
            </tr>
          </thead>
          <tbody>
            {visibleGames.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-10 text-center text-sm text-muted-foreground">
                  {activeTab === 'multiplayer' ? 'Пока нет игр' : 'Пока нет одиночных игр'}
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
                  <td className="px-4 py-4 text-xs font-mono text-muted-foreground/40 text-right">{idx + 1}</td>
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
                  <td className="px-4 py-4 text-right text-xs text-muted-foreground/60">
                    {formatDate(g.created_at)}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <CreateGameModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        defaultIsSolo={activeTab === 'solo'}
        onCreated={(gameId, isSolo, inviteToken) => {
          setModalOpen(false)
          if (isSolo) navigate(`/games/${gameId}/lobby`)
          else navigate(`/games/join/${inviteToken}`)
        }}
      />
    </div>
  )
}
