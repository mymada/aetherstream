import React, { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { getLibraries, getItems } from '../api.ts'
import MediaCard from './MediaCard.tsx'
import { C, R, T } from '../design.ts'

type Library = { id: string; name: string; type: string; item_count: number }
type Item = { id: string; title: string; year?: number; duration?: number; thumbnail?: string }

export default function LibraryBrowser() {
  const { libraryId } = useParams()
  const navigate = useNavigate()
  const [libraries, setLibraries] = useState<Library[]>([])
  const [items, setItems] = useState<Item[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')

  useEffect(() => {
    getLibraries().then(setLibraries).catch(() => {})
  }, [])

  useEffect(() => {
    setLoading(true)
    setSearch('')
    getItems(libraryId).then(setItems).catch(() => {}).finally(() => setLoading(false))
  }, [libraryId])

  const filtered = search.trim()
    ? items.filter((i) => i.title.toLowerCase().includes(search.toLowerCase()))
    : items

  const activeLib = libraries.find((l) => l.id === libraryId)

  return (
    <div style={{ padding: '28px', minHeight: '100%' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between', marginBottom: 20, flexWrap: 'wrap', gap: 12 }}>
        <div>
          <h1 style={{ fontSize: T['2xl'], fontWeight: 700, letterSpacing: '-0.02em' }}>
            {activeLib?.name ?? 'Toutes les bibliothèques'}
          </h1>
          {!loading && (
            <p style={{ color: C.text2, fontSize: T.sm, marginTop: 3 }}>
              {filtered.length} {filtered.length === 1 ? 'élément' : 'éléments'}
            </p>
          )}
        </div>

        {/* Library tabs */}
        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
          <TabBtn
            active={!libraryId}
            onClick={() => navigate('/libraries')}
          >Tout</TabBtn>
          {libraries.map((lib) => (
            <TabBtn
              key={lib.id}
              active={libraryId === lib.id}
              onClick={() => navigate(`/libraries/${lib.id}`)}
            >
              {lib.name}
            </TabBtn>
          ))}
        </div>
      </div>

      {/* Search */}
      <div style={{ position: 'relative', marginBottom: 24, maxWidth: 320 }}>
        <span style={{
          position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)',
          color: C.text3, fontSize: '0.9rem', pointerEvents: 'none',
        }}>⌕</span>
        <input
          type="search"
          placeholder="Rechercher…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          style={{
            width: '100%', padding: '9px 12px 9px 34px',
            background: C.surface, border: `1px solid ${C.border}`,
            borderRadius: R.md, color: C.text, fontSize: T.sm,
            transition: 'border-color 0.15s',
          }}
          onFocus={(e) => { e.target.style.borderColor = 'rgba(124,58,237,0.5)' }}
          onBlur={(e) => { e.target.style.borderColor = C.border }}
        />
      </div>

      {/* Grid */}
      {loading ? (
        <div style={{ color: C.text2, fontSize: T.base }}>Chargement…</div>
      ) : filtered.length === 0 ? (
        <div style={{
          textAlign: 'center', padding: '60px 20px', color: C.text2,
        }}>
          <div style={{ fontSize: '2rem', marginBottom: 12 }}>📭</div>
          <div style={{ fontSize: T.base }}>
            {search ? 'Aucun résultat pour cette recherche.' : 'Aucun élément dans cette bibliothèque.'}
          </div>
        </div>
      ) : (
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(148px, 1fr))',
          gap: 16,
        }}>
          {filtered.map((item) => (
            <MediaCard key={item.id} item={item} />
          ))}
        </div>
      )}
    </div>
  )
}

function TabBtn({ children, active, onClick }: {
  children: React.ReactNode; active: boolean; onClick: () => void
}) {
  return (
    <button
      onClick={onClick}
      style={{
        padding: '6px 14px', borderRadius: R.full,
        background: active ? C.accent : C.surface,
        color: active ? '#fff' : C.text2,
        border: `1px solid ${active ? 'transparent' : C.border}`,
        fontSize: T.sm, fontWeight: active ? 600 : 400,
        transition: 'all 0.15s',
      }}
      onMouseEnter={(e) => { if (!active) e.currentTarget.style.background = C.surf2 }}
      onMouseLeave={(e) => { if (!active) e.currentTarget.style.background = C.surface }}
    >
      {children}
    </button>
  )
}
