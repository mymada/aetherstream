import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { getLibraries, getItems, getThumbnailUrl } from '../api.ts'
import { getAllProgress, type WatchProgress } from '../progress.ts'
import { apiClient } from '../api/client.ts'
import MediaCard from './MediaCard.tsx'
import { C, T } from '../design.ts'

type Library = { id: string; name: string; type: string; path: string; item_count: number }
type Item = { id: string; title: string; year?: number; duration?: number; thumbnail?: string }

function pad2(n: number) { return String(n).padStart(2, '0') }

function Row({ title, items, libraryId, progresses }: {
  title: string
  items: Item[]
  libraryId: string
  progresses?: Record<string, WatchProgress>
}) {
  const trackRef = useRef<HTMLDivElement>(null)
  const [canLeft, setCanLeft] = useState(false)
  const [canRight, setCanRight] = useState(false)

  const updateArrows = () => {
    const el = trackRef.current
    if (!el) return
    setCanLeft(el.scrollLeft > 8)
    setCanRight(el.scrollLeft < el.scrollWidth - el.clientWidth - 8)
  }

  useEffect(() => {
    updateArrows()
    const el = trackRef.current
    el?.addEventListener('scroll', updateArrows, { passive: true })
    return () => el?.removeEventListener('scroll', updateArrows)
  }, [items])

  const scroll = (dir: 'left' | 'right') => {
    const el = trackRef.current
    if (!el) return
    el.scrollBy({ left: dir === 'right' ? 480 : -480, behavior: 'smooth' })
  }

  if (items.length === 0) return null

  return (
    <section style={{ marginBottom: 48 }}>
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        marginBottom: 16, padding: '0 48px',
      }}>
        <h2 style={{ fontSize: T.lg, fontWeight: 600, letterSpacing: '-0.01em' }}>
          {title}
        </h2>
        {libraryId && (
          <Link
            to={`/libraries/${libraryId}`}
            style={{
              color: C.text3, fontSize: T.xs, fontWeight: 500,
              letterSpacing: '0.04em', textTransform: 'uppercase',
              transition: 'color 0.15s',
            }}
            onMouseEnter={(e) => { e.currentTarget.style.color = C.text2 }}
            onMouseLeave={(e) => { e.currentTarget.style.color = C.text3 }}
          >
            Tout voir
          </Link>
        )}
      </div>

      <div style={{ position: 'relative' }}>
        {canLeft && (
          <button
            onClick={() => scroll('left')}
            style={{
              position: 'absolute', left: 0, top: '50%', transform: 'translateY(-50%)',
              zIndex: 2, width: 48, height: '100%',
              background: 'linear-gradient(to right, rgba(10,10,10,0.85) 0%, transparent 100%)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              color: C.text, fontSize: '1.1rem',
              border: 'none', cursor: 'pointer', paddingLeft: 8,
            }}
          >‹</button>
        )}

        <div
          ref={trackRef}
          style={{
            display: 'flex', gap: 10,
            overflowX: 'auto', overflowY: 'visible',
            padding: '8px 48px',
            scrollbarWidth: 'none',
            WebkitOverflowScrolling: 'touch',
          }}
          onLoad={updateArrows}
        >
          {items.map((item) => {
            const prog = progresses?.[item.id]
            const pct = prog && prog.duration > 0 ? prog.position / prog.duration : undefined
            return (
              <div key={item.id} style={{ flexShrink: 0, width: 148 }}>
                <MediaCard item={item} progress={pct} />
              </div>
            )
          })}
        </div>

        {canRight && (
          <button
            onClick={() => scroll('right')}
            style={{
              position: 'absolute', right: 0, top: '50%', transform: 'translateY(-50%)',
              zIndex: 2, width: 48, height: '100%',
              background: 'linear-gradient(to left, rgba(10,10,10,0.85) 0%, transparent 100%)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              color: C.text, fontSize: '1.1rem',
              border: 'none', cursor: 'pointer', paddingRight: 8,
            }}
          >›</button>
        )}
      </div>
    </section>
  )
}

