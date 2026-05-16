import { useEffect, useState, useCallback } from 'react'
import { C, R, T } from '../design.ts'

interface FsEntry {
  name: string
  path: string
  type: string
}

interface Props {
  onSelect: (path: string) => void
  onClose: () => void
  initialPath?: string
}

const token = () => localStorage.getItem('aetherstream_token') ?? ''

async function apiFetch<T>(url: string): Promise<T> {
  const res = await fetch(url, { headers: { Authorization: `Bearer ${token()}` } })
  if (!res.ok) throw new Error(`${res.status}`)
  return res.json() as Promise<T>
}

export default function FolderPickerModal({ onSelect, onClose, initialPath }: Props) {
  const [currentPath, setCurrentPath] = useState<string | null>(null)
  const [entries, setEntries] = useState<FsEntry[]>([])
  const [drives, setDrives] = useState<FsEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<string | null>(initialPath ?? null)

  // Load root drives on mount
  useEffect(() => {
    apiFetch<FsEntry[]>('/api/environment/drives')
      .then((d) => { setDrives(d); setLoading(false) })
      .catch(() => { setError('Impossible de lister les volumes'); setLoading(false) })
  }, [])

  const navigate = useCallback(async (path: string) => {
    setLoading(true)
    setError('')
    try {
      const data = await apiFetch<FsEntry[]>(`/api/environment/directory?path=${encodeURIComponent(path)}`)
      setCurrentPath(path)
      setEntries(data)
      setSelected(path)
    } catch {
      setError('Impossible de lire ce dossier')
    } finally {
      setLoading(false)
    }
  }, [])

  const goUp = useCallback(async () => {
    if (!currentPath || currentPath === '/') { setCurrentPath(null); return }
    try {
      const data = await apiFetch<{ path: string }>(`/api/environment/parent?path=${encodeURIComponent(currentPath)}`)
      if (data.path === currentPath) { setCurrentPath(null); return }
      navigate(data.path)
    } catch { /* silent */ }
  }, [currentPath, navigate])

  // Breadcrumb segments
  const segments = (() => {
    if (!currentPath) return []
    const parts = currentPath.split('/').filter(Boolean)
    return parts.map((seg, i) => ({
      label: seg,
      path: '/' + parts.slice(0, i + 1).join('/'),
    }))
  })()

  return (
    <div style={{
      position: 'fixed', inset: 0, zIndex: 1000,
      background: 'rgba(0,0,0,0.65)', backdropFilter: 'blur(4px)',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
    }} onClick={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <div style={{
        background: C.surface, border: `1px solid ${C.border}`, borderRadius: R.xl,
        width: 560, maxWidth: '95vw', display: 'flex', flexDirection: 'column',
        maxHeight: '80vh', overflow: 'hidden', boxShadow: '0 20px 60px rgba(0,0,0,0.5)',
      }}>
        {/* Header */}
        <div style={{ padding: '16px 20px', borderBottom: `1px solid ${C.border}`, display: 'flex', alignItems: 'center', gap: 12 }}>
          <span style={{ fontSize: '1.2rem' }}>📁</span>
          <span style={{ fontWeight: 600, fontSize: T.base, flex: 1 }}>Sélectionner un dossier</span>
          <button onClick={onClose} style={{ background: 'none', border: 'none', color: C.text3, fontSize: '1.25rem', cursor: 'pointer', lineHeight: 1, padding: '0 4px' }}>✕</button>
        </div>

        {/* Breadcrumb */}
        <div style={{ padding: '10px 20px', borderBottom: `1px solid ${C.border}`, display: 'flex', alignItems: 'center', gap: 4, flexWrap: 'wrap', minHeight: 40 }}>
          <button
            onClick={() => setCurrentPath(null)}
            style={{ background: 'none', border: 'none', color: C.accent, cursor: 'pointer', fontSize: T.xs, padding: '2px 4px', borderRadius: R.sm }}
          >
            Volumes
          </button>
          {segments.map((seg, i) => (
            <span key={seg.path} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ color: C.text3, fontSize: T.xs }}>/</span>
              <button
                onClick={() => navigate(seg.path)}
                style={{
                  background: 'none', border: 'none', cursor: 'pointer', fontSize: T.xs, padding: '2px 4px', borderRadius: R.sm,
                  color: i === segments.length - 1 ? C.text : C.accent,
                  fontWeight: i === segments.length - 1 ? 600 : 400,
                }}
              >
                {seg.label}
              </button>
            </span>
          ))}
        </div>

        {/* Selected path bar */}
        <div style={{ padding: '8px 20px', background: 'rgba(124,58,237,0.08)', borderBottom: `1px solid ${C.border}`, display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: T.xs, color: C.text3 }}>Sélectionné :</span>
          <span style={{ fontSize: T.xs, color: C.text2, fontFamily: 'monospace', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {selected ?? '—'}
          </span>
        </div>

        {/* File list */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '8px 0' }}>
          {loading && (
            <div style={{ padding: '32px 20px', textAlign: 'center', color: C.text3, fontSize: T.sm }}>Chargement…</div>
          )}
          {error && (
            <div style={{ padding: '12px 20px', color: '#ef4444', fontSize: T.sm }}>{error}</div>
          )}

          {/* Drive list (root level) */}
          {!loading && !currentPath && drives.map((d) => (
            <FolderRow
              key={d.path}
              entry={d}
              isSelected={selected === d.path}
              onClick={() => setSelected(d.path)}
              onDoubleClick={() => navigate(d.path)}
            />
          ))}

          {/* Back row */}
          {!loading && currentPath && (
            <div
              onClick={goUp}
              style={{
                display: 'flex', alignItems: 'center', gap: 10, padding: '8px 20px',
                cursor: 'pointer', borderRadius: R.md, margin: '0 8px',
                color: C.text2, fontSize: T.sm,
              }}
              onMouseEnter={(e) => (e.currentTarget.style.background = 'rgba(255,255,255,0.05)')}
              onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
            >
              <span style={{ fontSize: '1rem', minWidth: 20 }}>⬆</span>
              <span style={{ fontFamily: 'monospace' }}>..</span>
            </div>
          )}

          {/* Directory entries */}
          {!loading && currentPath && entries.length === 0 && (
            <div style={{ padding: '32px 20px', textAlign: 'center', color: C.text3, fontSize: T.sm }}>Dossier vide</div>
          )}
          {!loading && currentPath && entries.map((e) => (
            <FolderRow
              key={e.path}
              entry={e}
              isSelected={selected === e.path}
              onClick={() => setSelected(e.path)}
              onDoubleClick={() => navigate(e.path)}
            />
          ))}
        </div>

        {/* Footer */}
        <div style={{ padding: '14px 20px', borderTop: `1px solid ${C.border}`, display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
          <button
            onClick={onClose}
            style={{ padding: '8px 18px', background: 'rgba(255,255,255,0.06)', border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text2, fontSize: T.sm, cursor: 'pointer' }}
          >
            Annuler
          </button>
          <button
            disabled={!selected}
            onClick={() => selected && onSelect(selected)}
            style={{
              padding: '8px 20px', background: selected ? '#7c3aed' : C.surf3,
              border: 'none', borderRadius: R.md, color: selected ? '#fff' : C.text3,
              fontSize: T.sm, fontWeight: 600, cursor: selected ? 'pointer' : 'not-allowed',
            }}
          >
            Sélectionner
          </button>
        </div>
      </div>
    </div>
  )
}

function FolderRow({ entry, isSelected, onClick, onDoubleClick }: {
  entry: FsEntry
  isSelected: boolean
  onClick: () => void
  onDoubleClick: () => void
}) {
  return (
    <div
      onClick={onClick}
      onDoubleClick={onDoubleClick}
      style={{
        display: 'flex', alignItems: 'center', gap: 10,
        padding: '7px 20px', cursor: 'pointer', margin: '0 8px', borderRadius: R.md,
        background: isSelected ? 'rgba(124,58,237,0.18)' : 'transparent',
        border: isSelected ? `1px solid rgba(124,58,237,0.35)` : '1px solid transparent',
      }}
      onMouseEnter={(e) => { if (!isSelected) e.currentTarget.style.background = 'rgba(255,255,255,0.05)' }}
      onMouseLeave={(e) => { if (!isSelected) e.currentTarget.style.background = 'transparent' }}
    >
      <span style={{ fontSize: '0.95rem', minWidth: 20 }}>📁</span>
      <span style={{ fontSize: T.sm, color: isSelected ? '#c4b5fd' : C.text, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontFamily: 'monospace' }}>
        {entry.name}
      </span>
      <span
        style={{ fontSize: T.xs, color: C.text3, opacity: 0.6 }}
        title="Double-clic pour ouvrir"
      >
        →
      </span>
    </div>
  )
}
