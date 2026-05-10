import React, { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'
import { login as apiLogin } from '../api.ts'

const styles: Record<string, React.CSSProperties> = {
  container: {
    minHeight: '100vh',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: 'var(--bg-primary)',
  },
  card: {
    width: '100%',
    maxWidth: '360px',
    padding: '32px',
    borderRadius: '12px',
    background: 'var(--bg-secondary)',
    border: '1px solid var(--border)',
  },
  title: {
    fontSize: '1.5rem',
    fontWeight: 700,
    marginBottom: '24px',
    textAlign: 'center',
    color: 'var(--text-primary)',
  },
  input: {
    width: '100%',
    padding: '12px 14px',
    marginBottom: '14px',
    borderRadius: '8px',
    border: '1px solid var(--border)',
    background: 'var(--bg-tertiary)',
    color: 'var(--text-primary)',
    fontSize: '0.95rem',
  },
  button: {
    width: '100%',
    padding: '12px',
    borderRadius: '8px',
    background: 'var(--accent)',
    color: '#fff',
    fontWeight: 600,
    fontSize: '0.95rem',
    marginTop: '8px',
  },
  error: {
    color: 'var(--error)',
    fontSize: '0.85rem',
    marginBottom: '10px',
    textAlign: 'center',
  },
}

export default function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const { login } = useAuth()
  const navigate = useNavigate()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await apiLogin({ username, password })
      login(res.token)
      navigate('/', { replace: true })
    } catch (err: any) {
      setError(err.message || 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={styles.container}>
      <div style={styles.card}>
        <h1 style={styles.title}>AetherStream</h1>
        {error && <div style={styles.error}>{error}</div>}
        <form onSubmit={handleSubmit}>
          <input
            style={styles.input}
            type="text"
            placeholder="Username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
          />
          <input
            style={styles.input}
            type="password"
            placeholder="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
          <button style={styles.button} type="submit" disabled={loading}>
            {loading ? 'Signing in...' : 'Sign In'}
          </button>
        </form>
      </div>
    </div>
  )
}
