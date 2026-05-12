import { useEffect, useState } from 'react'
import { apiClient } from '../../api/client.ts'
import { C, R, T } from '../../design.ts'
import type { Activity } from '../../api/client.ts'

export default function AdminActivity() {
  const [activities, setActivities] = useState<Activity[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState('')

  useEffect(() => {
    loadActivities()
  }, [])

  async function loadActivities() {
    setLoading(true)
    try {
      const data = await apiClient.listActivity()
      setActivities(data)
    } catch {
      setActivities([])
    } finally {
      setLoading(false)
    }
  }

  const filtered = activities.filter((a) =>
    a.action.toLowerCase().includes(filter.toLowerCase()) ||
    a.details.toLowerCase().includes(filter.toLowerCase()) ||
    a.userId.toLowerCase().includes(filter.toLowerCase())
  )

  if (loading) return <div style={{ color: C.text3 }}>Chargement…</div>

  return (
    <div>
      <h1 style={{ fontSize: T['2xl'], fontWeight: 700, marginBottom: 24 }}>Activité</h1>

      {/* Filter */}
      <div style={{ marginBottom: 20 }}>
        <input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Filtrer par action, détail ou utilisateur…"
          style={{
            width: '100%', maxWidth: 400,
            padding: '9px 14px', background: C.surf2,
            border: `1px solid ${C.border}`, borderRadius: R.md,
            color: C.text, fontSize: T.sm,
          }}
        />
      </div>

      {/* Stats */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: 12, marginBottom: 24 }}>
        <StatCard label="Total" value={activities.length} />
        <StatCard label="Aujourd'hui" value={activities.filter((a) => isToday(a.createdAt)).length} />
        <StatCard label="Cette semaine" value={activities.filter((a) => isThisWeek(a.createdAt)).length} />
      </div>

      {/* Activity table */}
      <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: R.lg, overflow: 'hidden' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: T.sm }}>
          <thead>
            <tr style={{ background: 'rgba(255,255,255,0.03)', borderBottom: `1px solid ${C.border}` }}>
              <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 600, color: C.text2 }}>Date</th>
              <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 600, color: C.text2 }}>Utilisateur</th>
              <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 600, color: C.text2 }}>Action</th>
              <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 600, color: C.text2 }}>Détails</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((a) => (
              <tr key={a.id} style={{ borderBottom: `1px solid ${C.border}` }}>
                <td style={{ padding: '12px 16px', color: C.text2, fontSize: T.xs, whiteSpace: 'nowrap' }}>
                  {new Date(a.createdAt).toLocaleString('fr-FR')}
                </td>
                <td style={{ padding: '12px 16px', fontFamily: 'monospace', fontSize: T.xs, color: C.text3 }}>
                  {a.userId.slice(0, 8)}
                </td>
                <td style={{ padding: '12px 16px' }}>
                  <span style={{
                    display: 'inline-block', padding: '3px 10px', borderRadius: R.sm,
                    fontSize: '0.7rem', fontWeight: 700,
                    background: actionColor(a.action).bg,
                    color: actionColor(a.action).fg,
                  }}>
                    {a.action}
                  </span>
                </td>
                <td style={{ padding: '12px 16px', color: C.text2, fontSize: T.xs }}>
                  {a.details}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {filtered.length === 0 && (
          <div style={{ padding: 40, textAlign: 'center', color: C.text3 }}>Aucune activité</div>
        )}
      </div>
    </div>
  )
}

function StatCard({ label, value }: { label: string; value: number }) {
  return (
    <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: R.lg, padding: '16px 20px' }}>
      <div style={{ fontSize: T.xs, color: C.text2, marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: T.xl, fontWeight: 700 }}>{value}</div>
    </div>
  )
}

function isToday(iso: string): boolean {
  const d = new Date(iso)
  const now = new Date()
  return d.toDateString() === now.toDateString()
}

function isThisWeek(iso: string): boolean {
  const d = new Date(iso)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  return diff >= 0 && diff < 7 * 24 * 60 * 60 * 1000
}

function actionColor(action: string): { bg: string; fg: string } {
  switch (action) {
    case 'login': return { bg: 'rgba(34,197,94,0.12)', fg: '#22c55e' }
    case 'logout': return { bg: 'rgba(255,255,255,0.06)', fg: C.text2 }
    case 'play': return { bg: 'rgba(124,58,237,0.12)', fg: '#a78bfa' }
    case 'create': return { bg: 'rgba(59,130,246,0.12)', fg: '#60a5fa' }
    case 'delete': return { bg: 'rgba(239,68,68,0.12)', fg: '#ef4444' }
    default: return { bg: 'rgba(255,255,255,0.06)', fg: C.text2 }
  }
}
