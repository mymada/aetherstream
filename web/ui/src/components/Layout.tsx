import React from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'
import { C, R, T } from '../design.ts'

const NAV = [
  { path: '/',          label: 'Accueil',      icon: '⌂' },
  { path: '/libraries', label: 'Bibliothèques', icon: '▤' },
  { path: '/settings',  label: 'Paramètres',   icon: '⚙' },
]

export default function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const { logout } = useAuth()
  const isPlayer = location.pathname.startsWith('/player/')

  if (isPlayer) return <>{children}</>

  return (
    <div style={{ display: 'flex', height: '100vh', overflow: 'hidden' }}>
      {/* Sidebar */}
      <aside style={{
        width: 210,
        flexShrink: 0,
        background: C.surface,
        borderRight: `1px solid ${C.border}`,
        display: 'flex',
        flexDirection: 'column',
        padding: '0',
        zIndex: 10,
      }}>
        {/* Brand */}
        <div style={{ padding: '22px 20px 18px', borderBottom: `1px solid ${C.border}` }}>
          <div style={{
            fontSize: T.lg, fontWeight: 700, letterSpacing: '-0.01em',
            background: 'linear-gradient(135deg, #a78bfa, #7c3aed)',
            WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent',
          }}>
            AetherStream
          </div>
          <div style={{ color: C.text3, fontSize: T.xs, marginTop: 2 }}>Media Server</div>
        </div>

        {/* Nav */}
        <nav style={{ flex: 1, padding: '12px 10px' }}>
          {NAV.map(({ path, label, icon }) => {
            const active = path === '/'
              ? location.pathname === '/'
              : location.pathname.startsWith(path)
            return (
              <Link key={path} to={path} style={{
                display: 'flex', alignItems: 'center', gap: 10,
                padding: '9px 10px', marginBottom: 2,
                borderRadius: R.md,
                background: active ? 'rgba(124, 58, 237, 0.15)' : 'transparent',
                color: active ? '#a78bfa' : C.text2,
                fontSize: T.sm, fontWeight: active ? 600 : 400,
                transition: 'background 0.15s, color 0.15s',
                textDecoration: 'none',
              }}
                onMouseEnter={(e) => { if (!active) (e.currentTarget.style.background = C.surf2) }}
                onMouseLeave={(e) => { if (!active) (e.currentTarget.style.background = 'transparent') }}
              >
                <span style={{ fontSize: '1rem', opacity: active ? 1 : 0.6, width: 18, textAlign: 'center' }}>{icon}</span>
                {label}
              </Link>
            )
          })}
        </nav>

        {/* Bottom: logout */}
        <div style={{ padding: '12px 10px', borderTop: `1px solid ${C.border}` }}>
          <button
            onClick={logout}
            style={{
              width: '100%', padding: '9px 10px', borderRadius: R.md,
              background: 'transparent', color: C.text2,
              fontSize: T.sm, textAlign: 'left',
              display: 'flex', alignItems: 'center', gap: 10,
              transition: 'background 0.15s, color 0.15s',
            }}
            onMouseEnter={(e) => { e.currentTarget.style.background = C.surf2; e.currentTarget.style.color = C.text }}
            onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = C.text2 }}
          >
            <span style={{ fontSize: '1rem', opacity: 0.5, width: 18, textAlign: 'center' }}>↩</span>
            Déconnexion
          </button>
        </div>
      </aside>

      {/* Main */}
      <main style={{ flex: 1, overflow: 'auto', background: C.bg }}>
        {children}
      </main>
    </div>
  )
}
