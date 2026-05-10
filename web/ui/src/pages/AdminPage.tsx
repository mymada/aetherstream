import { useEffect, useState } from 'react'
import { apiClient, type User } from '../api/client'
import { Users, Trash, Shield } from 'lucide-react'

export default function AdminPage() {
  const [users, setUsers] = useState<User[]>([])
  const [error, setError] = useState('')
  const [newUser, setNewUser] = useState({ username: '', password: '', role: 'user' })
  const [showForm, setShowForm] = useState(false)

  useEffect(() => {
    apiClient.listUsers().then(setUsers).catch((e) => setError(e.message))
  }, [])

  const createUser = async () => {
    if (!newUser.username || !newUser.password) return
    try {
      const u = await apiClient.createUser(newUser.username, newUser.password, newUser.role)
      setUsers(prev => [...prev, u])
      setShowForm(false)
      setNewUser({ username: '', password: '', role: 'user' })
    } catch (e: any) {
      setError(e.message)
    }
  }

  const deleteUser = async (id: string) => {
    if (!confirm('Delete this user?')) return
    try {
      await apiClient.deleteUser(id)
      setUsers(prev => prev.filter(u => u.id !== id))
    } catch (e: any) {
      setError(e.message)
    }
  }

  return (
    <div>
      <h1 style={{ color: '#66fcf1' }}>Admin</h1>
      {error && <div className="error">{error}</div>}

      <div style={{ display: 'flex', gap: 12, marginBottom: 16 }}>
        <button onClick={() => setShowForm(!showForm)}><Users size={14} /> New User</button>
      </div>

      {showForm && (
        <div className="card" style={{ maxWidth: 480 }}>
          <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
            <input placeholder="Username" value={newUser.username} onChange={e => setNewUser({ ...newUser, username: e.target.value })} style={{ flex: 1 }} />
            <input placeholder="Password" type="password" value={newUser.password} onChange={e => setNewUser({ ...newUser, password: e.target.value })} style={{ flex: 1 }} />
            <select value={newUser.role} onChange={e => setNewUser({ ...newUser, role: e.target.value })}>
              <option value="user">User</option>
              <option value="admin">Admin</option>
            </select>
          </div>
          <button onClick={createUser}>Create</button>
        </div>
      )}

      <h2>Users</h2>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(280px,1fr))', gap: 12 }}>
        {users.map(u => (
          <div key={u.id} className="card" style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <Shield size={18} style={{ color: u.role === 'admin' ? '#66fcf1' : '#888' }} />
            <div style={{ flex: 1 }}>
              <div style={{ fontWeight: 600 }}>{u.username}</div>
              <div style={{ fontSize: 12, color: '#888' }}>{u.role} • {new Date(u.createdAt).toLocaleDateString()}</div>
            </div>
            <button onClick={() => deleteUser(u.id)} style={{ padding: '4px 8px', fontSize: 11, background: '#ef4444' }}><Trash size={12} /></button>
          </div>
        ))}
      </div>
    </div>
  )
}
