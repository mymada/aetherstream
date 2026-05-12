import React, { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'
import { C, T } from '../design.ts'

const NAV = [
  { path: '/',          label: 'Accueil' },
  { path: '/libraries', label: 'Bibliothèque' },
  { path: '/settings',  label: 'Paramètres' },
]

export default function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const { logout } = useAuth()
  const [scrolled, setScrolled] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)
  const isPlayer = location.pathname.startsWith('/player/')

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 10)
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  if (isPlayer) return <>{children}</>

  return (
    <div style={{ minHeight: '100vh', background: C.bg }}>
      {/* Top navbar */}
      <header style={{
        position: 'fixed', top: 0, left: 0, right: 0, zIndex: 100,
        height: 'var(--nav-h)',
        background: scrolled
          ? 'rgba(10,10,10,0.96)'
          : 'linear-gradient(to bottom, rgba(0,0,0,0.75) 0%, transparent 100%)',
        backdropFilter: scrolled ? 'blur(12px)' : 'none',
        borderBottom: scrolled ? '1px solid rgba(255,255,255,0.06)' : 'none',
        transition: 'background 0.3s, backdrop-filter 0.3s, border-color 0.3s',
        display: 'flex',
        alignItems: 'center',
        padding: '0 32px',
        gap: 40,
      }}>
        {/* Brand */}
        <Link to="/" style={{ flexShrink: 0, display: 'flex', alignItems: 'center' }}>
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" style={{ marginRight: 8 }}>
            <path d="M5 3l14 9-14 9V3z" fill="url(#logo-grad)" />
            <defs>
              <linearGradient id="logo-grad" x1="5" y1="3" x2="19" y2="21" gradientUnits="userSpaceOnUse">
                <stop stopColor="#a78bfa" />
                <stop offset="1" stopColor="#7c3aed" />
              </linearGradient>
            </defs>
          </svg>
          <span style={{
            fontSize: '1.1rem', fontWeight: 700, letterSpacing: '-0.02em',
            background: 'linear-gradient(135deg, #d4bbff, #a78bfa)',
            WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent',
          }}>
            AetherStream
          </span>
        </Link>

        {/* Nav links */}
        <nav style={{ display: 'flex', alignItems: 'center', gap: 4, flex: 1 }}>
          {NAV.map(({ path, label }) => {
            const active = path === '/'
              ? location.pathname === '/'
              : location.pathname.startsWith(path)
            return (
              <Link
                key={path}
                to={path}
                style={{
                  padding: '6px 14px',
                  borderRadius: 6,
                  fontSize: T.sm,
                  fontWeight: active ? 600 : 400,
                  color: active ? C.text : C.text2,
                  background: active ? 'rgba(255,255,255,0.08)' : 'transparent',
                  transition: 'color 0.15s, background 0.15s',
                  letterSpacing: '0.01em',
                }}
                onMouseEnter={(e) => { if (!active) e.currentTarget.style.color = C.text }}
                onMouseLeave={(e) => { if (!active) e.currentTarget.style.color = C.text2 }}
              >
                {label}
              </Link>
            )
          })}
        </nav>

        {/* User menu */}
        <div style={{ position: 'relative', flexShrink: 0 }}>
          <button
            onClick={() => setMenuOpen(!menuOpen)}
            style={{
              width: 36, height: 36, borderRadius: '50%',
              background: 'linear-gradient(135deg, #6d28d9, #4c1d95)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              color: '#fff', fontSize: T.sm, fontWeight: 700,
              border: '2px solid rgba(167,139,250,0.3)',
              transition: 'border-color 0.15s',
              flexShrink: 0,
            }}
            onMouseEnter={(e) => { e.currentTarget.style.borderColor = 'rgba(167,139,250,0.7)' }}
            onMouseLeave={(e) => { e.currentTarget.style.borderColor = 'rgba(167,139,250,0.3)' }}
          >
            A
          </button>
          {menuOpen && (
            <>
              <div
                style={{ position: 'fixed', inset: 0, zIndex: 1 }}
                onClick={() => setMenuOpen(false)}
              />
              <div style={{
                position: 'absolute', top: 44, right: 0, zIndex: 2,
                background: C.surf2, border: '1px solid rgba(255,255,255,0.1)',
                borderRadius: 10, padding: 6, minWidth: 180,
                boxShadow: '0 8px 32px rgba(0,0,0,0.6)',
                animation: 'fadeIn 0.15s ease',
              }}>
                <Link
                  to="/admin"
                  onClick={() => setMenuOpen(false)}
                  style={{
                    display: 'block', width: '100%', padding: '9px 14px', borderRadius: 6,
                    background: 'transparent', color: C.text2,
                    fontSize: T.sm, textAlign: 'left',
                    transition: 'background 0.15s, color 0.15s',
                    textDecoration: 'none',
                  }}
                  onMouseEnter={(e) => { e.currentTarget.style.background = C.surf3; e.currentTarget.style.color = C.text }}
                  onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = C.text2 }}
                >
                  Administration
                </Link>
                <button
                  onClick={() => { setMenuOpen(false); logout() }}
                  style={{
                    width: '100%', padding: '9px 14px', borderRadius: 6,
                    background: 'transparent', color: C.text2,
                    fontSize: T.sm, textAlign: 'left', display: 'block',
                    transition: 'background 0.15s, color 0.15s',
                  }}
                  onMouseEnter={(e) => { e.currentTarget.style.background = C.surf3; e.currentTarget.style.color = C.text }}
                  onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = C.text2 }}
                >
                  Se déconnecter
                </button>
              </div>
            </>
          )}
        </div>
      </header>

      {/* Page content */}
      <main>
        {children}
      </main>
    </div>
  )
}
