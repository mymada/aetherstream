import { useState } from 'react'
import { Link } from 'react-router-dom'
import { getThumbnailUrl } from '../api.ts'
import { C, R, T } from '../design.ts'

type Item = {
  id: string
  title: string
  year?: number
  duration?: number
  thumbnail?: string
}

function pad2(n: number) { return String(n).padStart(2, '0') }
function fmtDuration(secs: number): string {
  const h = Math.floor(secs / 3600)
  const m = Math.floor((secs % 3600) / 60)
  return h > 0 ? `${h}h${pad2(m)}` : `${m} min`
}

export default function MediaCard({ item }: { item: Item }) {
  const [hover, setHover] = useState(false)
  const [imgOk, setImgOk] = useState(true)

  return (
    <Link
      to={`/player/${item.id}`}
      style={{ display: 'block', textDecoration: 'none', color: 'inherit' }}
    >
      <div
        onMouseEnter={() => setHover(true)}
        onMouseLeave={() => setHover(false)}
        style={{
          position: 'relative',
          aspectRatio: '2/3',
          borderRadius: R.md,
          overflow: 'hidden',
          background: C.surf2,
          border: `1px solid ${C.border}`,
          cursor: 'pointer',
          transform: hover ? 'scale(1.04) translateY(-2px)' : 'scale(1)',
          transition: 'transform 0.2s ease, box-shadow 0.2s ease',
          boxShadow: hover ? '0 12px 40px rgba(0,0,0,0.6)' : '0 2px 8px rgba(0,0,0,0.3)',
        }}
      >
        {/* Poster image */}
        {item.thumbnail && imgOk ? (
          <img
            src={getThumbnailUrl(item.id)}
            alt={item.title}
            onError={() => setImgOk(false)}
            style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover' }}
          />
        ) : (
          /* Fallback gradient with title initial */
          <div style={{
            position: 'absolute', inset: 0,
            background: `linear-gradient(135deg, ${C.surf3} 0%, rgba(124,58,237,0.25) 100%)`,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>
            <span style={{ fontSize: '2.5rem', opacity: 0.25, fontWeight: 700, userSelect: 'none' }}>
              {item.title.charAt(0).toUpperCase()}
            </span>
          </div>
        )}

        {/* Bottom gradient + meta */}
        <div style={{
          position: 'absolute', bottom: 0, left: 0, right: 0,
          background: 'linear-gradient(to top, rgba(0,0,0,0.95) 0%, rgba(0,0,0,0.4) 55%, transparent 100%)',
          padding: '28px 10px 10px',
        }}>
          <div style={{
            color: '#fff', fontSize: T.xs, fontWeight: 600,
            lineHeight: 1.3, overflow: 'hidden',
            display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical',
          }}>
            {item.title}
          </div>
          <div style={{ display: 'flex', gap: 6, marginTop: 4, alignItems: 'center' }}>
            {item.year && (
              <span style={{ color: 'rgba(255,255,255,0.45)', fontSize: '0.7rem' }}>{item.year}</span>
            )}
            {item.duration && (
              <span style={{ color: 'rgba(255,255,255,0.35)', fontSize: '0.7rem' }}>
                {item.year ? '·' : ''} {fmtDuration(item.duration)}
              </span>
            )}
          </div>
        </div>

        {/* Play overlay */}
        <div style={{
          position: 'absolute', inset: 0,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          background: hover ? 'rgba(0,0,0,0.28)' : 'rgba(0,0,0,0)',
          transition: 'background 0.2s',
        }}>
          <div style={{
            width: 46, height: 46, borderRadius: R.full,
            background: hover ? 'rgba(255,255,255,0.92)' : 'rgba(255,255,255,0)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            transform: hover ? 'scale(1)' : 'scale(0.7)',
            opacity: hover ? 1 : 0,
            transition: 'all 0.2s ease',
          }}>
            <span style={{ color: '#000', fontSize: '1rem', marginLeft: 3 }}>▶</span>
          </div>
        </div>
      </div>
    </Link>
  )
}
