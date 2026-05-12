import { useEffect, useState } from 'react'
import { apiClient } from '../../api/client.ts'
import { C, R, T } from '../../design.ts'
import type { User } from '../../api/client.ts'

export default function AdminUsers() {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [newUsername, setNewUsername] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [newRole, setNewRole] = useState('user')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  useEffect(() => {
    loadUsers()
  }, [])

  async function loadUsers() {
    setLoading(true)
    try {
      const data = await apiClient.listUsers()
      setUsers(data)
    } catch {
      setError('Erreur lors du chargement des utilisateurs')
    } finally {
      setLoading(false)
    }
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setError(''); setSuccess('')
    if (!newUsername.trim() || !newPassword.trim()) {
      setError('Nom d\'utilisateur et mot de passe requis')
      return
    }
    setCreating(true)
    try {
      await apiClient.createUser(newUsername.trim(), newPassword.trim(), newRole)
      setSuccess('Utilisateur créé')
      setNewUsername(''); setNewPassword(''); setNewRole('user')
      await loadUsers()
    } catch (err: any) {
      setError(err.message || 'Erreur lors de la création')
    } finally {
      setCreating(false)
    }
  }

  async function handleUpdateRole(id: string, role: string) {
    try {
      await apiClient.updateUser(id, role)
      setSuccess('Rôle mis à jour')
      setEditingId(null)
      await loadUsers()
    } catch (err: any) {
      setError(err.message || 'Erreur lors de la mise à jour')
    }
  }

  async function handleDelete(id: string) {
    if (!confirm('Supprimer cet utilisateur ?')) return
    try {
      await apiClient.deleteUser(id)
      setSuccess('Utilisateur supprimé')
      await loadUsers()
    } catch (err: any) {
      setError(err.message || 'Erreur lors de la suppression')
    }
  }

  if (loading) return <div style={{ color: C.text3 }}>Chargement…</div>

  return (
    <div>
      <h1 style={{ fontSize: T['2xl'], fontWeight: 700, marginBottom: 24 }}>Utilisateurs</h1>

      {error && <Alert type="error">{error}</Alert>}
      {success && <Alert type="success">{success}</Alert>}

      {/* Create form */}
      <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: R.lg, padding: 20, marginBottom: 24 }}>
        <h2 style={{ fontSize: T.base, fontWeight: 600, marginBottom: 16 }}>Nouvel utilisateur</h2>
        <form onSubmit={handleCreate} style={{ display: 'flex', gap: 10, alignItems: 'flex-end', flexWrap: 'wrap' }}>
          <div style={{ flex: 1, minWidth: 160 }}>
            <label style={{ display: 'block', fontSize: T.xs, color: C.text2, marginBottom: 4 }}>Nom d'utilisateur</label>
            <input
              value={newUsername} onChange={(e) => setNewUsername(e.target.value)}
              placeholder="ex: john"
              style={{ width: '100%', padding: '8px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm }}
            />
          </div>
          <div style={{ flex: 1, minWidth: 160 }}>
            <label style={{ display: 'block', fontSize: T.xs, color: C.text2, marginBottom: 4 }}>Mot de passe</label>
            <input
              type="password"
              value={newPassword} onChange={(e) => setNewPassword(e.target.value)}
              placeholder="••••••••"
              style={{ width: '100%', padding: '8px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm }}
            />
          </div>
          <div style={{ width: 140 }}>
            <label style={{ display: 'block', fontSize: T.xs, color: C.text2, marginBottom: 4 }}>Rôle</label>
            <select
              value={newRole} onChange={(e) => setNewRole(e.target.value)}
              style={{ width: '100%', padding: '8px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm }}
            >
              <option value="user">Utilisateur</option>
              <option value="admin">Administrateur</option>
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

      {/* Users table */}
      <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: R.lg, overflow: 'hidden' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: T.sm }}>
          <thead>
            <tr style={{ background: 'rgba(255,255,255,0.03)', borderBottom: `1px solid ${C.border}` }}>
              <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 600, color: C.text2 }}>Utilisateur</th>
              <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 600, color: C.text2 }}>Rôle</th>
              <th style={{ padding: '12px 16px', textAlign: 'left', fontWeight: 600, color: C.text2 }}>Créé le</th>
              <th style={{ padding: '12px 16px', textAlign: 'right', fontWeight: 600, color: C.text2 }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {users.map((user) => (
              <tr key={user.id} style={{ borderBottom: `1px solid ${C.border}` }}>
                <td style={{ padding: '12px 16px' }}>
                  <div style={{ fontWeight: 500 }}>{user.username}</div>
                  <div style={{ fontSize: T.xs, color: C.text3, fontFamily: 'monospace' }}>{user.id.slice(0, 8)}</div>
                </td>
                <td style={{ padding: '12px 16px' }}>
                  {editingId === user.id ? (
                    <select
                      value={user.role}
                      onChange={(e) => handleUpdateRole(user.id, e.target.value)}
                      style={{ padding: '4px 8px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.sm, color: C.text, fontSize: T.sm }}
                    >
                      <option value="user">Utilisateur</option>
                      <option value="admin">Administrateur</option>
                    </select>
                  ) : (
                    <span style={{
                      display: 'inline-block', padding: '3px 10px', borderRadius: R.sm,
                      fontSize: '0.7rem', fontWeight: 700,
                      background: user.role === 'admin' ? 'rgba(124,58,237,0.15)' : 'rgba(255,255,255,0.06)',
                      color: user.role === 'admin' ? '#a78bfa' : C.text2,
                    }}>
                      {user.role}
                    </span>
                  )}
                </td>
                <td style={{ padding: '12px 16px', color: C.text2, fontSize: T.xs }}>
                  {new Date(user.createdAt).toLocaleDateString('fr-FR')}
                </td>
                <td style={{ padding: '12px 16px', textAlign: 'right' }}>
                  <button
                    onClick={() => setEditingId(editingId === user.id ? null : user.id)}
                    style={{ padding: '4px 10px', background: 'transparent', color: C.text2, border: 'none', fontSize: T.xs, cursor: 'pointer', marginRight: 8 }}
                  >
                    {editingId === user.id ? 'OK' : 'Modifier'}
                  </button>
                  <button
                    onClick={() => handleDelete(user.id)}
                    style={{ padding: '4px 10px', background: 'transparent', color: '#ef4444', border: 'none', fontSize: T.xs, cursor: 'pointer' }}
                  >
                    Supprimer
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {users.length === 0 && (
          <div style={{ padding: 40, textAlign: 'center', color: C.text3 }}>Aucun utilisateur</div>
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
