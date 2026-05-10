import { useEffect, useState } from 'react'
import { apiClient, type Library, type Activity } from '../api/client'
import { Film, Clock, HardDrive } from 'lucide-react'

export default function DashboardPage() {
  const [libs, setLibs] = useState<Library[]>([])
  const [activity, setActivity] = useState<Activity[]>([])
  const [info, setInfo] = useState<{ name: string; version: string; status: string } | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    apiClient.systemInfo().then(setInfo).catch(() => null)
    apiClient.listLibraries().then(setLibs).catch((e) => setError(e.message))
    apiClient.listActivity().then(setActivity).catch(() => null)
  }, [])

  return (
    <div>
      <h1 style={{ color: '#66fcf1' }}>Dashboard</h1>
      {info && (
        <div className="card" style={{ display: 'flex', gap: 16, alignItems: 'center' }}>
          <HardDrive size={28} />
          <div>
            <div style={{ fontSize: 18, fontWeight: 600 }}>{info.name} {info.version}</div>
            <div style={{ fontSize: 12, color: '#888' }}>Status: {info.status}</div>
          </div>
        </div>
      )}

      <h2>Libraries</h2>
      {error && <div className="error">{error}</div>}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(240px,1fr))', gap: 12 }}>
        {libs.map(lib => (
          <div key={lib.id} className="card">
            <Film size={20} style={{ color: '#66fcf1' }} />
            <div style={{ fontWeight: 600, marginTop: 8 }}>{lib.name}</div>
            <div style={{ fontSize: 12, color: '#888' }}>{lib.mediaType} • {lib.path}</div>
          </div>
        ))}
      </div>

      <h2>Recent Activity</h2>
      <div>
        {activity.length === 0 && <div style={{ color: '#888' }}>No activity yet.</div>}
        {activity.map(a => (
          <div key={a.id} className="card" style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
            <Clock size={16} />
            <div>
              <span style={{ fontWeight: 600 }}>{a.action}</span>
              <span style={{ color: '#888', marginLeft: 8 }}>{a.details}</span>
              <div style={{ fontSize: 11, color: '#666' }}>{new Date(a.createdAt).toLocaleString()}</div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
