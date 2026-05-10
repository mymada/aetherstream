import React from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'

const navItems = [
  { path: '/', label: 'Dashboard' },
  { path: '/libraries', label: 'Libraries' },
  { path: '/settings', label: 'Settings' },
]

const styles: Record<string, React.CSSProperties> = {
  layout: { display: 'flex', height: '100vh', overflow: 'hidden' },
  sidebar: {
    width: '220px',
    background: 'var(--bg-secondary)',
    borderRight: '1px solid var(--border)',
    display: 'flex',
    flexDirection: 'column',
    padding: '16px 0',
  },
  brand: {
    padding: '0 20px 20px',
    fontSize: '1.15rem',
    fontWeight: 700,
    color: 'var(--accent)',
  },
  navItem: {
    display: 'block',
    padding: '10px 20px',
    color: 'var(--text-secondary)',
    fontSize: '0.95rem',
    fontWeight: 500,
    borderLeft: '3px solid transparent',
  },
  navItemActive: {
    color: 'var(--text-primary)',
    background: 'var(--bg-tertiary)',
    borderLeftColor: 'var(--accent)',
  },
  main: { flex: 1, overflow: 'auto' },
}

export default function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const { logout } = useAuth()

  return (
    <div style={styles.layout}>
      <aside style={styles.sidebar}>
        <div style={styles.brand}>AetherStream</div>
        <nav>
          {navItems.map((item) => {
            const active = location.pathname === item.path || location.pathname.startsWith(item.path + '/')
            return (
              <Link
                key={item.path}
                to={item.path}
                style={{ ...styles.navItem, ...(active ? styles.navItemActive : {}) }}
              >
                {item.label}
              </Link>
            )
          })}
        </nav>
        <div style={{ marginTop: 'auto', padding: '0 20px' }}>
          <button
            onClick={logout}
            style={{
              width: '100%',
              padding: '10px',
              borderRadius: '8px',
              background: 'var(--bg-tertiary)',
              color: 'var(--text-secondary)',
              border: '1px solid var(--border)',
              fontSize: '0.9rem',
            }}
          >
            Logout
          </button>
        </div>
      </aside>
      <main style={styles.main}>{children}</main>
    </div>
  )
}
