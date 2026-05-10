import React, { useState } from 'react'
import { useAuth } from '../hooks/useAuth.ts'

const styles: Record<string, React.CSSProperties> = {
  container: { padding: '24px' },
  header: { fontSize: '1.3rem', fontWeight: 700, marginBottom: '20px' },
  card: {
    maxWidth: '480px',
    background: 'var(--bg-secondary)',
    border: '1px solid var(--border)',
    borderRadius: '10px',
    padding: '20px',
    marginBottom: '16px',
  },
  row: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: '12px',
  },
  label: { fontWeight: 600 },
  button: {
    padding: '10px 16px',
    borderRadius: '8px',
    background: 'var(--accent)',
    color: '#fff',
    fontWeight: 600,
    fontSize: '0.9rem',
  },
  danger: {
    background: 'var(--error)',
  },
}

export default function Settings() {
  const { logout } = useAuth()
  const [confirm, setConfirm] = useState(false)

  return (
    <div style={styles.container}>
      <h2 style={styles.header}>Settings</h2>
      <div style={styles.card}>
        <div style={styles.row}>
          <span style={styles.label}>Account</span>
        </div>
        <div style={styles.row}>
          <span style={{ color: 'var(--text-secondary)' }}>Sign out of AetherStream</span>
          <button style={{ ...styles.button, ...styles.danger }} onClick={() => {
            if (confirm) {
              logout()
            } else {
              setConfirm(true)
            }
          }}>
            {confirm ? 'Confirm Logout' : 'Logout'}
          </button>
        </div>
        {confirm && (
          <div style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', marginTop: '8px' }}>
            Click again to confirm.
          </div>
        )}
      </div>

      <div style={styles.card}>
        <div style={styles.row}>
          <span style={styles.label}>About</span>
        </div>
        <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
          AetherStream UI v0.1.0
        </div>
      </div>
    </div>
  )
}
