import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { getGameByToken, joinGameByToken, startGame, cancelGame, leaveGame, type Game } from '@/api/games'
import { ApiError } from '@/api/client'
import { errorMessage } from '@/lib/errors'
import { pluralize, displayName } from '@/lib/utils'
import { useAuth } from '@/context/AuthContext'
import { Button } from '@/components/ui/button'

export function GameLobbyPage() {
  const { token } = useParams<{ token: string }>()
  const { token: authToken, userId } = useAuth()
  const navigate = useNavigate()

  const [game, setGame] = useState<Game | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')
  const [linkCopied, setLinkCopied] = useState(false)
  const [countdown, setCountdown] = useState<3 | 2 | 1 | 0 | null>(null)

  const prevStatusRef = useRef<string | undefined>(undefined)

  const fetchGame = useCallback(async () => {
    if (!token) return
    try {
      const res = await getGameByToken(token)
      const g = res.game

      if (g.status === 'active') {
        if (prevStatusRef.current === 'pending') {
          prevStatusRef.current = g.status
          setGame(g)
          setCountdown(3)
          setTimeout(() => setCountdown(2), 1000)
          setTimeout(() => setCountdown(1), 2000)
          setTimeout(() => setCountdown(0), 3000)
          setTimeout(() => { setCountdown(null); navigate(`/games/${g.id}`) }, 4000)
        } else {
          navigate(`/games/${g.id}`, { replace: true })
        }
        return
      }
      if (g.status === 'finished' || g.status === 'cancelled') {
        navigate(`/games/${g.id}/results`, { replace: true })
        return
      }

      prevStatusRef.current = g.status
      setGame(g)
    } catch (err) {
      setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    } finally {
      setLoading(false)
    }
  }, [token, navigate])

  useEffect(() => {
    fetchGame()
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (game?.status !== 'pending') return
    const timer = setInterval(fetchGame, 1000)
    return () => clearInterval(timer)
  }, [game?.status, fetchGame])

  const handleJoin = async () => {
    if (!token) return
    setActionError('')
    try {
      const res = await joinGameByToken(token)
      setGame(res.game)
    } catch (err) {
      if (err instanceof ApiError && err.errorCode === 'ALREADY_PARTICIPANT') return
      setActionError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    }
  }

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

  const handleLeave = async () => {
    if (!game) return
    setActionError('')
    try {
      await leaveGame(game.id)
      navigate('/games')
    } catch (err) {
      setActionError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    }
  }

  const handleCopyLink = () => {
    navigator.clipboard.writeText(window.location.href).then(() => {
      setLinkCopied(true)
      setTimeout(() => setLinkCopied(false), 2000)
    })
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
  const isParticipant = userId != null && game.participants.some((p) => p.id === userId)
  const canStart = isCreator && game.participants.length >= 2

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
          <p className="text-xs text-muted-foreground mb-1">
            Игра #{game.id} · {game.is_public ? 'Публичная' : 'Приватная'}
          </p>
          <p className="text-sm font-medium">
            {game.problem_ids.length} {pluralize(game.problem_ids.length, 'задача', 'задачи', 'задач')}
          </p>
        </div>

        <div className="rounded-lg border border-border/60 bg-card/50 p-5">
          <p className="text-xs font-medium text-muted-foreground mb-3">
            Участники · {game.participants.length}
          </p>
          <div className="flex flex-col gap-2">
            {game.participants.map((p) => (
              <div key={p.id} className="flex items-center gap-2 text-sm">
                <span className="w-2 h-2 rounded-full bg-primary/60 flex-shrink-0" />
                <span className={p.id === userId ? 'text-primary font-medium' : ''}>
                  {displayName(p.name, p.id)}
                  {p.id === userId && ' (ты)'}
                  {p.id === game.creator_id && (
                    <span className="ml-1.5 text-xs text-muted-foreground">создатель</span>
                  )}
                </span>
              </div>
            ))}
          </div>
        </div>

        {actionError && <p className="text-sm text-destructive">{actionError}</p>}

        {!authToken ? (
          <Button onClick={() => navigate(`/login?next=/games/join/${token}`)}>
            Войти чтобы присоединиться
          </Button>
        ) : isCreator ? (
          <div className="flex flex-col gap-2">
            <Button onClick={handleStart} disabled={!canStart} className="w-full">
              Начать игру
            </Button>
            {!canStart && (
              <p className="text-xs text-center text-muted-foreground">
                Нужен хотя бы 1 соперник
              </p>
            )}
            <Button
              variant="outline"
              onClick={handleCopyLink}
              className={`w-full transition-colors ${linkCopied ? 'border-green-500/60 text-green-400 hover:text-green-400' : ''}`}
            >
              {linkCopied ? '✓ Скопировано' : 'Скопировать ссылку'}
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
            <Button variant="outline" onClick={handleCopyLink} className={`w-full transition-colors ${linkCopied ? 'border-green-500/60 text-green-400 hover:text-green-400' : ''}`}>
              {linkCopied ? '✓ Скопировано' : 'Скопировать ссылку'}
            </Button>
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
