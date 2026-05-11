import React, { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'
import { login as apiLogin } from '../api.ts'
import { C, R, T } from '../design.ts'

export default function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const { login } = useAuth()
  const navigate = useNavigate()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(''); setLoading(true)
    try {
      const res = await apiLogin({ username, password })
      login(res.token)
      navigate('/', { replace: true })
    } catch (err: any) {
      setError(err.message || 'Identifiants invalides')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: C.bg, padding: 16 }}>
      <div style={{ width: '100%', maxWidth: 380, background: C.surface, borderRadius: R.xl, border: `1px solid ${C.border}`, padding: '36px 32px', boxShadow: '0 24px 80px rgba(0,0,0,0.5)' }}>
        <div style={{ textAlign: 'center', marginBottom: 28 }}>
          <div style={{ fontSize: T.xl, fontWeight: 700, letterSpacing: '-0.01em', background: 'linear-gradient(135deg, #a78bfa, #7c3aed)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent', marginBottom: 4 }}>
            AetherStream
          </div>
          <div style={{ color: C.text2, fontSize: T.sm }}>Connexion à votre médiathèque</div>
        </div>

        {error && (
          <div style={{ background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', borderRadius: R.sm, padding: '10px 12px', color: '#ef4444', fontSize: T.sm, marginBottom: 16 }}>
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <AuthField label="Identifiant">
            <AuthInput type="text" placeholder="Nom d'utilisateur" value={username} autoFocus onChange={(e) => setUsername(e.target.value)} required />
          </AuthField>
          <AuthField label="Mot de passe">
            <AuthInput type="password" placeholder="••••••••" value={password} onChange={(e) => setPassword(e.target.value)} required />
          </AuthField>
          <button type="submit" disabled={loading} style={{ marginTop: 4, padding: '11px', borderRadius: R.md, background: loading ? C.surf3 : C.accent, color: '#fff', fontWeight: 600, fontSize: T.base, transition: 'background 0.15s', cursor: loading ? 'not-allowed' : 'pointer' }}>
            {loading ? 'Connexion…' : 'Se connecter'}
          </button>
        </form>

        <p style={{ textAlign: 'center', color: C.text2, fontSize: T.sm, marginTop: 20 }}>
          Pas de compte ?{' '}
          <Link to="/register" style={{ color: C.accent, fontWeight: 500 }}>S'inscrire</Link>
        </p>
      </div>
    </div>
  )
}

function AuthField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label style={{ display: 'block', color: C.text2, fontSize: T.xs, fontWeight: 600, marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.06em' }}>{label}</label>
      {children}
    </div>
  )
}

function AuthInput(props: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input {...props} style={{ width: '100%', padding: '10px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.base, transition: 'border-color 0.15s', ...props.style }}
      onFocus={(e) => { e.target.style.borderColor = 'rgba(124,58,237,0.6)' }}
      onBlur={(e) => { e.target.style.borderColor = C.border }}
    />
  )
}