function Hero({ item, progress }: { item: Item; progress?: WatchProgress }) {
  const [imgOk, setImgOk] = useState(true)

  const resumePct = progress && progress.duration > 0
    ? progress.position / progress.duration * 100
    : null
  const resumeLabel = progress
    ? `Reprendre · ${Math.floor(progress.position / 3600)}h${pad2(Math.floor((progress.position % 3600) / 60))}`
    : null

  return (
    <div style={{
      position: 'relative', width: '100%',
      height: 'clamp(380px, 52vw, 600px)',
      marginBottom: 40,
      overflow: 'hidden',
    }}>
      {/* Backdrop */}
      {item.thumbnail && imgOk ? (
        <img
          src={getThumbnailUrl(item.id)}
          alt=""
          onError={() => setImgOk(false)}
          style={{
            position: 'absolute', inset: 0, width: '100%', height: '100%',
            objectFit: 'cover', objectPosition: 'center top',
            filter: 'brightness(0.55)',
          }}
        />
      ) : (
        <div style={{
          position: 'absolute', inset: 0,
          background: 'linear-gradient(135deg, #1a0a2e 0%, #0f0a1a 100%)',
        }} />
      )}

      {/* Gradient overlay */}
      <div style={{
        position: 'absolute', inset: 0,
        background: 'linear-gradient(to top, rgba(10,10,10,1) 0%, rgba(10,10,10,0.3) 50%, rgba(10,10,10,0.05) 100%)',
      }} />
      <div style={{
        position: 'absolute', inset: 0,
        background: 'linear-gradient(to right, rgba(10,10,10,0.7) 0%, transparent 60%)',
      }} />

      {/* Content */}
      <div style={{
        position: 'absolute', bottom: 52, left: 48,
        maxWidth: 520,
        animation: 'fadeIn 0.5s ease',
      }}>
        <h1 style={{
          fontSize: 'clamp(1.6rem, 3.5vw, 2.4rem)',
          fontWeight: 700,
          lineHeight: 1.15,
          letterSpacing: '-0.03em',
          marginBottom: 12,
          textShadow: '0 2px 16px rgba(0,0,0,0.5)',
        }}>
          {item.title}
        </h1>

        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24 }}>
          {item.year && (
            <span style={{ color: C.text2, fontSize: T.sm }}>{item.year}</span>
          )}
          {item.duration && (
            <span style={{ color: C.text3, fontSize: T.sm }}>
              {item.year ? '·' : ''} {Math.floor(item.duration / 3600)}h{pad2(Math.floor((item.duration % 3600) / 60))}
            </span>
          )}
        </div>

        {/* Progress bar on hero */}
        {resumePct != null && (
          <div style={{
            width: 200, height: 3, background: 'rgba(255,255,255,0.2)',
            borderRadius: 2, marginBottom: 16, overflow: 'hidden',
          }}>
            <div style={{
              height: '100%', width: `${resumePct}%`,
              background: '#e50914', borderRadius: 2,
            }} />
          </div>
        )}

        <div style={{ display: 'flex', gap: 10 }}>
          <Link
            to={`/player/${item.id}`}
            style={{
              display: 'inline-flex', alignItems: 'center', gap: 8,
              padding: '11px 24px',
              background: '#fff', color: '#000',
              borderRadius: 6, fontWeight: 700, fontSize: T.sm,
              transition: 'background 0.15s',
            }}
            onMouseEnter={(e) => { e.currentTarget.style.background = 'rgba(255,255,255,0.85)' }}
            onMouseLeave={(e) => { e.currentTarget.style.background = '#fff' }}
          >
            <span style={{ fontSize: '0.75rem' }}>▶</span>
            {resumeLabel ?? 'Lecture'}
          </Link>
          <Link
            to={`/player/${item.id}`}
            style={{
              display: 'inline-flex', alignItems: 'center', gap: 8,
              padding: '11px 24px',
              background: 'rgba(255,255,255,0.12)', color: '#fff',
              borderRadius: 6, fontWeight: 600, fontSize: T.sm,
              backdropFilter: 'blur(8px)',
              transition: 'background 0.15s',
            }}
            onMouseEnter={(e) => { e.currentTarget.style.background = 'rgba(255,255,255,0.2)' }}
            onMouseLeave={(e) => { e.currentTarget.style.background = 'rgba(255,255,255,0.12)' }}
          >
            Plus d'infos
          </Link>
        </div>
      </div>
    </div>
  )
}

