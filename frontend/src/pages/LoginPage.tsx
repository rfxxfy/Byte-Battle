import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { enter, confirm, updateMe } from '../api/auth'
import { ApiError } from '../api/client'
import { useAuth } from '../context/AuthContext'
import { errorMessage } from '@/lib/errors'
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
  const [name, setName] = useState('')
  const [step, setStep] = useState<'email' | 'code' | 'name'>('email')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [resendCountdown, setResendCountdown] = useState(0)

  useEffect(() => {
    const saved = sessionStorage.getItem('login_code_step')
    if (!saved) return
    try {
      const { email: savedEmail, codeSentAt } = JSON.parse(saved)
      const elapsed = Math.floor((Date.now() - codeSentAt) / 1000)
      const remaining = Math.max(0, 60 - elapsed)
      setEmail(savedEmail)
      setStep('code')
      setResendCountdown(remaining)
    } catch {
      sessionStorage.removeItem('login_code_step')
    }
  }, [])

  useEffect(() => {
    if (resendCountdown <= 0) return
    const timer = setTimeout(() => setResendCountdown((c) => c - 1), 1000)
    return () => clearTimeout(timer)
  }, [resendCountdown])

  const handleEnter = async (e: React.SyntheticEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await enter(email)
      sessionStorage.setItem('login_code_step', JSON.stringify({ email, codeSentAt: Date.now() }))
      setStep('code')
      setResendCountdown(60)
    } catch (err) {
      if (err instanceof ApiError && err.errorCode === 'CODE_RECENTLY_SENT') {
        const saved = sessionStorage.getItem('login_code_step')
        if (saved) {
          const { codeSentAt } = JSON.parse(saved)
          const elapsed = Math.floor((Date.now() - codeSentAt) / 1000)
          setResendCountdown(Math.max(0, 60 - elapsed))
        }
        setStep('code')
      } else {
        setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
      }
    } finally {
      setLoading(false)
    }
  }

  const handleResend = async () => {
    setError('')
    setLoading(true)
    try {
      await enter(email)
      sessionStorage.setItem('login_code_step', JSON.stringify({ email, codeSentAt: Date.now() }))
      setCode('')
      setResendCountdown(60)
    } catch (err) {
      setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
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
      sessionStorage.removeItem('login_code_step')
      login(res.token, res.expires_at)
      if (!res.name) {
        setStep('name')
      } else {
        navigate('/games', { replace: true })
      }
    } catch (err) {
      setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
    } finally {
      setLoading(false)
    }
  }

  const handleSetName = async (e: React.SyntheticEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await updateMe(name)
      navigate('/games', { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? errorMessage(err.errorCode, err.message) : String(err))
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
            {step === 'email' && 'Введите email для входа'}
            {step === 'code' && `Код отправлен на ${email}`}
            {step === 'name' && 'Последний шаг — как вас называть?'}
          </p>
        </CardHeader>

        <CardContent className="pb-8 px-8">
          {step === 'email' && (
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
          )}
          {step === 'code' && (
            <form onSubmit={handleConfirm} className="flex flex-col gap-6">
              <div className="flex flex-col items-center gap-3">
                <Label>Код из письма</Label>
                <div onPaste={(e) => {
                  e.preventDefault()
                  const text = e.clipboardData.getData('text').replace(/\D/g, '').slice(0, 6)
                  setCode(text)
                }}>
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
              </div>
              {error && <p className="text-destructive text-sm text-center">{error}</p>}
              <Button
                type="submit"
                disabled={loading || code.length < 6}
                className="w-full h-11 text-base"
              >
                {loading ? 'Входим...' : 'Войти →'}
              </Button>
              <div className="flex flex-col items-center gap-2">
                <button
                  type="button"
                  onClick={handleResend}
                  disabled={loading || resendCountdown > 0}
                  className="text-sm text-muted-foreground hover:text-foreground transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {resendCountdown > 0 ? `Отправить повторно через ${resendCountdown}с` : 'Отправить повторно'}
                </button>
                <button
                  type="button"
                  onClick={() => { sessionStorage.removeItem('login_code_step'); setStep('email'); setCode(''); setError('') }}
                  className="text-sm text-muted-foreground hover:text-foreground transition-colors text-center"
                >
                  ← Изменить email
                </button>
              </div>
            </form>
          )}
          {step === 'name' && (
            <form onSubmit={handleSetName} className="flex flex-col gap-4">
              <div className="flex flex-col gap-2">
                <Label htmlFor="name">Имя</Label>
                <Input
                  id="name"
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Как вас называть?"
                  className="h-11 text-base"
                  required
                  autoFocus
                />
              </div>
              {error && <p className="text-destructive text-sm">{error}</p>}
              <Button type="submit" disabled={loading || !name.trim()} className="w-full h-11 text-base mt-1">
                {loading ? 'Сохраняем...' : 'Готово →'}
              </Button>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
