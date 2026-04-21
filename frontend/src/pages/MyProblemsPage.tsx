import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { listMyProblems, patchProblem, type MyProblem, type UploadResult } from '@/api/problems'
import { ApiError } from '@/api/client'
import { errorMessage } from '@/lib/errors'
import { Button } from '@/components/ui/button'
import { UploadProblemModal } from '@/components/UploadProblemModal'

const visibilityLabel: Record<MyProblem['visibility'], string> = {
  public: 'Публичная',
  unlisted: 'Скрытая',
  private: 'Приватная',
}

const visibilityClass: Record<MyProblem['visibility'], string> = {
  public: 'text-green-400 bg-green-400/10',
  unlisted: 'text-yellow-400 bg-yellow-400/10',
  private: 'text-muted-foreground bg-muted/40',
}

export function MyProblemsPage() {
  const navigate = useNavigate()
  const [problems, setProblems] = useState<MyProblem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [patchError, setPatchError] = useState('')
  const [uploadModal, setUploadModal] = useState<{ mode: 'new' | 'version'; slug?: string } | null>(null)

  const load = useCallback(() => {
    setLoading(true)
    listMyProblems()
      .then(res => setProblems(res.problems))
      .catch(err =>
        setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err)),
      )
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { load() }, [load])

  const handleVisibilityChange = async (id: string, visibility: string) => {
    setPatchError('')
    try {
      await patchProblem(id, visibility)
      setProblems(prev =>
        prev.map(p => p.id === id ? { ...p, visibility: visibility as MyProblem['visibility'] } : p),
      )
    } catch (err) {
      setPatchError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    }
  }

  const handleUploaded = (_result: UploadResult) => {
    load()
  }

  if (loading) return <p className="text-sm text-muted-foreground">Загрузка...</p>
  if (error) return <p className="text-sm text-destructive">{error}</p>

  return (
    <div className="flex flex-col gap-6 py-8">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Мои задачи</h1>
        <Button onClick={() => setUploadModal({ mode: 'new' })}>Загрузить задачу</Button>
      </div>

      {patchError && <p className="text-sm text-destructive">{patchError}</p>}

      {problems.length === 0 ? (
        <div className="py-16 text-center">
          <p className="text-sm text-muted-foreground">У вас пока нет задач.</p>
          <button
            onClick={() => setUploadModal({ mode: 'new' })}
            className="mt-2 text-sm text-primary hover:underline"
          >
            Загрузить первую →
          </button>
        </div>
      ) : (
        <div className="rounded-lg border border-border/60 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border/60 bg-muted/30 text-xs font-medium text-muted-foreground">
                <th className="px-4 py-3 text-left">Название</th>
                <th className="px-4 py-3 text-left w-40">Доступ</th>
                <th className="px-4 py-3 text-left w-20">Версия</th>
                <th className="px-4 py-3 text-right w-40">Действия</th>
              </tr>
            </thead>
            <tbody>
              {problems.map(p => (
                <tr
                  key={p.id}
                  className="border-b border-border/40 last:border-0 even:bg-muted/10"
                >
                  <td className="px-4 py-3">
                    <button
                      onClick={() => navigate(`/problems/${p.id}`)}
                      className="font-medium hover:text-primary transition-colors text-left"
                    >
                      {p.title}
                    </button>
                    <div className="text-xs font-mono text-muted-foreground/50 mt-0.5">{p.id}</div>
                  </td>
                  <td className="px-4 py-3">
                    <select
                      value={p.visibility}
                      onChange={e => handleVisibilityChange(p.id, e.target.value)}
                      onClick={e => e.stopPropagation()}
                      className={`px-2.5 py-0.5 rounded-full text-xs font-medium border-0 outline-none cursor-pointer ${visibilityClass[p.visibility]} bg-transparent`}
                    >
                      {Object.entries(visibilityLabel).map(([val, label]) => (
                        <option key={val} value={val} className="bg-background text-foreground">
                          {label}
                        </option>
                      ))}
                    </select>
                  </td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">
                    {p.version != null ? `v${p.version}` : '—'}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setUploadModal({ mode: 'version', slug: p.id })}
                    >
                      Новая версия
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {uploadModal && (
        <UploadProblemModal
          mode={uploadModal.mode}
          slug={uploadModal.slug}
          onClose={() => setUploadModal(null)}
          onUploaded={handleUploaded}
        />
      )}
    </div>
  )
}