export default function Dashboard() {
  const [libraries, setLibraries] = useState<Library[]>([])
  const [libraryItems, setLibraryItems] = useState<Record<string, Item[]>>({})
  const [loading, setLoading] = useState(true)
  const [allProgress, setAllProgress] = useState<Record<string, WatchProgress>>({})
  const [continueWatching, setContinueWatching] = useState<any[]>([])

  useEffect(() => {
    setAllProgress(getAllProgress())
    // Fetch continue watching from API
    apiClient.getContinueWatching().then((items) => {
      setContinueWatching(items)
    }).catch(() => {})
    getLibraries().then(async (libs) => {
      setLibraries(libs)
      const entries = await Promise.all(
        libs.map(async (lib) => {
          const items = await getItems(lib.id).catch(() => [])
          return [lib.id, items] as [string, Item[]]
        })
      )
      setLibraryItems(Object.fromEntries(entries))
    }).catch(() => {}).finally(() => setLoading(false))
  }, [])

  const allItems = Object.values(libraryItems).flat()
  const featuredItem = allItems.find((i) => i.thumbnail) ?? allItems[0]

  // "Continue watching" — merge API + localStorage
  const continueItems = continueWatching.length > 0
    ? continueWatching.map((cw) => ({
        id: cw.id,
        title: cw.name,
        thumbnail: cw.posterUrl,
        duration: cw.duration,
      }))
    : allItems
      .filter((i) => allProgress[i.id] != null)
      .sort((a, b) => (allProgress[b.id]?.savedAt ?? 0) - (allProgress[a.id]?.savedAt ?? 0))

  if (loading) {
    return (
      <div style={{
        paddingTop: 'calc(var(--nav-h) + 80px)',
        textAlign: 'center', color: C.text3, fontSize: T.sm,
      }}>
        Chargement…
      </div>
    )
  }

  if (allItems.length === 0) {
    return (
      <div style={{
        paddingTop: 'calc(var(--nav-h) + 80px)',
        textAlign: 'center', color: C.text2,
        padding: '120px 20px',
      }}>
        <div style={{ fontSize: T['2xl'], fontWeight: 700, marginBottom: 8 }}>Médiathèque vide</div>
        <p style={{ color: C.text3, fontSize: T.sm }}>Ajoutez des bibliothèques dans les paramètres pour commencer.</p>
      </div>
    )
  }

  return (
    <div style={{ paddingBottom: 64 }}>
      {featuredItem && (
        <div style={{ paddingTop: 'var(--nav-h)' }}>
          <Hero item={featuredItem} progress={allProgress[featuredItem.id]} />
        </div>
      )}

      {!featuredItem && <div style={{ height: 'calc(var(--nav-h) + 32px)' }} />}

      {/* Continue watching row */}
      {continueItems.length > 0 && (
        <Row
          title="Continuer à regarder"
          items={continueItems}
          libraryId=""
          progresses={allProgress}
        />
      )}

      {/* Library rows */}
      {libraries.map((lib) => {
        const items = libraryItems[lib.id] ?? []
        return (
          <Row
            key={lib.id}
            title={lib.name}
            items={items}
            libraryId={lib.id}
          />
        )
      })}
    </div>
  )
}
