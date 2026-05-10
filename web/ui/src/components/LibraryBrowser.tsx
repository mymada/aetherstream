import React, { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { getLibraries, getItems, getThumbnailUrl } from '../api.ts'

const styles: Record<string, React.CSSProperties> = {
  container: { padding: '24px' },
  header: { fontSize: '1.3rem', fontWeight: 700, marginBottom: '16px' },
  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))',
    gap: '14px',
  },
  item: {
    background: 'var(--bg-secondary)',
    border: '1px solid var(--border)',
    borderRadius: '10px',
    overflow: 'hidden',
    cursor: 'pointer',
  },
  thumb: {
    width: '100%',
    aspectRatio: '16/9',
    background: 'var(--bg-tertiary)',
    objectFit: 'cover',
  },
  meta: { padding: '10px 12px' },
  title: { fontWeight: 600, fontSize: '0.95rem', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  sub: { color: 'var(--text-secondary)', fontSize: '0.8rem', marginTop: '4px' },
  select: {
    padding: '10px 14px',
    borderRadius: '8px',
    border: '1px solid var(--border)',
    background: 'var(--bg-tertiary)',
    color: 'var(--text-primary)',
    marginBottom: '16px',
    fontSize: '0.95rem',
  },
}

export default function LibraryBrowser() {
  const { libraryId } = useParams()
  const [libraries, setLibraries] = useState<any[]>([])
  const [items, setItems] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    getLibraries().then(setLibraries).catch(() => {})
  }, [])

  useEffect(() => {
    setLoading(true)
    getItems(libraryId)
      .then(setItems)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [libraryId])

  return (
    <div style={styles.container}>
      <h2 style={styles.header}>Library Browser</h2>
      <select
        style={styles.select}
        value={libraryId || ''}
        onChange={(e) => {
          const id = e.target.value
          window.location.href = id ? `/libraries/${id}` : '/libraries'
        }}
      >
        <option value="">All Libraries</option>
        {libraries.map((l) => (
          <option key={l.id} value={l.id}>
            {l.name}
          </option>
        ))}
      </select>

      {loading ? (
        <div>Loading...</div>
      ) : items.length === 0 ? (
        <div style={{ color: 'var(--text-secondary)' }}>No items found.</div>
      ) : (
        <div style={styles.grid}>
          {items.map((item) => (
            <Link key={item.id} to={`/player/${item.id}`} style={{ textDecoration: 'none', color: 'inherit' }}>
              <div style={styles.item}>
                <img
                  src={item.thumbnail ? getThumbnailUrl(item.id) : ''}
                  alt={item.title}
                  style={styles.thumb}
                  onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
                />
                <div style={styles.meta}>
                  <div style={styles.title}>{item.title}</div>
                  <div style={styles.sub}>
                    {item.year ? item.year : ''} {item.duration ? `· ${Math.round(item.duration / 60)}m` : ''}
                  </div>
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}
