import { useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { uploadProblem, uploadProblemVersion, type UploadResult, type UploadResponse } from '@/api/problems'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'

function isBatch(r: UploadResponse): r is { problems: UploadResult[] } {
  return 'problems' in r
}

function ProblemResult({ title, version, slug }: { title: string; version: number; slug: string }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    navigator.clipboard.writeText(slug)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-baseline gap-2">
        <span className="text-sm font-medium">{title}</span>
        <span className="text-xs text-muted-foreground">v{version}</span>
      </div>
      <div className="flex items-center gap-2">
        <span className="font-mono text-xs text-muted-foreground flex-1 truncate">{slug}</span>
        <button
          onClick={copy}
          className={`text-xs shrink-0 transition-colors ${
            copied ? 'text-green-400' : 'text-muted-foreground hover:text-foreground'
          }`}
        >
          {copied ? '✓ скопировано' : 'копировать'}
        </button>
      </div>
    </div>
  )
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} Б`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} КБ`
  return `${(bytes / 1024 / 1024).toFixed(2)} МБ`
}

interface Props {
  mode: 'new' | 'version'
  slug?: string
  onClose: () => void
  onUploaded: () => void
}

export function UploadProblemModal({ mode, slug, onClose, onUploaded }: Props) {
  const navigate = useNavigate()
  const [file, setFile] = useState<File | null>(null)
  const [visibility, setVisibility] = useState<'public' | 'unlisted' | 'private'>('public')
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState('')
  const [result, setResult] = useState<UploadResponse | null>(null)
  const [dragging, setDragging] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleFile = (f: File) => {
    setFile(f)
    setError('')
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    setDragging(false)
    const f = e.dataTransfer.files[0]
    if (f) handleFile(f)
  }

  const handleUpload = async () => {
    if (!file) return
    setUploading(true)
    setError('')
    try {
      const res =
        mode === 'new'
          ? await uploadProblem(file, visibility)
          : await uploadProblemVersion(slug!, file)
      setResult(res)
      onUploaded()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err))
    } finally {
      setUploading(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onClick={!uploading ? onClose : undefined}
    >
      <div
        className="bg-card border border-border/60 rounded-xl p-6 w-full max-w-md shadow-2xl shadow-black/40 mx-4"
        onClick={e => e.stopPropagation()}
      >
        <h2 className="text-lg font-semibold mb-1">
          {mode === 'new' ? 'Загрузить задачу' : `Новая версия — ${slug}`}
        </h2>
        <p className="text-sm text-muted-foreground mb-5">
          {mode === 'new'
            ? 'Архив .tar.gz или .zip с manifest.json, statement.md, reference/ и tests/'
            : 'Загрузите архив с обновлённой задачей'}
        </p>

        {result ? (
          <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-4">
              <p className="text-sm font-semibold text-green-400">✓ Загружено успешно!</p>
              {isBatch(result) ? (
                <div className="flex flex-col gap-3">
                  {result.problems.map(p => (
                    <ProblemResult key={p.slug} title={p.title} version={p.version} slug={p.slug} />
                  ))}
                </div>
              ) : (
                <ProblemResult title={result.title} version={result.version} slug={result.slug} />
              )}
            </div>
            <div className="flex gap-2">
              <Button variant="outline" className="flex-1" onClick={onClose}>
                Закрыть
              </Button>
              {!isBatch(result) && (
                <Button
                  className="flex-1"
                  onClick={() => { onClose(); navigate(`/problems/${result.slug}`) }}
                >
                  Открыть задачу →
                </Button>
              )}
            </div>
          </div>
        ) : (
          <div className="flex flex-col gap-4">
            <div
              className={`border-2 border-dashed rounded-lg px-4 py-8 text-center cursor-pointer transition-colors ${
                dragging
                  ? 'border-primary bg-primary/5'
                  : file
                    ? 'border-border bg-muted/20'
                    : 'border-border/60 hover:border-border'
              }`}
              onClick={() => fileInputRef.current?.click()}
              onDragOver={e => { e.preventDefault(); setDragging(true) }}
              onDragLeave={() => setDragging(false)}
              onDrop={handleDrop}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".tar.gz,.zip,application/gzip,application/zip,application/x-gzip"
                className="hidden"
                onChange={e => {
                  const f = e.target.files?.[0]
                  if (f) handleFile(f)
                }}
              />
              {file ? (
                <div>
                  <p className="text-sm font-medium">{file.name}</p>
                  <p className="text-xs text-muted-foreground mt-1">
                    {formatFileSize(file.size)} · нажмите для замены
                  </p>
                </div>
              ) : (
                <div>
                  <p className="text-sm text-muted-foreground">
                    Перетащите архив сюда или нажмите для выбора
                  </p>
                  <p className="text-xs text-muted-foreground/60 mt-1">.tar.gz или .zip, до 50 МБ</p>
                </div>
              )}
            </div>

            {mode === 'new' && (
              <div className="flex flex-col gap-1.5">
                <label className="text-sm text-muted-foreground">Доступ</label>
                <div className="flex gap-2">
                  {(['public', 'unlisted', 'private'] as const).map(v => (
                    <button
                      key={v}
                      type="button"
                      onClick={() => setVisibility(v)}
                      className={`flex-1 py-1.5 rounded-md text-xs border transition-colors ${
                        visibility === v
                          ? 'border-primary bg-primary/10 text-foreground'
                          : 'border-border text-muted-foreground hover:text-foreground'
                      }`}
                    >
                      {v === 'public' ? 'Публичная' : v === 'unlisted' ? 'Скрытая' : 'Приватная'}
                    </button>
                  ))}
                </div>
                <p className="text-xs text-muted-foreground/70">
                  {visibility === 'public' && 'Появляется в каталоге задач, доступна всем'}
                  {visibility === 'unlisted' && 'Не в каталоге, открывается по прямой ссылке'}
                  {visibility === 'private' && 'Доступна только вам'}
                </p>
              </div>
            )}

            {error && (
              <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2">
                <p className="text-sm text-destructive whitespace-pre-wrap">{error}</p>
              </div>
            )}

            <p className="text-xs text-muted-foreground/60">
              Авторское решение будет запущено против всех тест-кейсов
            </p>

            <div className="flex gap-2">
              <Button variant="outline" className="flex-1" onClick={onClose} disabled={uploading}>
                Отмена
              </Button>
              <Button className="flex-1" onClick={handleUpload} disabled={!file || uploading}>
                {uploading ? 'Загружаем и валидируем...' : 'Загрузить'}
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
