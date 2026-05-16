import React, { useEffect, useRef, useState } from 'react'
import { useAuth } from '../hooks/useAuth.ts'
import {
  getLibraries, getJobs, cancelJob, getTranscodeDirs, deleteTranscodeDir,
  type Job, type TranscodeDir,
} from '../api.ts'
import { apiClient } from '../api/client.ts'
import { C, R, T } from '../design.ts'
import type { Account, Profile } from '../api/client.ts'

// ── Helpers ───────────────────────────────────────────────────────────────────

function fmtBytes(b: number): string {
  if (b >= 1e9) return (b / 1e9).toFixed(1) + ' Go'
  if (b >= 1e6) return (b / 1e6).toFixed(0) + ' Mo'
  if (b >= 1e3) return (b / 1e3).toFixed(0) + ' Ko'
  return b + ' o'
}

function fmtAge(iso: string): string {
  const s = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (s < 60) return `${s}s`
  if (s < 3600) return `${Math.floor(s / 60)}min`
  if (s < 86400) return `${Math.floor(s / 3600)}h`
  return `${Math.floor(s / 86400)}j`
}

const STATUS_LABEL: Record<Job['status'], string> = {
  queued: 'En attente',
  running: 'En cours',
  completed: 'Terminé',
  failed: 'Erreur',
  cancelled: 'Annulé',
}
const STATUS_COLOR: Record<Job['status'], string> = {
  queued: 'rgba(255,255,255,0.25)',
  running: '#7c3aed',
  completed: '#22c55e',
  failed: '#ef4444',
  cancelled: 'rgba(255,255,255,0.2)',
}

// ── Jobs section ──────────────────────────────────────────────────────────────

function JobsSection() {
  const [jobs, setJobs] = useState<Job[]>([])
  const [cancelling, setCancelling] = useState<Set<string>>(new Set())
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const load = () => getJobs().then(setJobs).catch(() => {})

  useEffect(() => {
    load()
  }, [])

  // Auto-refresh while there are active jobs
  useEffect(() => {
    const hasActive = jobs.some((j) => j.status === 'queued' || j.status === 'running')
    if (timerRef.current) clearTimeout(timerRef.current)
    if (hasActive) {
      timerRef.current = setTimeout(() => load(), 2000)
    }
    return () => { if (timerRef.current) clearTimeout(timerRef.current) }
  }, [jobs])

  async function handleCancel(id: string) {
    setCancelling((s) => new Set(s).add(id))
    try { await cancelJob(id) } catch {}
    await load()
    setCancelling((s) => { const n = new Set(s); n.delete(id); return n })
  }

  if (jobs.length === 0) {
    return <div style={{ color: C.text2, fontSize: T.sm, padding: '4px 0' }}>Aucune tâche.</div>
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      {jobs.map((job) => {
        const active = job.status === 'queued' || job.status === 'running'
        return (
          <div key={job.id} style={{
            display: 'flex', alignItems: 'center', gap: 12,
            padding: '11px 14px', background: C.surf2,
            borderRadius: R.md, border: `1px solid ${active ? 'rgba(124,58,237,0.3)' : C.border}`,
            transition: 'border-color 0.2s',
          }}>
            {/* Status dot / spinner */}
            <div style={{ flexShrink: 0, width: 10, height: 10 }}>
              {job.status === 'running' ? (
                <div style={{
                  width: 10, height: 10, borderRadius: R.full,
                  border: '2px solid rgba(124,58,237,0.3)',
                  borderTopColor: '#7c3aed',
                  animation: 'spin 0.8s linear infinite',
                }} />
              ) : (
                <div style={{
                  width: 10, height: 10, borderRadius: R.full,
                  background: STATUS_COLOR[job.status],
                }} />
              )}
            </div>

            {/* Info */}
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: T.sm, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {job.item_title || job.item_id}
                {job.audio_index > 0 && (
                  <span style={{ color: C.text3, fontWeight: 400, marginLeft: 6 }}>piste audio {job.audio_index}</span>
                )}
              </div>
              <div style={{ display: 'flex', gap: 8, marginTop: 2, alignItems: 'center' }}>
                <span style={{ fontSize: '0.68rem', color: STATUS_COLOR[job.status], fontWeight: 700 }}>
                  {STATUS_LABEL[job.status]}
                </span>
                <span style={{ color: C.text3, fontSize: '0.68rem' }}>
                  {job.profiles?.join(', ')}
                </span>
                {job.disk_bytes ? (
                  <span style={{ color: C.text3, fontSize: '0.68rem' }}>{fmtBytes(job.disk_bytes)}</span>
                ) : null}
                {job.error && (
                  <span style={{ color: '#ef4444', fontSize: '0.68rem' }} title={job.error}>⚠ {job.error.slice(0, 60)}</span>
                )}
                <span style={{ color: C.text3, fontSize: '0.68rem', marginLeft: 'auto' }}>
                  {fmtAge(job.created_at)}
                </span>
              </div>
            </div>

            {/* Cancel button */}
            {active && (
              <button
                onClick={() => handleCancel(job.id)}
                disabled={cancelling.has(job.id)}
                style={{
                  padding: '4px 10px', borderRadius: R.sm, fontSize: '0.7rem', fontWeight: 700,
                  background: 'rgba(239,68,68,0.1)', color: '#ef4444',
                  border: '1px solid rgba(239,68,68,0.25)', cursor: 'pointer',
                  opacity: cancelling.has(job.id) ? 0.5 : 1, flexShrink: 0,
                }}
              >
                Annuler
              </button>
            )}
          </div>
        )
      })}
    </div>
  )
}

