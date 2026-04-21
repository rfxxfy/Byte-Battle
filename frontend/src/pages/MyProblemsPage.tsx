import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { listMyProblems, patchProblem, type MyProblem } from '@/api/problems'
import { ApiError } from '@/api/client'
import { errorMessage } from '@/lib/errors'
import { Button } from '@/components/ui/button'
import { UploadProblemModal } from '@/components/UploadProblemModal'

const visibilityLabel: Record<MyProblem['visibility'], string> = {
  public: 'Публичная',
  unlisted: 'Скрытая',
  private: 'Приватная',
}

const visibilityColor: Record<MyProblem['visibility'], string> = {
  public: 'text-green-400',
  unlisted: 'text-yellow-400',
  private: 'text-muted-foreground',
}

function VisibilityDropdown({
  value,
  onChange,
}: {
  value: MyProblem['visibility']
  onChange: (v: MyProblem['visibility']) => void
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  return (
    <div ref={ref} className="relative w-fit">
      <button
        type="button"
        onClick={e => { e.stopPropagation(); setOpen(o => !o) }}
        className={`flex items-center gap-1 text-xs font-medium transition-opacity hover:opacity-70 ${visibilityColor[value]}`}
      >
        {visibilityLabel[value]}
        <svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor" className="opacity-60">
          <path d="M2 3.5L5 6.5L8 3.5" stroke="currentColor" strokeWidth="1.2" fill="none" strokeLinecap="round"/>
        </svg>
      </button>
      {open && (
        <div className="absolute top-full left-0 mt-1 z-20 bg-popover border border-border rounded-md shadow-lg overflow-hidden min-w-[7rem]">
          {(Object.keys(visibilityLabel) as MyProblem['visibility'][]).map(v => (
            <button
              key={v}
              type="button"
              onClick={e => { e.stopPropagation(); onChange(v); setOpen(false) }}
              className={`w-full text-left px-3 py-1.5 text-xs transition-colors hover:bg-muted/50 ${
                v === value ? visibilityColor[v] + ' font-medium' : 'text-muted-foreground'
              }`}
            >
              {visibilityLabel[v]}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

export function MyProblemsPage() {
  const navigate = useNavigate()
  const [problems, setProblems] = useState<MyProblem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [patchError, setPatchError] = useState('')
  const [uploadModal, setUploadModal] = useState<{ mode: 'new' | 'version'; slug?: string } | null>(null)

  const load = useCallback((initial = false) => {
    if (initial) setLoading(true)
    listMyProblems()
      .then(res => setProblems(res.problems))
      .catch(err =>
        setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err)),
      )
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { load(true) }, [load])

  const handleVisibilityChange = async (id: string, visibility: MyProblem['visibility']) => {
    setPatchError('')
    setProblems(prev => prev.map(p => p.id === id ? { ...p, visibility } : p))
    try {
      await patchProblem(id, visibility)
    } catch (err) {
      load()
      setPatchError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    }
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
          <table className="w-full text-sm table-fixed">
            <thead>
              <tr className="border-b border-border/60 bg-muted/30 text-xs font-medium text-muted-foreground">
                <th className="px-4 py-3 text-left">Название</th>
                <th className="px-4 py-3 text-left w-48">Slug</th>
                <th className="px-4 py-3 text-left w-28">Доступ</th>
                <th className="px-4 py-3 text-right w-24">Версия</th>
              </tr>
            </thead>
            <tbody>
              {problems.map(p => (
                <tr
                  key={p.id}
                  onClick={() => navigate(`/problems/${p.id}`)}
                  className="border-b border-border/40 last:border-0 even:bg-muted/20 hover:bg-muted/10 cursor-pointer transition-colors"
                >
                  <td className="px-4 py-4 max-w-0"><div className="truncate font-medium">{p.title}</div></td>
                  <td className="px-4 py-4 max-w-0"><div className="truncate font-mono text-xs text-muted-foreground/50">{p.id}</div></td>
                  <td className="px-4 py-4" onClick={e => e.stopPropagation()}>
                    <VisibilityDropdown
                      value={p.visibility}
                      onChange={v => handleVisibilityChange(p.id, v)}
                    />
                  </td>
                  <td className="px-4 py-4 text-xs text-muted-foreground text-right">
                    {p.version != null ? `v${p.version}` : '—'}
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
          onUploaded={() => load()}
        />
      )}
    </div>
  )
}
