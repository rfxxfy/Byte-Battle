import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { listProblems, type Problem } from '@/api/problems'
import { ApiError } from '@/api/client'
import { errorMessage } from '@/lib/errors'
import { difficultyLabel, difficultyClass } from '@/lib/difficulty'

export function ProblemsPage() {
  const navigate = useNavigate()
  const [problems, setProblems] = useState<Problem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    listProblems()
      .then((res) => setProblems(res.problems))
      .catch((err) =>
        setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err)),
      )
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <p className="text-sm text-muted-foreground">Загрузка...</p>
  if (error) return <p className="text-sm text-destructive">{error}</p>

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-semibold">Задачи</h1>

      <div className="rounded-lg border border-border/60 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border/60 bg-muted/30">
              <th className="px-4 py-3 text-left font-medium text-muted-foreground w-48">ID</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Название</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground w-32">Сложность</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground w-36">Лимит времени</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground w-24">Тесты</th>
            </tr>
          </thead>
          <tbody>
            {problems.map((p) => (
              <tr
                key={p.id}
                onClick={() => navigate(`/problems/${p.id}`)}
                className="border-b border-border/40 last:border-0 hover:bg-muted/20 cursor-pointer transition-colors"
              >
                <td className="px-4 py-3 font-mono text-xs text-muted-foreground">{p.id}</td>
                <td className="px-4 py-3 font-medium">{p.title}</td>
                <td className="px-4 py-3">
                  <span
                    className={`px-2 py-0.5 rounded-full text-xs font-medium ${difficultyClass[p.difficulty]}`}
                  >
                    {difficultyLabel[p.difficulty]}
                  </span>
                </td>
                <td className="px-4 py-3 text-muted-foreground">{p.time_limit_ms} мс</td>
                <td className="px-4 py-3 text-muted-foreground">{p.test_count ?? '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