// ── Storage section ───────────────────────────────────────────────────────────

function StorageSection() {
  const [dirs, setDirs] = useState<TranscodeDir[]>([])
  const [deleting, setDeleting] = useState<Set<string>>(new Set())
  const [confirmKey, setConfirmKey] = useState<string | null>(null)

  const load = () => getTranscodeDirs().then(setDirs).catch(() => {})
  useEffect(() => { load() }, [])

  const total = dirs.reduce((s, d) => s + d.disk_bytes, 0)

  async function handleDelete(key: string) {
    if (confirmKey !== key) { setConfirmKey(key); return }
    setConfirmKey(null)
    setDeleting((s) => new Set(s).add(key))
    try { await deleteTranscodeDir(key) } catch {}
    await load()
    setDeleting((s) => { const n = new Set(s); n.delete(key); return n })
  }

  async function handleDeleteAll() {
    const inactive = dirs.filter((d) => !d.active)
    for (const d of inactive) {
      setDeleting((s) => new Set(s).add(d.key))
      try { await deleteTranscodeDir(d.key) } catch {}
    }
    await load()
    setDeleting(new Set())
  }

  if (dirs.length === 0) {
    return <div style={{ color: C.text2, fontSize: T.sm, padding: '4px 0' }}>Aucun transcodage sur le disque.</div>
  }

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
        <span style={{ color: C.text2, fontSize: T.xs }}>
          {dirs.length} dossier{dirs.length > 1 ? 's' : ''} · {fmtBytes(total)} total
        </span>
        <button
          onClick={handleDeleteAll}
          style={{ padding: '5px 12px', borderRadius: R.sm, fontSize: '0.7rem', fontWeight: 700, background: 'rgba(239,68,68,0.1)', color: '#ef4444', border: '1px solid rgba(239,68,68,0.25)', cursor: 'pointer' }}
        >
          Tout nettoyer
        </button>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        {dirs.map((d) => (
          <div key={d.key} style={{
            display: 'flex', alignItems: 'center', gap: 10,
            padding: '9px 12px', background: C.surf2,
            borderRadius: R.md, border: `1px solid ${d.active ? 'rgba(124,58,237,0.3)' : C.border}`,
          }}>
            {d.active && (
              <div style={{ width: 8, height: 8, borderRadius: R.full, background: '#7c3aed', flexShrink: 0, animation: 'pulse 1.5s ease-in-out infinite' }} />
            )}
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: T.xs, fontFamily: 'monospace', color: C.text2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {d.key}
              </div>
              <div style={{ fontSize: '0.68rem', color: C.text3, marginTop: 1 }}>
                {fmtBytes(d.disk_bytes)}
                {d.audio_index > 0 && ` · piste audio ${d.audio_index}`}
                {d.active && <span style={{ color: '#7c3aed', marginLeft: 6 }}>en cours</span>}
              </div>
            </div>
            <button
              onClick={() => handleDelete(d.key)}
              disabled={d.active || deleting.has(d.key)}
              style={{
                padding: '4px 10px', borderRadius: R.sm, fontSize: '0.7rem', fontWeight: 700,
                background: confirmKey === d.key ? 'rgba(239,68,68,0.25)' : 'rgba(255,255,255,0.05)',
                color: confirmKey === d.key ? '#ef4444' : C.text2,
                border: `1px solid ${confirmKey === d.key ? 'rgba(239,68,68,0.4)' : C.border}`,
                cursor: d.active ? 'not-allowed' : 'pointer',
                opacity: d.active || deleting.has(d.key) ? 0.4 : 1,
                flexShrink: 0, transition: 'all 0.15s',
              }}
            >
              {deleting.has(d.key) ? '…' : confirmKey === d.key ? 'Confirmer' : 'Supprimer'}
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}

// ── Account section ───────────────────────────────────────────────────────────

function AccountSection() {
  const [account, setAccount] = useState<Account | null>(null)
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  // Edit states
  const [editingEmail, setEditingEmail] = useState(false)
  const [newEmail, setNewEmail] = useState('')
  const [changingPassword, setChangingPassword] = useState(false)
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')

  // Profile creation
  const [creatingProfile, setCreatingProfile] = useState(false)
  const [newProfileName, setNewProfileName] = useState('')

  useEffect(() => {
    loadAccount()
  }, [])

  async function loadAccount() {
    setLoading(true)
    try {
      const acc = await apiClient.get<Account>('/api/account/me')
      setAccount(acc)
      setNewEmail(acc.email)

      // Load profiles
      const profs = await apiClient.get<Profile[]>('/api/profiles')
      setProfiles(profs)
    } catch (err: any) {
      setError(err.message || 'Erreur de chargement')
    } finally {
      setLoading(false)
    }
  }

  async function handleUpdateEmail() {
    if (!newEmail.trim()) return
    try {
      await apiClient.put('/api/account/me', { email: newEmail.trim() })
      setSuccess('Email mis à jour')
      setEditingEmail(false)
      await loadAccount()
    } catch (err: any) {
      setError(err.message || 'Erreur')
    }
  }

  async function handleChangePassword() {
    if (!currentPassword || !newPassword) {
      setError('Tous les champs sont requis')
      return
    }
    try {
      await apiClient.post('/api/account/me/password', {
        currentPassword,
        newPassword,
      })
      setSuccess('Mot de passe changé')
      setChangingPassword(false)
      setCurrentPassword('')
      setNewPassword('')
    } catch (err: any) {
      setError(err.message || 'Erreur')
    }
  }

  async function handleCreateProfile() {
    if (!newProfileName.trim()) {
      setError('Nom du profil requis')
      return
    }
    try {
      await apiClient.post('/api/profiles', { name: newProfileName.trim() })
      setSuccess('Profil créé')
      setCreatingProfile(false)
      setNewProfileName('')
      await loadAccount()
    } catch (err: any) {
      setError(err.message || 'Erreur')
    }
  }

  async function handleDeleteProfile(profileId: string) {
    if (!confirm('Supprimer ce profil ?')) return
    try {
      await apiClient.delete(`/api/profiles/${profileId}`)
      setSuccess('Profil supprimé')
      await loadAccount()
    } catch (err: any) {
      setError(err.message || 'Erreur')
    }
  }

  if (loading) return <div style={{ color: C.text3 }}>Chargement…</div>
  if (!account) return <div style={{ color: C.text3 }}>Non connecté</div>

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      {error && <Alert type="error">{error}</Alert>}
      {success && <Alert type="success">{success}</Alert>}

      {/* Email */}
      <div>
        <div style={{ fontSize: T.xs, color: C.text2, marginBottom: 4 }}>Email</div>
        {editingEmail ? (
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              value={newEmail}
              onChange={(e) => setNewEmail(e.target.value)}
              style={{ flex: 1, padding: '8px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm }}
            />
            <button onClick={handleUpdateEmail} style={{ padding: '8px 16px', background: '#7c3aed', color: '#fff', border: 'none', borderRadius: R.md, fontSize: T.sm, cursor: 'pointer' }}>OK</button>
            <button onClick={() => { setEditingEmail(false); setNewEmail(account.email) }} style={{ padding: '8px 16px', background: 'transparent', color: C.text2, border: `1px solid ${C.border}`, borderRadius: R.md, fontSize: T.sm, cursor: 'pointer' }}>Annuler</button>
          </div>
        ) : (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span style={{ fontSize: T.sm }}>{account.email}</span>
            <button onClick={() => setEditingEmail(true)} style={{ padding: '4px 12px', background: 'transparent', color: C.text2, border: `1px solid ${C.border}`, borderRadius: R.sm, fontSize: T.xs, cursor: 'pointer' }}>Modifier</button>
          </div>
        )}
      </div>

      {/* Password */}
      <div>
        <div style={{ fontSize: T.xs, color: C.text2, marginBottom: 4 }}>Mot de passe</div>
        {changingPassword ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <input
              type="password"
              placeholder="Mot de passe actuel"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              style={{ padding: '8px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm }}
            />
            <input
              type="password"
              placeholder="Nouveau mot de passe"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              style={{ padding: '8px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm }}
            />
            <div style={{ display: 'flex', gap: 8 }}>
              <button onClick={handleChangePassword} style={{ padding: '8px 16px', background: '#7c3aed', color: '#fff', border: 'none', borderRadius: R.md, fontSize: T.sm, cursor: 'pointer' }}>Changer</button>
              <button onClick={() => { setChangingPassword(false); setCurrentPassword(''); setNewPassword('') }} style={{ padding: '8px 16px', background: 'transparent', color: C.text2, border: `1px solid ${C.border}`, borderRadius: R.md, fontSize: T.sm, cursor: 'pointer' }}>Annuler</button>
            </div>
          </div>
        ) : (
          <button onClick={() => setChangingPassword(true)} style={{ padding: '4px 12px', background: 'transparent', color: C.text2, border: `1px solid ${C.border}`, borderRadius: R.sm, fontSize: T.xs, cursor: 'pointer' }}>Changer le mot de passe</button>
        )}
      </div>

      {/* Role */}
      <div>
        <div style={{ fontSize: T.xs, color: C.text2, marginBottom: 4 }}>Rôle</div>
        <span style={{
          display: 'inline-block', padding: '3px 10px', borderRadius: R.sm,
          fontSize: '0.7rem', fontWeight: 700,
          background: account.role === 'admin' ? 'rgba(124,58,237,0.15)' : 'rgba(255,255,255,0.06)',
          color: account.role === 'admin' ? '#a78bfa' : C.text2,
        }}>
          {account.role === 'admin' ? 'Administrateur' : 'Utilisateur'}
        </span>
      </div>

      {/* Profiles */}
      <div style={{ borderTop: `1px solid ${C.border}`, paddingTop: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
          <div style={{ fontSize: T.sm, fontWeight: 600 }}>Mes profils</div>
          <div style={{ fontSize: T.xs, color: C.text3 }}>{profiles.length} / {account.maxProfiles}</div>
        </div>

        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10, marginBottom: 12 }}>
          {profiles.map((profile) => (
            <div key={profile.id} style={{
              display: 'flex', alignItems: 'center', gap: 8,
              padding: '8px 14px', background: C.surf2, borderRadius: R.md,
            }}>
              <div style={{
                width: 28, height: 28, borderRadius: '50%',
                background: '#4b5563', display: 'flex', alignItems: 'center', justifyContent: 'center',
                fontSize: '0.8rem', color: '#fff',
              }}>
                {profile.name[0].toUpperCase()}
              </div>
              <span style={{ fontSize: T.sm }}>{profile.name}</span>
              <button
                onClick={() => handleDeleteProfile(profile.id)}
                style={{ background: 'none', border: 'none', color: '#ef4444', cursor: 'pointer', fontSize: T.xs, padding: '0 4px' }}
              >
                ✕
              </button>
            </div>
          ))}
        </div>

        {creatingProfile ? (
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              value={newProfileName}
              onChange={(e) => setNewProfileName(e.target.value)}
              placeholder="Nom du profil"
              style={{ flex: 1, padding: '8px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm }}
            />
            <button onClick={handleCreateProfile} style={{ padding: '8px 16px', background: '#7c3aed', color: '#fff', border: 'none', borderRadius: R.md, fontSize: T.sm, cursor: 'pointer' }}>Créer</button>
            <button onClick={() => { setCreatingProfile(false); setNewProfileName('') }} style={{ padding: '8px 16px', background: 'transparent', color: C.text2, border: `1px solid ${C.border}`, borderRadius: R.md, fontSize: T.sm, cursor: 'pointer' }}>Annuler</button>
          </div>
        ) : (
          profiles.length < account.maxProfiles && (
            <button
              onClick={() => setCreatingProfile(true)}
              style={{ padding: '8px 16px', background: 'transparent', color: '#7c3aed', border: `1px dashed ${C.border}`, borderRadius: R.md, fontSize: T.sm, cursor: 'pointer' }}
            >
              + Ajouter un profil
            </button>
          )
        )}
      </div>
    </div>
  )
}

// ── System section (admin only) ───────────────────────────────────────────────

function SystemSection() {
  const token = localStorage.getItem('aetherstream_token')

  let isAdmin = false
  if (token) {
    try {
      const payload = JSON.parse(atob(token.split('.')[1]))
      isAdmin = payload.role === 'admin'
    } catch {}
  }

  const [maxJobs, setMaxJobs] = useState(2)
  const [hwaccel, setHwaccel] = useState('auto')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  useEffect(() => {
    if (!isAdmin) { setLoading(false); return }
    apiClient.get<{ ffmpeg_max_jobs: number; ffmpeg_hwaccel: string }>('/api/settings')
      .then((s) => { setMaxJobs(s.ffmpeg_max_jobs); setHwaccel(s.ffmpeg_hwaccel) })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  if (!isAdmin) return null

  async function handleSave() {
    setSaving(true); setError(''); setSuccess('')
    try {
      await apiClient.put('/api/settings', { ffmpeg_max_jobs: maxJobs, ffmpeg_hwaccel: hwaccel })
      setSuccess('Paramètres enregistrés')
    } catch (err: any) {
      setError(err.message || 'Erreur')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Section title="Système" style={{ marginTop: 20 }}>
      {loading ? (
        <div style={{ color: C.text3 }}>Chargement…</div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {error && <Alert type="error">{error}</Alert>}
          {success && <Alert type="success">{success}</Alert>}

          <div>
            <div style={{ fontSize: T.xs, color: C.text2, marginBottom: 4 }}>Jobs FFmpeg simultanés</div>
            <input
              type="number" min={1} max={16}
              value={maxJobs}
              onChange={(e) => setMaxJobs(Number(e.target.value))}
              style={{ width: 80, padding: '8px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm }}
            />
          </div>

          <div>
            <div style={{ fontSize: T.xs, color: C.text2, marginBottom: 4 }}>Accélération matérielle</div>
            <select
              value={hwaccel}
              onChange={(e) => setHwaccel(e.target.value)}
              style={{ padding: '8px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm }}
            >
              <option value="auto">Auto-détection</option>
              <option value="none">Aucune (CPU)</option>
              <option value="vaapi">VAAPI (Intel/AMD)</option>
              <option value="nvenc">NVENC (NVIDIA)</option>
              <option value="qsv">QSV (Intel)</option>
            </select>
          </div>

          <button
            onClick={handleSave}
            disabled={saving}
            style={{ alignSelf: 'flex-start', padding: '8px 20px', background: saving ? C.surf3 : C.accent, color: '#fff', border: 'none', borderRadius: R.md, fontSize: T.sm, fontWeight: 600, cursor: saving ? 'not-allowed' : 'pointer' }}
          >
            {saving ? 'Enregistrement…' : 'Enregistrer'}
          </button>
        </div>
      )}
    </Section>
  )
}

// ── Main component ────────────────────────────────────────────────────────────

export default function Settings() {
  const { logout } = useAuth()
  const [libraries, setLibraries] = useState<any[]>([])
  const [name, setName] = useState('')
  const [path, setPath] = useState('')
  const [mediaType, setMediaType] = useState('movie')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [creating, setCreating] = useState(false)
  const [confirmLogout, setConfirmLogout] = useState(false)
  const token = localStorage.getItem('aetherstream_token')

  useEffect(() => { getLibraries().then(setLibraries).catch(() => {}) }, [])

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setError(''); setSuccess('')
    if (!name.trim() || !path.trim()) { setError('Nom et chemin requis.'); return }
    setCreating(true)
    try {
      const res = await fetch('/api/libraries', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ name: name.trim(), path: path.trim(), media_type: mediaType }),
      })
      if (!res.ok) { setError((await res.text()) || `Erreur ${res.status}`); return }
      setSuccess('Bibliothèque créée, scan en cours…')
      setName(''); setPath(''); setMediaType('movie')
      setTimeout(() => { getLibraries().then(setLibraries).catch(() => {}); setSuccess('') }, 2000)
    } catch { setError('Erreur réseau.') }
    finally { setCreating(false) }
  }

  async function handleScan(id: string) {
    await fetch(`/api/libraries/${id}/scan`, { method: 'POST', headers: { Authorization: `Bearer ${token}` } })
    setSuccess('Scan démarré !')
    setTimeout(() => { getLibraries().then(setLibraries).catch(() => {}); setSuccess('') }, 3000)
  }

  return (
    <div style={{ padding: '32px 28px', maxWidth: 680 }}>
      <h1 style={{ fontSize: T['2xl'], fontWeight: 700, letterSpacing: '-0.02em', marginBottom: 28 }}>Paramètres</h1>

      {/* Account */}
      <Section title="Mon compte">
        <AccountSection />
      </Section>

      {/* System settings (admin only) */}
      <SystemSection />

      {/* Libraries */}
      <Section title="Bibliothèques" style={{ marginTop: 20 }}>
        {libraries.length === 0 ? (
          <div style={{ color: C.text2, fontSize: T.sm, padding: '12px 0' }}>Aucune bibliothèque configurée.</div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginBottom: 16 }}>
            {libraries.map((lib) => (
              <div key={lib.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 14px', background: C.surf2, borderRadius: R.md, border: `1px solid ${C.border}` }}>
                <div>
                  <div style={{ fontWeight: 600, fontSize: T.sm }}>{lib.name}</div>
                  <div style={{ color: C.text2, fontSize: T.xs, marginTop: 2 }}>
                    {lib.path} · {lib.type} · {lib.item_count} éléments
                  </div>
                </div>
                <button
                  onClick={() => handleScan(lib.id)}
                  style={{ padding: '6px 14px', borderRadius: R.md, background: C.surf3, color: C.text, border: `1px solid ${C.border}`, fontSize: T.xs, fontWeight: 600, cursor: 'pointer', transition: 'background 0.15s' }}
                  onMouseEnter={(e) => { e.currentTarget.style.background = C.accent; e.currentTarget.style.color = '#fff' }}
                  onMouseLeave={(e) => { e.currentTarget.style.background = C.surf3; e.currentTarget.style.color = C.text }}
                >
                  Scanner
                </button>
              </div>
            ))}
          </div>
        )}
        <div style={{ borderTop: `1px solid ${C.border}`, paddingTop: 16 }}>
          <div style={{ fontWeight: 600, fontSize: T.sm, marginBottom: 14, color: C.text2 }}>Ajouter une bibliothèque</div>
          {error && <Alert type="error">{error}</Alert>}
          {success && <Alert type="success">{success}</Alert>}
          <form onSubmit={handleCreate} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <SettingInput placeholder="Nom (ex : Films)" value={name} onChange={(e) => setName(e.target.value)} />
            <SettingInput placeholder="Chemin (ex : /media/films)" value={path} onChange={(e) => setPath(e.target.value)} />
            <select
              value={mediaType} onChange={(e) => setMediaType(e.target.value)}
              style={{ padding: '9px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm }}
            >
              <option value="movie">Film</option>
              <option value="show">Série</option>
              <option value="music">Musique</option>
            </select>
            <button type="submit" disabled={creating} style={{ padding: '10px', borderRadius: R.md, background: creating ? C.surf3 : C.accent, color: '#fff', fontWeight: 600, fontSize: T.sm, cursor: creating ? 'not-allowed' : 'pointer', transition: 'background 0.15s', alignSelf: 'flex-start', minWidth: 160 }}>
              {creating ? 'Création…' : 'Créer la bibliothèque'}
            </button>
          </form>
        </div>
      </Section>

      {/* Tasks */}
      <Section title="Tâches de transcodage" style={{ marginTop: 20 }}>
        <JobsSection />
      </Section>

      {/* Storage */}
      <Section title="Stockage des transcodes" style={{ marginTop: 20 }}>
        <StorageSection />
      </Section>

      {/* Logout */}
      <Section title="Session" style={{ marginTop: 20 }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontSize: T.sm, fontWeight: 500 }}>Déconnexion</div>
            <div style={{ color: C.text2, fontSize: T.xs, marginTop: 2 }}>Fermer la session en cours</div>
          </div>
          <button
            onClick={() => { if (confirmLogout) logout(); else setConfirmLogout(true) }}
            style={{ padding: '8px 16px', borderRadius: R.md, background: confirmLogout ? '#ef4444' : C.surf3, color: '#fff', border: 'none', fontSize: T.sm, fontWeight: 600, cursor: 'pointer', transition: 'background 0.15s', minWidth: 120 }}
          >
            {confirmLogout ? 'Confirmer' : 'Se déconnecter'}
          </button>
        </div>
        {confirmLogout && (
          <div style={{ color: C.text2, fontSize: T.xs, marginTop: 8 }}>Cliquez à nouveau pour confirmer la déconnexion.</div>
        )}
      </Section>
    </div>
  )
}

// ── Shared components ─────────────────────────────────────────────────────────

function Section({ title, children, style }: { title: string; children: React.ReactNode; style?: React.CSSProperties }) {
  return (
    <section style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: R.lg, padding: '20px 20px', ...style }}>
      <h2 style={{ fontSize: T.base, fontWeight: 600, marginBottom: 16, paddingBottom: 12, borderBottom: `1px solid ${C.border}` }}>{title}</h2>
      {children}
    </section>
  )
}

function SettingInput(props: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input {...props} style={{ padding: '9px 12px', background: C.surf2, border: `1px solid ${C.border}`, borderRadius: R.md, color: C.text, fontSize: T.sm, width: '100%', transition: 'border-color 0.15s', ...props.style }}
      onFocus={(e) => { e.target.style.borderColor = 'rgba(124,58,237,0.5)' }}
      onBlur={(e) => { e.target.style.borderColor = C.border }}
    />
  )
}

function Alert({ type, children }: { type: 'error' | 'success'; children: React.ReactNode }) {
  const isErr = type === 'error'
  return (
    <div style={{ background: isErr ? 'rgba(239,68,68,0.1)' : 'rgba(34,197,94,0.1)', border: `1px solid ${isErr ? 'rgba(239,68,68,0.3)' : 'rgba(34,197,94,0.3)'}`, borderRadius: R.sm, padding: '8px 12px', color: isErr ? '#ef4444' : '#22c55e', fontSize: T.xs, marginBottom: 10 }}>
      {children}
    </div>
  )
}
