import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuth } from './hooks/useAuth.ts'
import LoginPage from './components/LoginPage.tsx'
import RegisterPage from './components/RegisterPage.tsx'
import Dashboard from './components/Dashboard.tsx'
import LibraryBrowser from './components/LibraryBrowser.tsx'
import MediaPlayerEnhanced from './components/MediaPlayerEnhanced.tsx'
import Settings from './components/Settings.tsx'
import Layout from './components/Layout.tsx'
import TVPage from './pages/TV/TVPage.tsx'

import AdminLayout from './components/admin/AdminLayout.tsx'
import AdminDashboard from './components/admin/AdminDashboard.tsx'
import AdminUsers from './components/admin/AdminUsers.tsx'
import AdminLibraries from './components/admin/AdminLibraries.tsx'
import AdminActivity from './components/admin/AdminActivity.tsx'

function App() {
  const { token } = useAuth()

  if (!token) {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    )
  }

  return (
    <Routes>
      {/* Admin routes with sidebar layout */}
      <Route path="/admin" element={<AdminLayout><AdminDashboard /></AdminLayout>} />
      <Route path="/admin/users" element={<AdminLayout><AdminUsers /></AdminLayout>} />
      <Route path="/admin/libraries" element={<AdminLayout><AdminLibraries /></AdminLayout>} />
      <Route path="/admin/activity" element={<AdminLayout><AdminActivity /></AdminLayout>} />

      {/* Main app routes */}
      <Route path="/*" element={
        <Layout>
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/libraries" element={<LibraryBrowser />} />
            <Route path="/libraries/:libraryId" element={<LibraryBrowser />} />
            <Route path="/player/:id" element={<MediaPlayerEnhanced />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="/tv" element={<TVPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Layout>
      } />
    </Routes>
  )
}

export default App
