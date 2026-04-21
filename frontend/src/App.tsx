import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider, useAuth } from './context/AuthContext'
import { ProtectedRoute } from './components/ProtectedRoute'
import { AppLayout } from './components/AppLayout'
import { LoginPage } from './pages/LoginPage'
import { ProblemsPage } from './pages/ProblemsPage'
import { ProblemPage } from './pages/ProblemPage'
import { MyProblemsPage } from './pages/MyProblemsPage'
import { GamesPage } from './pages/GamesPage'
import { GamePage } from './pages/GamePage'
import { ProfilePage } from './pages/ProfilePage'
import { GameResultsPage } from './pages/GameResultsPage'
import { GameLobbyPage } from './pages/GameLobbyPage'
import { SoloLobbyPage } from './pages/SoloLobbyPage'

function RootRedirect() {
  const { token, loading } = useAuth()
  if (loading) return null
  return <Navigate to={token ? '/games' : '/login'} replace />
}

function GuestRoute({ children }: { children: React.ReactNode }) {
  const { token, loading } = useAuth()
  if (loading) return null
  if (token) return <Navigate to="/games" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/" element={<RootRedirect />} />
          <Route path="/login" element={<GuestRoute><LoginPage /></GuestRoute>} />
          <Route
            element={
              <ProtectedRoute>
                <AppLayout />
              </ProtectedRoute>
            }
          >
            <Route path="/problems" element={<ProblemsPage />} />
            <Route path="/problems/mine" element={<MyProblemsPage />} />
            <Route path="/problems/:id" element={<ProblemPage />} />
            <Route path="/games" element={<GamesPage />} />
            <Route path="/games/:id" element={<GamePage />} />
            <Route path="/games/:id/results" element={<GameResultsPage />} />
            <Route path="/games/:id/lobby" element={<SoloLobbyPage />} />
            <Route path="/profile" element={<ProfilePage />} />
          </Route>
          <Route element={<AppLayout />}>
            <Route path="/games/join/:token" element={<GameLobbyPage />} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
