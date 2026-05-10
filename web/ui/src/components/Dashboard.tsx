import React, { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getSystemInfo, getLibraries } from '../api.ts'

const styles: Record<string, React.CSSProperties> = {
  container: { padding: '24px' },
  header: { fontSize: '1.4rem', fontWeight: 700, marginBottom: '20px' },
  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
    gap: '16px',
  },
  card: {
    background: 'var(--bg-secondary)',
    border: '1px solid var(--border)',
    borderRadius: '10px',
    padding: '18px',
  },
  cardTitle: { fontWeight: 600, marginBottom: '8px', fontSize: '1rem' },
  cardValue: { color: 'var(--text-secondary)', fontSize: '0.9rem' },
  section: { marginTop: '28px' },
  sectionTitle: { fontSize: '1.1rem', fontWeight: 600, marginBottom: '12px' },
  libraryLink: {
    display: 'block',
    padding: '14px',
    borderRadius: '8px',
    background: 'var(--bg-tertiary)',
    marginBottom: '10px',
    border: '1px solid var(--border)',
  },
}

export default function Dashboard() {
  const [info, setInfo] = useState<any>(null)
  const [libraries, setLibraries] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.all([getSystemInfo(), getLibraries()])
      .then(([i, l]) => {
        setInfo(i)
        setLibraries(l)
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <div style={styles.container}>Loading...</div>

  return (
    <div style={styles.container}>
      <h2 style={styles.header}>Dashboard</h2>
      <div style={styles.grid}>
        <div style={styles.card}>
          <div style={styles.cardTitle}>Version</div>
          <div style={styles.cardValue}>{info?.version || 'N/A'}</div>
        </div>
        <div style={styles.card}>
          <div style={styles.cardTitle}>Uptime</div>
          <div style={styles.cardValue}>{info?.uptime || 'N/A'}</div>
        </div>
        <div style={styles.card}>
          <div style={styles.cardTitle}>Libraries</div>
          <div style={styles.cardValue}>{info?.libraries_count ?? libraries.length}</div>
        </div>
        <div style={styles.card}>
          <div style={styles.cardTitle}>Items</div>
          <div style={styles.cardValue}>{info?.items_count ?? 'N/A'}</div>
        </div>
      </div>

      <div style={styles.section}>
        <h3 style={styles.sectionTitle}>Libraries</h3>
        {libraries.map((lib) => (
          <Link key={lib.id} to={`/libraries/${lib.id}`} style={styles.libraryLink}>
            <strong>{lib.name}</strong>
            <div style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', marginTop: '4px' }}>
              {lib.type} · {lib.item_count} items
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}
