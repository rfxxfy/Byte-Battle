import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider, useAuth } from './context/AuthContext'
import { ProtectedRoute } from './components/ProtectedRoute'
import { AppLayout } from './components/AppLayout'
import { LoginPage } from './pages/LoginPage'
import { ProblemsPage } from './pages/ProblemsPage'
import { ProblemPage } from './pages/ProblemPage'
import { GamesPage } from './pages/GamesPage'
import { GamePage } from './pages/GamePage'
import { ProfilePage } from './pages/ProfilePage'

function RootRedirect() {
  const { token, loading } = useAuth()
  if (loading) return null
  return <Navigate to={token ? '/games' : '/login'} replace />
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/" element={<RootRedirect />} />
          <Route path="/login" element={<LoginPage />} />
          <Route
            element={
              <ProtectedRoute>
                <AppLayout />
              </ProtectedRoute>
            }
          >
            <Route path="/problems" element={<ProblemsPage />} />
            <Route path="/problems/:id" element={<ProblemPage />} />
            <Route path="/games" element={<GamesPage />} />
            <Route path="/games/:id" element={<GamePage />} />
            <Route path="/profile" element={<ProfilePage />} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
