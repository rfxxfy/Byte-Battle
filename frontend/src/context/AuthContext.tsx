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
  login: (token: string, userId: string, name: string | null, email: string | null) => void
  logout: () => Promise<void>
  setName: (name: string) => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>(() => {
    const token = localStorage.getItem('token')
    return { token, userId: null, email: null, name: null, loading: !!token }
  })

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) return
    me()
      .then((res) => {
        setState((prev) => ({ ...prev, token, userId: res.user_id, email: res.email ?? null, name: res.name ?? null, loading: false }))
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) {
          localStorage.removeItem('token')
          setState({ token: null, userId: null, email: null, name: null, loading: false })
        } else {
          setState((prev) => ({ ...prev, userId: null, loading: false }))
        }
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

  const login = (token: string, userId: string, name: string | null, email: string | null) => {
    localStorage.setItem('token', token)
    setState({ token, userId, name, email, loading: false })
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

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
