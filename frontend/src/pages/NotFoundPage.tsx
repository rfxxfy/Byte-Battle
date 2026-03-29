import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'

export function NotFoundPage() {
  const navigate = useNavigate()

  return (
    <div className="min-h-screen bg-background flex flex-col items-center justify-center gap-4 text-center px-6">
      <p className="text-8xl font-bold text-primary/20 select-none tabular-nums">404</p>
      <div className="flex flex-col gap-1">
        <h1 className="text-lg font-semibold">Страница не найдена</h1>
        <p className="text-sm text-muted-foreground">Такого пути не существует</p>
      </div>
      <Button className="mt-2" onClick={() => navigate('/')}>На главную</Button>
    </div>
  )
}
