import { useEffect, useState } from 'react'
import { apiClient } from '../../api/client.ts'
import { C, R, T } from '../../design.ts'
import type { Library, User, Activity } from '../../api/client.ts'

export default function AdminDashboard() {
  const [stats, setStats] = useState({ users: 0, libraries: 0, items: 0, activities: 0 })
  const [recentActivity, setRecentActivity] = useState<Activity[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadStats()
  }, [])

  async function loadStats() {
    try {
      const [users, libraries, activities] = await Promise.all([
        apiClient.listUsers().catch(() => [] as User[]),
        apiClient.listLibraries().catch(() => [] as Library[]),
        apiClient.listActivity().catch(() => [] as Activity[]),
      ])
      setStats({
        users: users.length,
        libraries: libraries.length,
        items: libraries.reduce((sum, l) => sum + (l as any).item_count || 0, 0),
        activities: activities.length,
      })
      setRecentActivity(activities.slice(0, 5))
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }

  if (loading) return <div style={{ color: C.text3 }}>Chargement…</div>

  return (
    <div>
      <h1 style={{ fontSize: T['2xl'], fontWeight: 700, marginBottom: 24 }}>Vue d'ensemble</h1>

      {/* Stats cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 16, marginBottom: 32 }}>
        <StatCard label="Utilisateurs" value={stats.users} icon="◉" color="#a78bfa" />
        <StatCard label="Bibliothèques" value={stats.libraries} icon="▣" color="#60a5fa" />
        <StatCard label="Éléments" value={stats.items} icon="◈" color="#22c55e" />
        <StatCard label="Activités" value={stats.activities} icon="◆" color="#f59e0b" />
      </div>

      {/* Recent activity */}
      <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: R.lg, padding: 20 }}>
        <h2 style={{ fontSize: T.base, fontWeight: 600, marginBottom: 16 }}>Activité récente</h2>
        {recentActivity.length === 0 ? (
          <div style={{ color: C.text3, fontSize: T.sm, padding: '20px 0' }}>Aucune activité récente</div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {recentActivity.map((a) => (
              <div key={a.id} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 0', borderBottom: `1px solid ${C.border}` }}>
                <div style={{
                  width: 8, height: 8, borderRadius: '50%',
                  background: actionColor(a.action),
                  flexShrink: 0,
                }} />
                <div style={{ flex: 1 }}>
                  <span style={{ fontWeight: 500, fontSize: T.sm }}>{a.action}</span>
                  <span style={{ color: C.text2, fontSize: T.xs, marginLeft: 8 }}>{a.details}</span>
                </div>
                <div style={{ color: C.text3, fontSize: T.xs, whiteSpace: 'nowrap' }}>
                  {new Date(a.createdAt).toLocaleString('fr-FR')}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function StatCard({ label, value, icon, color }: { label: string; value: number; icon: string; color: string }) {
  return (
    <div style={{
      background: C.surface, border: `1px solid ${C.border}`, borderRadius: R.lg,
      padding: '20px 24px', display: 'flex', alignItems: 'center', gap: 14,
    }}>
      <div style={{
        width: 44, height: 44, borderRadius: R.md,
        background: `${color}15`, color: color,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        fontSize: '1.2rem',
      }}>
        {icon}
      </div>
      <div>
        <div style={{ fontSize: T.xs, color: C.text2, marginBottom: 2 }}>{label}</div>
        <div style={{ fontSize: T.xl, fontWeight: 700 }}>{value}</div>
      </div>
    </div>
  )
}

function actionColor(action: string): string {
  switch (action) {
    case 'login': return '#22c55e'
    case 'logout': return '#6b7280'
    case 'play': return '#a78bfa'
    case 'create': return '#60a5fa'
    case 'delete': return '#ef4444'
    default: return '#6b7280'
  }
}
