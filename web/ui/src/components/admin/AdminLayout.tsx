import { Link, useLocation } from 'react-router-dom'
import { C, T } from '../../design.ts'

const NAV = [
  { path: '/admin', label: 'Vue d\'ensemble', icon: '◈' },
  { path: '/admin/users', label: 'Utilisateurs', icon: '◉' },
  { path: '/admin/libraries', label: 'Bibliothèques', icon: '▣' },
  { path: '/admin/activity', label: 'Activité', icon: '◆' },
]

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const location = useLocation()

  return (
    <div style={{ display: 'flex', minHeight: '100vh', background: C.bg }}>
      {/* Sidebar */}
      <aside style={{
        width: 240, flexShrink: 0,
        background: C.surface, borderRight: `1px solid ${C.border}`,
        padding: '24px 0',
      }}>
        <div style={{ padding: '0 20px 20px', borderBottom: `1px solid ${C.border}` }}>
          <div style={{ fontSize: T.sm, fontWeight: 700, color: C.text }}>
            Administration
          </div>
          <div style={{ fontSize: '0.7rem', color: C.text3, marginTop: 2 }}>
            AetherStream
          </div>
        </div>

        <nav style={{ padding: '12px 0' }}>
          {NAV.map(({ path, label, icon }) => {
            const active = location.pathname === path
            return (
              <Link
                key={path}
                to={path}
                style={{
                  display: 'flex', alignItems: 'center', gap: 10,
                  padding: '10px 20px',
                  fontSize: T.sm,
                  color: active ? C.text : C.text2,
                  background: active ? 'rgba(124,58,237,0.12)' : 'transparent',
                  borderLeft: active ? '3px solid #7c3aed' : '3px solid transparent',
                  transition: 'all 0.15s',
                  textDecoration: 'none',
                }}
                onMouseEnter={(e) => {
                  if (!active) e.currentTarget.style.background = 'rgba(255,255,255,0.04)'
                }}
                onMouseLeave={(e) => {
                  if (!active) e.currentTarget.style.background = 'transparent'
                }}
              >
                <span style={{ fontSize: '0.8rem', opacity: 0.7 }}>{icon}</span>
                {label}
              </Link>
            )
          })}
        </nav>

        <div style={{ marginTop: 'auto', padding: '20px', borderTop: `1px solid ${C.border}` }}>
          <Link
            to="/"
            style={{
              display: 'flex', alignItems: 'center', gap: 8,
              fontSize: T.xs, color: C.text3,
              textDecoration: 'none',
            }}
          >
            ← Retour à l'accueil
          </Link>
        </div>
      </aside>

      {/* Content */}
      <main style={{ flex: 1, padding: '32px 40px', overflow: 'auto' }}>
        {children}
      </main>
    </div>
  )
}
