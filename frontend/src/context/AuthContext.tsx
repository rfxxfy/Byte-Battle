import { createContext, useContext, useEffect, useState } from 'react'
import { me, logout as apiLogout } from '../api/auth'

interface AuthState {
  token: string | null
  userId: number | null
  loading: boolean
}

interface AuthContextValue extends AuthState {
  login: (token: string, expiresAt: string) => void
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>({
    token: localStorage.getItem('token'),
    userId: null,
    loading: true,
  })

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) {
      setState({ token: null, userId: null, loading: false })
      return
    }
    me()
      .then((res) => {
        setState({ token, userId: res.user_id, loading: false })
      })
      .catch(() => {
        localStorage.removeItem('token')
        setState({ token: null, userId: null, loading: false })
      })
  }, [])

  const login = (token: string, _expiresAt: string) => {
    localStorage.setItem('token', token)
    setState((prev) => ({ ...prev, token, loading: true }))
    me().then((res) => {
      setState({ token, userId: res.user_id, loading: false })
    })
  }

  const logout = async () => {
    await apiLogout().catch(() => {})
    localStorage.removeItem('token')
    setState({ token: null, userId: null, loading: false })
  }

  return (
    <AuthContext.Provider value={{ ...state, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
