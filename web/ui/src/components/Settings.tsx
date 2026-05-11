import React, { useEffect, useRef, useState } from 'react'
import { useAuth } from '../hooks/useAuth.ts'
import {
  getLibraries, getJobs, cancelJob, getTranscodeDirs, deleteTranscodeDir,
  type Job, type TranscodeDir,
} from '../api.ts'
import { C, R, T } from '../design.ts'

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

      {/* Libraries */}
      <Section title="Bibliothèques">
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

      {/* Account */}
      <Section title="Compte" style={{ marginTop: 20 }}>
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
