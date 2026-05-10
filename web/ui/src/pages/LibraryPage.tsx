import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiClient, type Library, type MediaItem, type Collection } from '../api/client'
import { FolderOpen, Play, Plus } from 'lucide-react'

export default function LibraryPage() {
  const [libs, setLibs] = useState<Library[]>([])
  const [items, setItems] = useState<MediaItem[]>([])
  const [collections, setCollections] = useState<Collection[]>([])
  const [selectedLib, setSelectedLib] = useState<string>('')
  const [error, setError] = useState('')
  const [newLib, setNewLib] = useState({ name: '', path: '', mediaType: 'movie' })
  const [showNewLib, setShowNewLib] = useState(false)

  useEffect(() => {
    apiClient.listLibraries().then(setLibs).catch((e) => setError(e.message))
    apiClient.listCollections().then(setCollections).catch(() => null)
  }, [])

  useEffect(() => {
    if (!selectedLib) return
    apiClient.listItems(selectedLib).then(setItems).catch((e) => setError(e.message))
  }, [selectedLib])

  const createLibrary = async () => {
    if (!newLib.name || !newLib.path) return
    try {
      const lib = await apiClient.createLibrary(newLib.name, newLib.path, newLib.mediaType)
      setLibs(prev => [...prev, lib])
      setShowNewLib(false)
      setNewLib({ name: '', path: '', mediaType: 'movie' })
    } catch (e: any) {
      setError(e.message)
    }
  }

  const scanLibrary = async (id: string) => {
    try {
      await apiClient.scanLibrary(id)
      setError('Scan started')
    } catch (e: any) {
      setError(e.message)
    }
  }

  return (
    <div>
      <h1 style={{ color: '#66fcf1' }}>Libraries</h1>
      {error && <div className="error">{error}</div>}

      <div style={{ display: 'flex', gap: 12, marginBottom: 16 }}>
        <button onClick={() => setShowNewLib(!showNewLib)}><Plus size={14} /> New Library</button>
      </div>

      {showNewLib && (
        <div className="card" style={{ maxWidth: 480 }}>
          <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
            <input placeholder="Name" value={newLib.name} onChange={e => setNewLib({ ...newLib, name: e.target.value })} style={{ flex: 1 }} />
            <input placeholder="Path" value={newLib.path} onChange={e => setNewLib({ ...newLib, path: e.target.value })} style={{ flex: 2 }} />
            <select value={newLib.mediaType} onChange={e => setNewLib({ ...newLib, mediaType: e.target.value })}>
              <option value="movie">Movie</option>
              <option value="tv">TV</option>
              <option value="music">Music</option>
            </select>
          </div>
          <button onClick={createLibrary}>Create</button>
        </div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(220px,1fr))', gap: 12 }}>
        {libs.map(lib => (
          <div key={lib.id} className="card" style={{ cursor: 'pointer', border: selectedLib === lib.id ? '1px solid #66fcf1' : '1px solid transparent' }}
               onClick={() => setSelectedLib(lib.id)}>
            <FolderOpen size={20} style={{ color: '#66fcf1' }} />
            <div style={{ fontWeight: 600, marginTop: 8 }}>{lib.name}</div>
            <div style={{ fontSize: 12, color: '#888' }}>{lib.mediaType}</div>
            <div style={{ marginTop: 8, display: 'flex', gap: 8 }}>
              <button onClick={e => { e.stopPropagation(); scanLibrary(lib.id) }} style={{ fontSize: 11, padding: '4px 8px' }}>Scan</button>
            </div>
          </div>
        ))}
      </div>

      {selectedLib && (
        <>
          <h2 style={{ marginTop: 24 }}>Items</h2>
          {items.length === 0 && <div style={{ color: '#888' }}>No items in this library.</div>}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(260px,1fr))', gap: 12 }}>
            {items.map(item => (
              <div key={item.id} className="card">
                <div style={{ fontWeight: 600 }}>{item.name}</div>
                <div style={{ fontSize: 12, color: '#888' }}>{item.container} • {Math.round(item.sizeBytes / 1024 / 1024)} MB • {Math.round(item.durationSeconds / 60)} min</div>
                <div style={{ marginTop: 8, display: 'flex', gap: 8 }}>
                  <Link to={`/player/${item.id}`}><button style={{ fontSize: 11, padding: '4px 8px' }}><Play size={12} /> Play</button></Link>
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      <h2 style={{ marginTop: 24 }}>Collections</h2>
      {collections.length === 0 && <div style={{ color: '#888' }}>No collections yet.</div>}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(220px,1fr))', gap: 12 }}>
        {collections.map(col => (
          <div key={col.id} className="card">
            <div style={{ fontWeight: 600 }}>{col.name}</div>
            <div style={{ fontSize: 12, color: '#888' }}>{col.type}</div>
          </div>
        ))}
      </div>
    </div>
  )
}
