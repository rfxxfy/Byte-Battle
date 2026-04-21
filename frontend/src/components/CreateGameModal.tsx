import { useCallback, useEffect, useRef, useState } from 'react'
import { listProblems, listMyProblems, getProblem, type Problem, type MyProblem } from '@/api/problems'
import { createGame } from '@/api/games'
import { ApiError } from '@/api/client'
import { errorMessage } from '@/lib/errors'
import { difficultyLabel, difficultyClass } from '@/lib/difficulty'
import { formatTimer } from '@/lib/utils'
import { Button } from '@/components/ui/button'

interface ProblemChip {
  id: string
  title: string
  difficulty?: 'easy' | 'medium' | 'hard'
}

interface DropdownItem extends ProblemChip {
  isMine: boolean
}

type TimerOption = 15 | 30 | 60 | 90 | 120 | 'custom' | null

interface Props {
  open: boolean
  onClose: () => void
  onCreated: (gameId: number, isSolo: boolean, inviteToken?: string) => void
  initialSelected?: ProblemChip[]
}

export function CreateGameModal({ open, onClose, onCreated, initialSelected }: Props) {
  const [selectedItems, setSelectedItems] = useState<ProblemChip[]>([])
  const [search, setSearch] = useState('')
  const [dropdownVisible, setDropdownVisible] = useState(false)
  const [dropdownItems, setDropdownItems] = useState<DropdownItem[]>([])
  const [searching, setSearching] = useState(false)

  const [isSolo, setIsSolo] = useState(false)
  const [isPublic, setIsPublic] = useState(true)
  const [timerOption, setTimerOption] = useState<TimerOption>(null)
  const [customMinutes, setCustomMinutes] = useState('')
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')

  const searchRef = useRef<HTMLInputElement>(null)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!open) return
    setSelectedItems(initialSelected ?? [])
    setSearch('')
    setDropdownItems([])
    setDropdownVisible(false)
    setIsSolo(false)
    setIsPublic(true)
    setTimerOption(null)
    setCustomMinutes('')
    setCreating(false)
    setError('')
  }, [open])

  const doSearch = useCallback(async (q: string) => {
    setSearching(true)
    try {
      const [publicRes, mineRes] = await Promise.all([
        listProblems(q, 8).catch(() => ({ problems: [] as Problem[], total: 0 })),
        listMyProblems(q).catch(() => ({ problems: [] as MyProblem[] })),
      ])

      const map = new Map<string, DropdownItem>()
      for (const p of mineRes.problems) {
        map.set(p.id, { id: p.id, title: p.title, isMine: true })
      }
      for (const p of publicRes.problems) {
        const existing = map.get(p.id)
        if (existing) {
          existing.difficulty = p.difficulty
        } else {
          map.set(p.id, { id: p.id, title: p.title, difficulty: p.difficulty, isMine: false })
        }
      }

      let items = Array.from(map.values())
      items.sort((a, b) => (b.isMine ? 1 : 0) - (a.isMine ? 1 : 0))
      items = items.slice(0, 10)

      if (items.length === 0 && q.trim()) {
        try {
          const res = await getProblem(q.trim())
          items = [{
            id: res.problem.id,
            title: res.problem.title,
            difficulty: res.problem.difficulty,
            isMine: false,
          }]
        } catch {
          // not found
        }
      }

      setDropdownItems(items)
    } finally {
      setSearching(false)
    }
  }, [])

  const handleFocus = () => {
    setDropdownVisible(true)
    if (dropdownItems.length === 0 && !searching) {
      doSearch(search)
    }
  }

  const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value
    setSearch(val)
    setDropdownVisible(true)
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => doSearch(val), 300)
  }

  const toggleItem = (item: DropdownItem) => {
    setSelectedItems(prev => {
      const exists = prev.some(p => p.id === item.id)
      if (exists) return prev.filter(p => p.id !== item.id)
      if (prev.length >= 20) return prev
      return [...prev, { id: item.id, title: item.title, difficulty: item.difficulty }]
    })
  }

  const removeItem = (id: string) => {
    setSelectedItems(prev => prev.filter(p => p.id !== id))
  }

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setDropdownVisible(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

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

  const handleCreate = async () => {
    if (selectedItems.length === 0) return
    setCreating(true)
    setError('')
    try {
      const res = await createGame(
        selectedItems.map(p => p.id),
        isSolo ? false : isPublic,
        isSolo,
        resolvedTimeLimitMinutes(),
      )
      onCreated(res.game.id, isSolo, res.game.invite_token ?? undefined)
    } catch (err) {
      setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
      setCreating(false)
    }
  }

  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="bg-card border border-border/60 rounded-xl p-6 w-full max-w-sm shadow-2xl shadow-black/40 mx-4"
        onClick={e => e.stopPropagation()}
      >
        <h2 className="text-lg font-semibold mb-5">Новая игра</h2>

        <div className="flex flex-col gap-2">
          <label className="text-sm text-muted-foreground">
            Задачи{selectedItems.length > 0 ? ` (${selectedItems.length}/20)` : ''}
          </label>
          <div ref={containerRef} className="relative">
            <div
              className="min-h-[2.5rem] flex flex-wrap gap-1.5 items-center rounded-md border border-input px-3 py-1.5 cursor-text bg-background"
              onClick={() => searchRef.current?.focus()}
            >
              {selectedItems.map(item => (
                <span
                  key={item.id}
                  className="flex items-center gap-1 px-2 py-0.5 rounded-md bg-primary/15 text-sm text-foreground"
                >
                  {item.title}
                  <button
                    type="button"
                    onClick={e => { e.stopPropagation(); removeItem(item.id) }}
                    className="text-muted-foreground hover:text-foreground leading-none ml-0.5"
                  >
                    ×
                  </button>
                </span>
              ))}
              <input
                ref={searchRef}
                value={search}
                onChange={handleSearchChange}
                onFocus={handleFocus}
                placeholder={selectedItems.length === 0 ? 'Поиск задачи...' : ''}
                className="flex-1 min-w-[80px] bg-transparent outline-none text-sm placeholder:text-muted-foreground/50"
              />
            </div>

            {dropdownVisible && (
              <div className="absolute top-full left-0 right-0 mt-1 bg-popover border border-border rounded-md shadow-lg z-10 max-h-52 overflow-y-auto">
                {searching ? (
                  <div className="px-3 py-2 text-sm text-muted-foreground">Поиск...</div>
                ) : dropdownItems.length === 0 ? (
                  <div className="px-3 py-2 text-sm text-muted-foreground">
                    {search ? 'Ничего не найдено' : 'Начните вводить название или slug'}
                  </div>
                ) : (
                  dropdownItems.map(item => {
                    const isSelected = selectedItems.some(p => p.id === item.id)
                    return (
                      <button
                        key={item.id}
                        type="button"
                        onClick={() => toggleItem(item)}
                        className={`w-full flex items-center gap-2 px-3 py-2 text-sm text-left transition-colors hover:bg-muted/50 ${isSelected ? 'opacity-60' : ''}`}
                      >
                        <span className="flex-1 truncate">{item.title}</span>
                        <div className="flex items-center gap-1.5 shrink-0">
                          {item.isMine && (
                            <span className="text-xs px-1.5 py-0.5 rounded bg-violet-500/15 text-violet-400">
                              Мои
                            </span>
                          )}
                          {item.difficulty && (
                            <span className={`text-xs px-1.5 py-0.5 rounded-full border ${difficultyClass[item.difficulty]}`}>
                              {difficultyLabel[item.difficulty]}
                            </span>
                          )}
                          {isSelected && <span className="text-primary text-xs">✓</span>}
                        </div>
                      </button>
                    )
                  })
                )}
              </div>
            )}
          </div>
        </div>

        <label className="flex items-center gap-2 text-sm mt-4 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={isSolo}
            onChange={e => {
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
              {([15, 30, 60, 90, 120] as const).map(m => (
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
                  {formatTimer(m)}
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
                  onChange={e => setCustomMinutes(e.target.value)}
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
              onChange={e => setIsPublic(e.target.checked)}
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

        {error && <p className="text-sm text-destructive mt-3">{error}</p>}

        <div className="flex gap-2 mt-4">
          <Button variant="outline" className="flex-1" onClick={onClose}>
            Отмена
          </Button>
          <Button
            className="flex-1"
            onClick={handleCreate}
            disabled={creating || selectedItems.length === 0 || !customMinutesValid}
          >
            {creating ? 'Создаём...' : 'Создать →'}
          </Button>
        </div>
      </div>
    </div>
  )
}
