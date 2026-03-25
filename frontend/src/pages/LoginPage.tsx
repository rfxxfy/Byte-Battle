import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { enter, confirm } from '../api/auth'
import { ApiError } from '../api/client'
import { useAuth } from '../context/AuthContext'
import styles from './LoginPage.module.css'

export function LoginPage() {
  const navigate = useNavigate()
  const { login } = useAuth()

  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [step, setStep] = useState<'email' | 'code'>('email')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleEnter = async (e: React.FormEvent) => {
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

  const handleConfirm = async (e: React.FormEvent) => {
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
    <div className={styles.container}>
      <div className={styles.card}>
        <h1 className={styles.title}>Byte Battle</h1>

        {step === 'email' ? (
          <form onSubmit={handleEnter} className={styles.form}>
            <label className={styles.label}>
              Email
              <input
                className={styles.input}
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
                required
                autoFocus
              />
            </label>
            {error && <p className={styles.error}>{error}</p>}
            <button className={styles.button} type="submit" disabled={loading}>
              {loading ? 'Отправляем...' : 'Получить код'}
            </button>
          </form>
        ) : (
          <form onSubmit={handleConfirm} className={styles.form}>
            <p className={styles.hint}>Код отправлен на {email}</p>
            <label className={styles.label}>
              Код из письма
              <input
                className={styles.input}
                type="text"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                placeholder="123456"
                required
                autoFocus
              />
            </label>
            {error && <p className={styles.error}>{error}</p>}
            <button className={styles.button} type="submit" disabled={loading}>
              {loading ? 'Входим...' : 'Войти'}
            </button>
            <button
              className={styles.back}
              type="button"
              onClick={() => { setStep('email'); setError('') }}
            >
              ← Изменить email
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
