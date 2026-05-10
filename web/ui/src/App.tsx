import { Routes, Route, Navigate, Outlet } from 'react-router-dom'
import { AuthProvider, useAuth } from './context/AuthContext'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import LibraryPage from './pages/LibraryPage'
import PlayerPage from './pages/PlayerPage'
import AdminPage from './pages/AdminPage'

function RequireAuth() {
  const { token } = useAuth()
  return token ? <Layout><Outlet /></Layout> : <Navigate to="/login" replace />
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<RequireAuth />}>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/libraries" element={<LibraryPage />} />
        <Route path="/player/:id" element={<PlayerPage />} />
        <Route path="/admin" element={<AdminPage />} />
      </Route>
    </Routes>
  )
}

export default function App() {
  return (
    <AuthProvider>
      <AppRoutes />
    </AuthProvider>
  )
}
