import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuth } from './hooks/useAuth.ts'
import LoginPage from './components/LoginPage.tsx'
import Dashboard from './components/Dashboard.tsx'
import LibraryBrowser from './components/LibraryBrowser.tsx'
import MediaPlayer from './components/MediaPlayer.tsx'
import Settings from './components/Settings.tsx'
import Layout from './components/Layout.tsx'
import TVPage from './pages/TV/TVPage.tsx'

function App() {
  const { token } = useAuth()

  if (!token) {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    )
  }

  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/libraries" element={<LibraryBrowser />} />
        <Route path="/libraries/:libraryId" element={<LibraryBrowser />} />
        <Route path="/player/:id" element={<MediaPlayer />} />
        <Route path="/settings" element={<Settings />} />
        <Route path="/tv" element={<TVPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Layout>
  )
}

export default App
