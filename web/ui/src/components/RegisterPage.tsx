import React, { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { register } from '../api.ts'

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: '100vh',
    background: 'var(--bg-primary)',
  },
  form: {
    display: 'flex',
    flexDirection: 'column',
    gap: '12px',
    width: '320px',
    padding: '24px',
    borderRadius: '12px',
    background: 'var(--bg-secondary)',
    border: '1px solid var(--border)',
  },
  input: {
    padding: '10px 14px',
    borderRadius: '8px',
    border: '1px solid var(--border)',
    background: 'var(--bg-primary)',
    color: 'var(--text-primary)',
    fontSize: '1rem',
  },
  button: {
    padding: '10px',
    borderRadius: '8px',
    border: 'none',
    background: 'var(--accent)',
    color: '#fff',
    fontWeight: 600,
    cursor: 'pointer',
    fontSize: '1rem',
  },
  error: {
    color: 'var(--danger)',
    fontSize: '0.85rem',
    textAlign: 'center',
  },
  link: {
    color: 'var(--accent)',
    textAlign: 'center',
    fontSize: '0.9rem',
    marginTop: '8px',
  },
}

export default function RegisterPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await register({ username, password })
      navigate('/login')
    } catch (err: any) {
      setError(err.message || 'Registration failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={styles.container}>
      <form style={styles.form} onSubmit={handleSubmit}>
        <h2 style={{ margin: '0 0 8px', textAlign: 'center' }}>Create Account</h2>
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
          placeholder="Password (min 8 chars)"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          minLength={8}
        />
        {error && <div style={styles.error}>{error}</div>}
        <button style={styles.button} type="submit" disabled={loading}>
          {loading ? 'Creating...' : 'Create Account'}
        </button>
        <Link to="/login" style={styles.link}>Already have an account? Sign in</Link>
      </form>
    </div>
  )
}
