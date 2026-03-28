import { useAuth } from '@/context/AuthContext'

export function ProfilePage() {
  const { email } = useAuth()

  return (
    <div className="flex flex-col gap-6 max-w-2xl">
      <h1 className="text-2xl font-semibold">Профиль</h1>

      <div className="rounded-lg border border-border/60 bg-card/50 p-6 flex flex-col gap-3">
        <p className="text-xs text-muted-foreground uppercase tracking-wide font-medium">Аккаунт</p>
        <div className="flex flex-col gap-1">
          <p className="text-xs text-muted-foreground">Email</p>
          <p className="text-sm font-medium">{email}</p>
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
