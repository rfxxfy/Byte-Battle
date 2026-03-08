import { createContext, useContext, useEffect, useState } from 'react'
import { me, logout as apiLogout } from '../api/auth'
import { ApiError } from '../api/client'

interface AuthState {
  token: string | null
  userId: string | null
  email: string | null
  name: string | null
  loading: boolean
}

interface AuthContextValue extends AuthState {
  login: (token: string, expiresAt: string) => void
  logout: () => Promise<void>
  setName: (name: string) => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>({
    token: localStorage.getItem('token'),
    userId: null,
    email: null,
    name: null,
    loading: true,
  })

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) {
      setState({ token: null, userId: null, email: null, name: null, loading: false })
      return
    }
    me()
      .then((res) => {
        setState((prev) => ({ ...prev, token, userId: res.user_id, name: res.name ?? null, loading: false }))
      })
      .catch((err) => {
        if (err instanceof ApiError) {
          localStorage.removeItem('token')
        }
        setState({ token: null, userId: null, email: null, name: null, loading: false })
      })
  }, [])

  useEffect(() => {
    const handler = () => {
      localStorage.removeItem('token')
      setState({ token: null, userId: null, email: null, name: null, loading: false })
    }
    window.addEventListener('unauthorized', handler)
    return () => window.removeEventListener('unauthorized', handler)
  }, [])

  const login = (token: string, _expiresAt: string) => {
    localStorage.setItem('token', token)
    setState((prev) => ({ ...prev, token, loading: true }))
    me()
      .then((res) => {
        setState((prev) => ({ ...prev, token, userId: res.user_id, name: res.name ?? null, loading: false }))
      })
      .catch(() => {
        setState((prev) => ({ ...prev, loading: false }))
      })
  }

  const logout = async () => {
    await apiLogout().catch(() => {})
    localStorage.removeItem('token')
    setState({ token: null, userId: null, email: null, name: null, loading: false })
  }

  const setName = (name: string) => {
    setState((prev) => ({ ...prev, name }))
  }

  return (
    <AuthContext.Provider value={{ ...state, login, logout, setName }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
