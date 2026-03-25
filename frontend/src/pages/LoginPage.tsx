import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { enter, confirm } from '../api/auth'
import { ApiError } from '../api/client'
import { useAuth } from '../context/AuthContext'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { InputOTP, InputOTPGroup, InputOTPSlot } from '@/components/ui/input-otp'

export function LoginPage() {
  const navigate = useNavigate()
  const { login } = useAuth()

  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [step, setStep] = useState<'email' | 'code'>('email')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleEnter = async (e: React.SyntheticEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await enter(email)
      setStep('code')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Что-то пошло не так')
    } finally {
      setLoading(false)
    }
  }

  const handleConfirm = async (e: React.SyntheticEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await confirm(email, code)
      login(res.token, res.expires_at)
      navigate('/games', { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Что-то пошло не так')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="relative min-h-screen flex items-center justify-center bg-background overflow-hidden">
      {/* Grid background */}
      <div
        className="absolute inset-0 opacity-[0.15]"
        style={{
          backgroundImage: `
            linear-gradient(rgba(34,197,94,0.4) 1px, transparent 1px),
            linear-gradient(90deg, rgba(34,197,94,0.4) 1px, transparent 1px)
          `,
          backgroundSize: '40px 40px',
        }}
      />
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_60%_50%_at_50%_50%,rgba(34,197,94,0.08),transparent)]" />

      <Card className="relative z-10 w-full max-w-md border-border/60 bg-card/80 backdrop-blur-sm shadow-2xl shadow-green-950/20">
        <CardHeader className="pb-4 text-center space-y-1 pt-8">
          <h1 className="text-2xl font-semibold tracking-tight">Byte Battle</h1>
          <p className="text-sm text-muted-foreground">
            {step === 'email' ? 'Введи email для входа' : `Код отправлен на ${email}`}
          </p>
        </CardHeader>

        <CardContent className="pb-8 px-8">
          {step === 'email' ? (
            <form onSubmit={handleEnter} className="flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="you@example.com"
                  className="h-11 text-base"
                  required
                  autoFocus
                />
              </div>
              {error && <p className="text-destructive text-sm">{error}</p>}
              <Button type="submit" disabled={loading} className="w-full h-11 text-base mt-1">
                {loading ? 'Отправляем...' : 'Получить код →'}
              </Button>
            </form>
          ) : (
            <form onSubmit={handleConfirm} className="flex flex-col gap-6">
              <div className="flex flex-col items-center gap-3">
                <Label>Код из письма</Label>
                <InputOTP
                  maxLength={6}
                  value={code}
                  onChange={setCode}
                  autoFocus
                  containerClassName="gap-2"
                >
                  {[0, 1, 2, 3, 4, 5].map((i) => (
                    <InputOTPGroup key={i}>
                      <InputOTPSlot index={i} className="h-13 w-13 text-xl font-bold rounded-lg border border-input" />
                    </InputOTPGroup>
                  ))}
                </InputOTP>
              </div>
              {error && <p className="text-destructive text-sm text-center">{error}</p>}
              <Button
                type="submit"
                disabled={loading || code.length < 6}
                className="w-full h-11 text-base"
              >
                {loading ? 'Входим...' : 'Войти →'}
              </Button>
              <button
                type="button"
                onClick={() => { setStep('email'); setCode(''); setError('') }}
                className="text-sm text-muted-foreground hover:text-foreground transition-colors text-center"
              >
                ← Изменить email
              </button>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
