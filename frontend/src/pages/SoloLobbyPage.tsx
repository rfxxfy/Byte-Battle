import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { getGame, startGame, cancelGame, type Game } from '@/api/games'
import { ApiError } from '@/api/client'
import { errorMessage } from '@/lib/errors'
import { useAuth } from '@/context/AuthContext'
import { Button } from '@/components/ui/button'

function formatTimer(minutes: number | null | undefined): string {
  if (!minutes) return 'Без таймера'
  if (minutes < 60) return `${minutes} мин`
  return `${minutes / 60} ч`
}

export function SoloLobbyPage() {
  const { id } = useParams<{ id: string }>()
  const gameId = Number(id)
  const { userId } = useAuth()
  const navigate = useNavigate()

  const [game, setGame] = useState<Game | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const [countdown, setCountdown] = useState<3 | 2 | 1 | 0 | null>(null)

  const prevStatusRef = useRef<string | undefined>(undefined)

  const fetchGame = useCallback(async () => {
    try {
      const res = await getGame(gameId)
      const g = res.game

      if (!g.is_solo) {
        if (g.invite_token) navigate(`/games/join/${g.invite_token}`, { replace: true })
        else navigate('/games', { replace: true })
        return
      }

      if (g.status === 'active') {
        if (prevStatusRef.current === 'pending') {
          prevStatusRef.current = g.status
          setGame(g)
          setCountdown(3)
          setTimeout(() => setCountdown(2), 1000)
          setTimeout(() => setCountdown(1), 2000)
          setTimeout(() => setCountdown(0), 3000)
          setTimeout(() => { setCountdown(null); navigate(`/games/${gameId}`) }, 4000)
        } else {
          navigate(`/games/${gameId}`, { replace: true })
        }
        return
      }
      if (g.status === 'finished' || g.status === 'cancelled') {
        navigate(`/games/${gameId}/results`, { replace: true })
        return
      }

      prevStatusRef.current = g.status
      setGame(g)
    } catch (err) {
      setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    } finally {
      setLoading(false)
    }
  }, [gameId, navigate])

  useEffect(() => {
    fetchGame()
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (game?.status !== 'pending') return
    const timer = setInterval(fetchGame, 1000)
    return () => clearInterval(timer)
  }, [game?.status, fetchGame])

  const handleStart = async () => {
    if (!game) return
    setActionError('')
    try {
      await startGame(game.id)
      await fetchGame()
    } catch (err) {
      setActionError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    }
  }

  const handleCancel = async () => {
    if (!game) return
    setActionError('')
    try {
      await cancelGame(game.id)
      navigate('/games')
    } catch (err) {
      setActionError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    }
  }

  if (loading) return <p className="text-sm text-muted-foreground">Загрузка...</p>

  if (error || !game) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[calc(100vh-80px)] gap-3">
        <p className="text-sm text-destructive">{error || 'Игра не найдена'}</p>
        <Button variant="outline" onClick={() => navigate('/games')}>К списку игр</Button>
      </div>
    )
  }

  const isCreator = userId != null && game.creator_id === userId

  return (
    <div className="flex flex-col items-center justify-center min-h-[calc(100vh-80px)] py-8">
      <div className="w-full max-w-md flex flex-col gap-4">
        <Link
          to="/games"
          className="text-sm text-muted-foreground hover:text-foreground transition-colors self-start"
        >
          ← Игры
        </Link>

        <div className="rounded-lg border border-border/60 bg-card/50 p-5">
          <p className="text-xs text-muted-foreground mb-1">Соло · Игра #{game.id}</p>
          <p className="text-sm font-medium">
            {game.problem_ids.length === 1 ? '1 задача' : `${game.problem_ids.length} задачи`}
          </p>
          <p className="text-xs text-muted-foreground mt-2">
            Таймер: <span className="text-foreground">{formatTimer(game.time_limit_minutes)}</span>
          </p>
        </div>

        {actionError && <p className="text-sm text-destructive">{actionError}</p>}

        {isCreator ? (
          <div className="flex flex-col gap-2">
            <Button onClick={handleStart} className="w-full">
              Начать
            </Button>
            <Button variant="outline" onClick={handleCancel} className="w-full">
              Отменить
            </Button>
          </div>
        ) : (
          <p className="text-sm text-center text-muted-foreground">Нет доступа к этой игре</p>
        )}
      </div>

      {countdown !== null && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm overflow-hidden">
          <span
            key={countdown}
            className={`animate-count-tick font-bold tracking-tight select-none ${countdown === 0 ? 'text-4xl text-primary' : 'text-8xl'}`}
          >
            {countdown === 0 ? 'Начали!' : countdown}
          </span>
        </div>
      )}
    </div>
  )
}
