import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { Home, Library, Shield, LogOut } from 'lucide-react'

export default function Layout({ children }: { children?: React.ReactNode }) {
  const { logout, user } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()

  const isActive = (path: string) => location.pathname === path

  return (
    <div>
      <nav className="nav">
        <strong style={{ color: '#66fcf1', marginRight: 12 }}>AetherStream</strong>
        <Link to="/" className={isActive('/') ? 'active' : ''}><Home size={16} /> Dashboard</Link>
        <Link to="/libraries" className={isActive('/libraries') ? 'active' : ''}><Library size={16} /> Libraries</Link>
        {user?.role === 'admin' && (
          <Link to="/admin" className={isActive('/admin') ? 'active' : ''}><Shield size={16} /> Admin</Link>
        )}
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 12, alignItems: 'center' }}>
          <span style={{ fontSize: 12, color: '#888' }}>{user?.user_id}</span>
          <button onClick={() => { logout(); navigate('/login') }} style={{ padding: '4px 8px', fontSize: 12 }}>
            <LogOut size={14} /> Logout
          </button>
        </div>
      </nav>
      <main className="page">{children}</main>
    </div>
  )
}
