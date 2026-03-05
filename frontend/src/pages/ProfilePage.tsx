import { useState } from 'react'
import { useAuth } from '@/context/AuthContext'
import { updateMe } from '@/api/auth'
import { Button } from '@/components/ui/button'

export function ProfilePage() {
  const { email, name, setName } = useAuth()

  const [editing, setEditing] = useState(false)
  const [inputName, setInputName] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const handleEdit = () => {
    setInputName(name ?? '')
    setEditing(true)
    setError('')
  }

  const handleSave = async () => {
    setSaving(true)
    setError('')
    try {
      await updateMe(inputName.trim())
      setName(inputName.trim())
      setEditing(false)
    } catch {
      setError('Не удалось сохранить')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex flex-col gap-6 max-w-2xl">
      <h1 className="text-2xl font-semibold">Профиль</h1>

      <div className="rounded-lg border border-border/60 bg-card/50 p-6 flex flex-col gap-4">
        <p className="text-xs text-muted-foreground uppercase tracking-wide font-medium">Аккаунт</p>

        <div className="flex flex-col gap-1">
          <p className="text-xs text-muted-foreground">Email</p>
          <p className="text-sm font-medium">{email}</p>
        </div>

        <div className="flex flex-col gap-1">
          <p className="text-xs text-muted-foreground">Имя</p>
          {editing ? (
            <div className="flex items-center gap-2">
              <input
                autoFocus
                value={inputName}
                onChange={(e) => setInputName(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') handleSave(); if (e.key === 'Escape') setEditing(false) }}
                className="h-8 rounded-md border border-input bg-background px-3 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring w-48"
              />
              <Button size="sm" onClick={handleSave} disabled={saving}>
                {saving ? 'Сохраняем...' : 'Сохранить'}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setEditing(false)}>
                Отмена
              </Button>
            </div>
          ) : (
            <div className="flex items-center gap-3">
              <p className="text-sm font-medium">{name ?? '—'}</p>
              <button
                onClick={handleEdit}
                className="text-xs text-muted-foreground hover:text-foreground transition-colors"
              >
                Изменить
              </button>
            </div>
          )}
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>
      </div>

      <div className="rounded-lg border border-border/60 bg-card/50 p-6 flex flex-col gap-3">
        <p className="text-xs text-muted-foreground uppercase tracking-wide font-medium">Статистика</p>
        <div className="grid grid-cols-3 gap-4">
          <div className="flex flex-col gap-1">
            <p className="text-2xl font-semibold">—</p>
            <p className="text-xs text-muted-foreground">Побед</p>
          </div>
          <div className="flex flex-col gap-1">
            <p className="text-2xl font-semibold">—</p>
            <p className="text-xs text-muted-foreground">Игр сыграно</p>
          </div>
          <div className="flex flex-col gap-1">
            <p className="text-2xl font-semibold">—</p>
            <p className="text-xs text-muted-foreground">Задач решено</p>
          </div>
        </div>
      </div>
    </div>
  )
}
