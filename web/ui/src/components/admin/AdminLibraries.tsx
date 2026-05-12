import { useEffect, useState } from 'react'
import { apiClient } from '../../api/client.ts'
import { C, R, T } from '../../design.ts'
import type { Library } from '../../api/client.ts'

export default function AdminLibraries() {
  const [libraries, setLibraries] = useState<Library[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [scanning, setScanning] = useState<Set<string>>(new Set())
  const [newName, setNewName] = useState('')
  const [newPath, setNewPath] = useState('')
  const [newType, setNewType] = useState('movie')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  useEffect(() => {
    loadLibraries()
  }, [])

  async function loadLibraries() {
    setLoading(true)
    try {
      const data = await apiClient.listLibraries()
      setLibraries(data)
    } catch {
      setError('Erreur lors du chargement des bibliothèques')
    } finally {
      setLoading(false)
    }
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setError(''); setSuccess('')
    if (!newName.trim() || !newPath.trim()) {
      setError('Nom et chemin requis')
      return
    }
    setCreating(true)
    try {
      await apiClient.createLibrary(newName.trim(), newPath.trim(), newType)
      setSuccess('Bibliothèque créée')
      setNewName(''); setNewPath(''); setNewType('movie')
      await loadLibraries()
    } catch (err: any) {
      setError(err.message || 'Erreur lors de la création')
    } finally {
      setCreating(false)
    }
  }

  async function handleScan(id: string) {
    setScanning((s) => new Set(s).add(id))
    try {
      await apiClient.scanLibrary(id)
      setSuccess('Scan démarré')
    } catch (err: any) {
      setError(err.message || 'Erreur lors du scan')
    }
    setTimeout(() => {
      setScanning((s) => { const n = new Set(s); n.delete(id); return n })
    }, 3000)
  }

  if (loading) return <div style={{ color: C.text3 }}>Chargement…</div>

  return (
    <div>
      <h1 style={{ fontSize: T['2xl'], fontWeight: 700, marginBottom: 24 }}>Bibliothèques</h1>

      {error && <Alert type="error">{error}</Alert>}
      {success && <Alert type="success">{success}</Alert>}

      {/* Create form */}
      <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: R.lg, padding: 20, marginBottom: 24 }}>
        <h2 style={{ fontSize: T.base, fontWeight: 600, marginBottom: 16 }}>Nouvelle bibliothèque</h2>
        <form onSubmit={handleCreate} style={{ display: 'flex', gap: 10, alignItems: 'flex-end', flexWrap: 'wrap' }}>
          <div style={{ flex: 1, minWidth: 160 }}>
            <label style={{ display: 'block', fontSize: T.xs, color: C.text2, marginBottom: 4 }}>Nom</label>
            <input
              value={newName} onChange={(e) => setNewName(e.target.value)}
              placeholder="ex: Films"
              style={{ width: '100%', padding: '8px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm }}
            />
          </div>
          <div style={{ flex: 2, minWidth: 200 }}>
            <label style={{ display: 'block', fontSize: T.xs, color: C.text2, marginBottom: 4 }}>Chemin</label>
            <input
              value={newPath} onChange={(e) => setNewPath(e.target.value)}
              placeholder="ex: /media/films"
              style={{ width: '100%', padding: '8px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm }}
            />
          </div>
          <div style={{ width: 140 }}>
            <label style={{ display: 'block', fontSize: T.xs, color: C.text2, marginBottom: 4 }}>Type</label>
            <select
              value={newType} onChange={(e) => setNewType(e.target.value)}
              style={{ width: '100%', padding: '8px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm }}
            >
              <option value="movie">Film</option>
              <option value="show">Série</option>
              <option value="music">Musique</option>
            </select>
          </div>
          <button
            type="submit" disabled={creating}
            style={{ padding: '9px 20px', background: creating ? C.surf3 : '#7c3aed', color: '#fff', border: 'none', borderRadius: R.md, fontWeight: 600, fontSize: T.sm, cursor: creating ? 'not-allowed' : 'pointer' }}
          >
            {creating ? 'Création…' : 'Créer'}
          </button>
        </form>
      </div>

      {/* Libraries table */}
      <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: R.lg, overflow: 'hidden' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: T.sm }}>
          <thead>
            <tr style={{ background: 'rgba(255,255,255,0.03)', borderBottom: `1px solid ${C.border}` }}>
              <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 600, color: C.text2 }}>Nom</th>
              <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 600, color: C.text2 }}>Chemin</th>
              <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 600, color: C.text2 }}>Type</th>
              <th style={{ padding: '12px 16px', textAlign: 'right', fontWeight: 600, color: C.text2 }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {libraries.map((lib) => (
              <tr key={lib.id} style={{ borderBottom: `1px solid ${C.border}` }}>
                <td style={{ padding: '12px 16px' }}>
                  <div style={{ fontWeight: 500 }}>{lib.name}</div>
                  <div style={{ fontSize: T.xs, color: C.text3, fontFamily: 'monospace' }}>{lib.id.slice(0, 8)}</div>
                </td>
                <td style={{ padding: '12px 16px', color: C.text2, fontFamily: 'monospace', fontSize: T.xs }}>{lib.path}</td>
                <td style={{ padding: '12px 16px' }}>
                  <span style={{
                    display: 'inline-block', padding: '3px 10px', borderRadius: R.sm,
                    fontSize: '0.7rem', fontWeight: 700,
                    background: 'rgba(255,255,255,0.06)', color: C.text2,
                  }}>
                    {lib.mediaType}
                  </span>
                </td>
                <td style={{ padding: '12px 16px', textAlign: 'right' }}>
                  <button
                    onClick={() => handleScan(lib.id)}
                    disabled={scanning.has(lib.id)}
                    style={{ padding: '5px 12px', background: scanning.has(lib.id) ? C.surf3 : 'rgba(124,58,237,0.15)', color: scanning.has(lib.id) ? C.text3 : '#a78bfa', border: 'none', borderRadius: R.sm, fontSize: T.xs, fontWeight: 600, cursor: scanning.has(lib.id) ? 'not-allowed' : 'pointer' }}
                  >
                    {scanning.has(lib.id) ? 'Scan…' : 'Scanner'}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {libraries.length === 0 && (
          <div style={{ padding: 40, textAlign: 'center', color: C.text3 }}>Aucune bibliothèque</div>
        )}
      </div>
    </div>
  )
}

function Alert({ type, children }: { type: 'error' | 'success'; children: React.ReactNode }) {
  const isErr = type === 'error'
  return (
    <div style={{
      background: isErr ? 'rgba(239,68,68,0.1)' : 'rgba(34,197,94,0.1)',
      border: `1px solid ${isErr ? 'rgba(239,68,68,0.3)' : 'rgba(34,197,94,0.3)'}`,
      borderRadius: R.sm, padding: '10px 14px',
      color: isErr ? '#ef4444' : '#22c55e',
      fontSize: T.sm, marginBottom: 16,
    }}>
      {children}
    </div>
  )
}
