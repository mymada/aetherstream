import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getSystemInfo, getLibraries, getItems } from '../api.ts'
import MediaCard from './MediaCard.tsx'
import { C, R, T } from '../design.ts'

type Library = { id: string; name: string; type: string; path: string; item_count: number }
type Item = { id: string; title: string; year?: number; duration?: number; thumbnail?: string }

function LibraryIcon({ type }: { type: string }) {
  return (
    <span style={{
      fontSize: '1.2rem', opacity: 0.7,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
    }}>
      {type === 'movie' ? '🎬' : type === 'show' ? '📺' : type === 'music' ? '🎵' : '▤'}
    </span>
  )
}

function StatCard({ label, value }: { label: string; value: string | number }) {
  return (
    <div style={{
      background: C.surface, border: `1px solid ${C.border}`,
      borderRadius: R.md, padding: '14px 18px',
    }}>
      <div style={{ color: C.text3, fontSize: T.xs, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: 6 }}>
        {label}
      </div>
      <div style={{ color: C.text, fontSize: T.xl, fontWeight: 700 }}>{value}</div>
    </div>
  )
}

export default function Dashboard() {
  const [info, setInfo] = useState<any>(null)
  const [libraries, setLibraries] = useState<Library[]>([])
  const [recentItems, setRecentItems] = useState<Item[]>([])

  useEffect(() => {
    getSystemInfo().then(setInfo).catch(() => {})
    getLibraries().then((libs) => {
      setLibraries(libs)
      if (libs.length > 0) {
        getItems(libs[0].id).then((items) => setRecentItems(items.slice(0, 12))).catch(() => {})
      }
    }).catch(() => {})
  }, [])

  return (
    <div style={{ padding: '32px 28px', maxWidth: 1400 }}>
      {/* Header */}
      <div style={{ marginBottom: 28 }}>
        <h1 style={{ fontSize: T['2xl'], fontWeight: 700, letterSpacing: '-0.02em' }}>
          Bonjour 👋
        </h1>
        <p style={{ color: C.text2, fontSize: T.sm, marginTop: 4 }}>
          Votre médiathèque personnelle
        </p>
      </div>

      {/* Stats */}
      {info && (
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))',
          gap: 12, marginBottom: 36,
        }}>
          <StatCard label="Bibliothèques" value={info.libraries_count ?? libraries.length} />
          <StatCard label="Médias" value={info.items_count ?? '—'} />
          <StatCard label="Version" value={info.version ?? '—'} />
          <StatCard label="Uptime" value={info.uptime ?? '—'} />
        </div>
      )}

      {/* Libraries */}
      {libraries.length > 0 && (
        <section style={{ marginBottom: 40 }}>
          <h2 style={{ fontSize: T.lg, fontWeight: 600, marginBottom: 14 }}>Bibliothèques</h2>
          <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
            {libraries.map((lib) => (
              <Link
                key={lib.id}
                to={`/libraries/${lib.id}`}
                style={{
                  display: 'flex', alignItems: 'center', gap: 10,
                  background: C.surface, border: `1px solid ${C.border}`,
                  borderRadius: R.lg, padding: '12px 16px',
                  textDecoration: 'none', color: C.text,
                  transition: 'background 0.15s, border-color 0.15s',
                  minWidth: 160,
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background = C.surf2
                  e.currentTarget.style.borderColor = 'rgba(124,58,237,0.4)'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = C.surface
                  e.currentTarget.style.borderColor = C.border
                }}
              >
                <LibraryIcon type={lib.type} />
                <div>
                  <div style={{ fontWeight: 600, fontSize: T.sm }}>{lib.name}</div>
                  <div style={{ color: C.text2, fontSize: T.xs, marginTop: 1 }}>
                    {lib.item_count} {lib.item_count === 1 ? 'élément' : 'éléments'}
                  </div>
                </div>
              </Link>
            ))}
          </div>
        </section>
      )}

      {/* Recent items */}
      {recentItems.length > 0 && (
        <section>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 14 }}>
            <h2 style={{ fontSize: T.lg, fontWeight: 600 }}>
              {libraries[0]?.name ?? 'Récemment ajoutés'}
            </h2>
            <Link
              to={`/libraries/${libraries[0]?.id ?? ''}`}
              style={{ color: C.accent, fontSize: T.sm, fontWeight: 500 }}
            >
              Voir tout →
            </Link>
          </div>
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))',
            gap: 14,
          }}>
            {recentItems.map((item) => (
              <MediaCard key={item.id} item={item} />
            ))}
          </div>
        </section>
      )}
    </div>
  )
}
