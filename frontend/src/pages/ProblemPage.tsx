import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { getProblem, type Problem } from '@/api/problems'
import { createGame } from '@/api/games'
import { ApiError } from '@/api/client'
import { errorMessage } from '@/lib/errors'
import { difficultyLabel, difficultyClass } from '@/lib/difficulty'
import { Button } from '@/components/ui/button'
import { ProblemDescription } from '@/components/ProblemDescription'

export function ProblemPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [problem, setProblem] = useState<Problem | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState('')

  useEffect(() => {
    if (!id) return
    getProblem(id)
      .then((res) => setProblem(res.problem))
      .catch((err) =>
        setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err)),
      )
      .finally(() => setLoading(false))
  }, [id])

  const handleCreateGame = async () => {
    if (!problem) return
    setCreating(true)
    setCreateError('')
    try {
      const res = await createGame([problem.id])
      navigate(`/games/${res.game.id}`)
    } catch (err) {
      setCreateError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    } finally {
      setCreating(false)
    }
  }

  if (loading) return <p className="text-sm text-muted-foreground">Загрузка...</p>
  if (error) return <p className="text-sm text-destructive">{error}</p>
  if (!problem) return null

  return (
    <div className="flex flex-col gap-6 max-w-3xl">
      <Link
        to="/problems"
        className="text-sm text-muted-foreground hover:text-foreground transition-colors w-fit"
      >
        ← Все задачи
      </Link>

      <div className="flex items-start justify-between gap-4">
        <h1 className="text-2xl font-semibold">{problem.title}</h1>
        <div className="flex flex-col items-end gap-1">
          <Button onClick={handleCreateGame} disabled={creating} className="shrink-0">
            {creating ? 'Создаём...' : 'Создать игру →'}
          </Button>
          {createError && <p className="text-xs text-destructive">{createError}</p>}
        </div>
      </div>

      <div className="flex items-center gap-2 flex-wrap">
        <span
          className={`px-2.5 py-0.5 rounded-full text-xs font-medium border ${difficultyClass[problem.difficulty]}`}
        >
          {difficultyLabel[problem.difficulty]}
        </span>
        <span className="px-2.5 py-0.5 rounded-full text-xs font-medium text-muted-foreground bg-muted/50 border border-border/40">
          {problem.time_limit_ms} мс
        </span>
        <span className="px-2.5 py-0.5 rounded-full text-xs font-medium text-muted-foreground bg-muted/50 border border-border/40">
          {problem.memory_limit_mb} МБ памяти
        </span>
        {problem.test_count != null && (
          <span className="px-2.5 py-0.5 rounded-full text-xs font-medium text-muted-foreground bg-muted/50 border border-border/40">
            {problem.test_count} тестов
          </span>
        )}
      </div>

      <div className="rounded-lg border border-border/60 bg-card/50 p-6">
        <ProblemDescription content={problem.description} />
      </div>
    </div>
  )
}
