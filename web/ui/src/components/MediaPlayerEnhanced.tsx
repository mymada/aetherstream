import React, { useEffect, useRef, useState, useCallback, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import Hls from 'hls.js'
import {
  getItem, getHlsUrl, getStreamUrl,
  getSubtitleTracks, getSubtitleVttUrl,
  getAudioTracks, getHlsUrlWithAudio, getHlsUrlWithStart,
  type SubtitleTrack, type AudioTrack,
} from '../api.ts'
import { apiClient } from '../api/client.ts'
import { saveProgress, loadProgress } from '../progress.ts'
import { C, R, T } from '../design.ts'

// ─── Helpers ────────────────────────────────────────────────────────────────

function fmtTime(s: number): string {
  if (!isFinite(s) || isNaN(s)) return '0:00'
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), sec = Math.floor(s % 60)
  return h > 0 ? `${h}:${pad2(m)}:${pad2(sec)}` : `${m}:${pad2(sec)}`
}
function pad2(n: number) { return String(n).padStart(2, '0') }

const LANG: Record<string, string> = {
  fre: 'Français', fra: 'Français', eng: 'English', spa: 'Español',
  deu: 'Deutsch', ger: 'Deutsch', ita: 'Italiano', por: 'Português',
  jpn: '日本語', zho: '中文', chi: '中文', kor: '한국어', und: '?',
}
const langName = (c: string) => LANG[c?.toLowerCase()] ?? c?.toUpperCase() ?? '?'

function subLabel(t: SubtitleTrack) {
  const base = langName(t.language)
  if (t.title) return `${base} — ${t.title}`
  return base
}
function audioLabel(t: AudioTrack) {
  const base = langName(t.language)
  if (t.title) return `${base} — ${t.title}`
  return base
}

// ─── VTT parser (pure) ──────────────────────────────────────────────────────
type Cue = { start: number; end: number; text: string }

function parseVTT(vtt: string): Cue[] {
  const cues: Cue[] = []
  const lines = vtt.split('\n')
  let i = 0
  while (i < lines.length) {
    if (lines[i].includes('-->')) {
      const arrow = lines[i].indexOf('-->')
      const start = parseTime(lines[i].slice(0, arrow).trim())
      const end = parseTime(lines[i].slice(arrow + 3).trim().split(/\s/)[0])
      i++
      const parts: string[] = []
      while (i < lines.length && lines[i].trim() !== '') { parts.push(lines[i]); i++ }
      if (parts.length && isFinite(start) && isFinite(end) && end > start)
        cues.push({ start, end, text: parts.join('\n') })
    } else { i++ }
  }
  return cues
}
function parseTime(t: string): number {
  const p = t.trim().split(':')
  if (p.length === 3) return +p[0] * 3600 + +p[1] * 60 + parseFloat(p[2])
  if (p.length === 2) return +p[0] * 60 + parseFloat(p[1])
  return NaN
}

// ─── Constants ───────────────────────────────────────────────────────────────
type Level = { index: number; label: string }
const SPEEDS = [0.5, 0.75, 1, 1.25, 1.5, 2]

// ─── Component ───────────────────────────────────────────────────────────────
export default function MediaPlayerEnhanced() {
  const { id } = useParams()
  const navigate = useNavigate()

  // Refs
  const videoRef = useRef<HTMLVideoElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const hlsRef = useRef<Hls | null>(null)
  const hideTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const retryTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const seekAfterLoad = useRef<number>(0)
  const streamOffsetRef = useRef(0)
  const itemRef = useRef<any>(null)
  const playbackRateRef = useRef(1)
  const loadedCues = useRef<Map<number, Cue[]>>(new Map())

  // Playback state
  const [item, setItem] = useState<any>(null)
  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [buffered, setBuffered] = useState(0)
  const [volume, setVolume] = useState(1)
  const [muted, setMuted] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [streamOffset, setStreamOffset] = useState(0)
  const [playbackRate, setPlaybackRate] = useState(1)

  // HLS levels
  const [levels, setLevels] = useState<Level[]>([])
  const [currentLevel, setCurrentLevel] = useState(-1)

  // Track data from probe
  const [subTracks, setSubTracks] = useState<SubtitleTrack[]>([])
  const [audioTracks, setAudioTracks] = useState<AudioTrack[]>([])

  // Selections
  const [activeSub, setActiveSub] = useState<number | 'off'>('off')
  const [activeAudio, setActiveAudio] = useState(0)
  const [subLoading, setSubLoading] = useState(false)

  // UI
  const [showControls, setShowControls] = useState(true)
  const [showSettings, setShowSettings] = useState(false)
  const [settingsSection, setSettingsSection] = useState<'sub' | 'audio' | 'quality'>('sub')
  const [, setIsFullscreen] = useState(false)
  const [resumePos, setResumePos] = useState<number | null>(null)

  // Chapters
  const [chapters, setChapters] = useState<{ name: string; position: number }[]>([])
  const [showChapters, setShowChapters] = useState(false)

  // Trickplay
  const [, setTrickplayVTT] = useState<string>('')

  // Keep refs in sync
  useEffect(() => { streamOffsetRef.current = streamOffset }, [streamOffset])
  useEffect(() => { itemRef.current = item }, [item])
  useEffect(() => { playbackRateRef.current = playbackRate }, [playbackRate])

  // Apply playback rate to video element
  useEffect(() => {
    const v = videoRef.current
    if (v) v.playbackRate = playbackRate
  }, [playbackRate])

  // ── Fetch chapters + trickplay ────────────────────────────────────────────
  useEffect(() => {
    if (!id) return
    apiClient.listChapters(id).then((ch) => {
      if (ch && ch.length > 0) setChapters(ch)
    }).catch(() => {})
    apiClient.getTrickplay(id).then((tp) => {
      if (tp && tp.vtt) setTrickplayVTT(tp.vtt)
    }).catch(() => {})
  }, [id])

  // ── HLS lifecycle ─────────────────────────────────────────────────────────
  const destroyHls = useCallback(() => {
    hlsRef.current?.destroy()
    hlsRef.current = null
  }, [])

  const startHls = useCallback((audioIdx: number, startSec?: number) => {
    const video = videoRef.current
    if (!video || !id) return
    destroyHls()
    setActiveSub('off')
    loadedCues.current.clear()
    setLoading(true)

    const offset = startSec ?? 0
    setStreamOffset(offset)
    seekAfterLoad.current = 0

    let url: string
    if (audioIdx > 0) {
      url = getHlsUrlWithAudio(id, audioIdx, offset)
    } else if (offset > 0) {
      url = getHlsUrlWithStart(id, offset)
    } else {
      url = getHlsUrl(id)
    }
    console.log(`[AetherStream] startHls audioIdx=${audioIdx} start=${offset} url=${url}`)

    if (Hls.isSupported()) {
      const hls = new Hls({ enableWorker: true, backBufferLength: 90 })
      hlsRef.current = hls
      hls.on(Hls.Events.MANIFEST_PARSED, (_e, data) => {
        console.log('[AetherStream] MANIFEST_PARSED levels:', data.levels.map((l) => l.height + 'p'))
        setLoading(false)
        setLevels(data.levels.map((l, i) => ({ index: i, label: l.height ? `${l.height}p` : `Q${i + 1}` })))
        if (seekAfterLoad.current > 0) {
          video.currentTime = seekAfterLoad.current
          seekAfterLoad.current = 0
        }
        video.playbackRate = playbackRateRef.current
        video.play().catch(() => {})
        setPlaying(true)
      })
      hls.on(Hls.Events.LEVEL_SWITCHED, (_e, data) => setCurrentLevel(data.level))
      hls.on(Hls.Events.ERROR, (_e, data) => {
        const status = (data.response as any)?.code
        console.warn(`[AetherStream] HLS error fatal=${data.fatal} type=${data.type} details=${data.details} status=${status}`, data)
        if (data.fatal) {
          if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
            hls.recoverMediaError()
          } else {
            console.log('[AetherStream] fatal error, fallback to direct stream')
            setLoading(false)
            destroyHls()
            video.src = getStreamUrl(id)
            video.play().catch(() => {})
          }
        }
      })
      hls.loadSource(url)
      hls.attachMedia(video)
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      console.log('[AetherStream] native HLS')
      setLoading(false)
      video.src = url; video.play().catch(() => {}); setPlaying(true)
    } else {
      console.log('[AetherStream] no HLS support, direct stream fallback')
      setLoading(false)
      video.src = getStreamUrl(id); video.play().catch(() => {}); setPlaying(true)
    }
  }, [id, destroyHls])

  // Load tracks + start HLS, resume from saved progress if available
  useEffect(() => {
    if (!id) return
    let cancelled = false

    const savedProg = loadProgress(id)
    const resumeFrom = savedProg && savedProg.position > 15 ? savedProg.position : 0
    if (resumeFrom > 15) setResumePos(resumeFrom)

    getItem(id).then(setItem).catch(() => {})
    getSubtitleTracks(id).then((tracks) => {
      if (cancelled) return
      setSubTracks(tracks)
    }).catch((e) => console.error('[AetherStream] subtitle tracks error:', e))
    getAudioTracks(id).then((tracks) => {
      if (cancelled) return
      setAudioTracks(tracks)
      const defTrack = tracks.find((t) => t.default)
      const initAudio = defTrack ? defTrack.sub_index : 0
      setActiveAudio(initAudio)
      startHls(initAudio, resumeFrom > 0 ? resumeFrom : undefined)
    }).catch((e) => {
      console.error('[AetherStream] audio tracks error:', e)
      if (!cancelled) startHls(0, resumeFrom > 0 ? resumeFrom : undefined)
    })
    return () => {
      cancelled = true
      if (retryTimer.current) clearTimeout(retryTimer.current)
      destroyHls()
      const v = videoRef.current
      if (v) { v.pause(); v.removeAttribute('src'); v.load() }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  // Auto-dismiss resume notification after 5s
  useEffect(() => {
    if (resumePos === null) return
    const t = setTimeout(() => setResumePos(null), 5000)
    return () => clearTimeout(t)
  }, [resumePos])

  // Save progress every 5s while playing + API sync
  useEffect(() => {
    if (!id) return
    const interval = setInterval(() => {
      const v = videoRef.current
      if (!v || v.currentTime < 5) return
      const realPos = v.currentTime + streamOffsetRef.current
      const dur = itemRef.current?.duration ?? 0
      if (realPos > 5 && dur > 0) {
        saveProgress(id, realPos, dur)
        apiClient.saveProgress(id, realPos, dur).catch(() => {})
      }
    }, 5000)
    return () => clearInterval(interval)
  }, [id])

  // Save progress on unmount / navigation away
  useEffect(() => {
    return () => {
      if (!id) return
      const v = videoRef.current
      if (!v || v.currentTime < 5) return
      const realPos = v.currentTime + streamOffsetRef.current
      const dur = itemRef.current?.duration ?? 0
      if (realPos > 5 && dur > 0) {
        saveProgress(id, realPos, dur)
        apiClient.saveProgress(id, realPos, dur).catch(() => {})
      }
    }
  }, [id])

  // ── Video events ──────────────────────────────────────────────────────────
  useEffect(() => {
    const v = videoRef.current
    if (!v) return
    const onTime = () => {
      setCurrentTime(v.currentTime)
      if (v.buffered.length && v.duration)
        setBuffered(v.buffered.end(v.buffered.length - 1) / v.duration * 100)
    }
    const onDur = () => setDuration(v.duration || 0)
    const onPlay = () => setPlaying(true)
    const onPause = () => setPlaying(false)
    const onErr = () => setError('Erreur de lecture.')
    v.addEventListener('timeupdate', onTime)
    v.addEventListener('durationchange', onDur)
    v.addEventListener('play', onPlay)
    v.addEventListener('pause', onPause)
    v.addEventListener('error', onErr)
    return () => {
      v.removeEventListener('timeupdate', onTime)
      v.removeEventListener('durationchange', onDur)
      v.removeEventListener('play', onPlay)
      v.removeEventListener('pause', onPause)
      v.removeEventListener('error', onErr)
    }
  }, [])

  useEffect(() => {
    const onFs = () => setIsFullscreen(!!document.fullscreenElement)
    document.addEventListener('fullscreenchange', onFs)
    return () => document.removeEventListener('fullscreenchange', onFs)
  }, [])

  // ── Controls auto-hide ────────────────────────────────────────────────────
  const resetHide = useCallback(() => {
    setShowControls(true)
    if (hideTimer.current) clearTimeout(hideTimer.current)
    hideTimer.current = setTimeout(() => { setShowControls(false); setShowSettings(false); setShowChapters(false) }, 3500)
  }, [])

  // ── Subtitle loading ──────────────────────────────────────────────────────
  const activateSubtitle = useCallback(async (idx: number | 'off') => {
    setShowSettings(false)
    if (idx === 'off') { setActiveSub('off'); return }
    setActiveSub(idx)
    if (loadedCues.current.has(idx)) return
    setSubLoading(true)
    try {
      const res = await fetch(getSubtitleVttUrl(id!, idx))
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const text = await res.text()
      loadedCues.current.set(idx, parseVTT(text))
    } catch (e) {
      console.error('[AetherStream] subtitle error:', e)
      setActiveSub('off')
    } finally {
      setSubLoading(false)
    }
  }, [id])

  // ── Active subtitle cue lookup ────────────────────────────────────────────
  const activeCueText: string | null = useMemo(() => {
    if (activeSub === 'off') return null
    const cues = loadedCues.current.get(activeSub as number)
    if (!cues) return null
    return cues.find((c) => currentTime >= c.start && currentTime < c.end)?.text ?? null
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentTime, activeSub])

  // ── Audio switch ──────────────────────────────────────────────────────────
  const switchAudio = useCallback((idx: number) => {
    const realPos = (videoRef.current?.currentTime ?? 0) + streamOffsetRef.current
    setActiveAudio(idx)
    startHls(idx, realPos)
    setShowSettings(false)
  }, [startHls])

  // ── Chapter jump ──────────────────────────────────────────────────────────
  const jumpToChapter = useCallback((pos: number) => {
    const v = videoRef.current
    if (!v) return
    const targetRelative = pos - streamOffsetRef.current
    if (targetRelative < 0) {
      startHls(activeAudio, pos)
    } else {
      v.currentTime = targetRelative
      setCurrentTime(targetRelative)
    }
    setShowChapters(false)
  }, [startHls, activeAudio])

  // ── Playback controls ─────────────────────────────────────────────────────
  const togglePlay = useCallback(() => {
    const v = videoRef.current; if (!v) return
    v.paused ? v.play().catch(() => {}) : v.pause()
  }, [])

  const seek = useCallback((delta: number) => {
    const v = videoRef.current; if (!v) return
    v.currentTime = Math.max(0, Math.min(v.currentTime + delta, v.duration || 0))
  }, [])

  const onSeekBar = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const v = videoRef.current; if (!v) return
    const totalDuration = itemRef.current?.duration ?? (v.duration || 0) + streamOffsetRef.current
    if (!totalDuration) return
    const targetAbsolute = parseFloat(e.target.value) / 100 * totalDuration
    const targetRelative = targetAbsolute - streamOffsetRef.current
    if (targetRelative < 0) {
      startHls(activeAudio, targetAbsolute)
      return
    }
    v.currentTime = targetRelative
    setCurrentTime(targetRelative)
  }, [startHls, activeAudio])

  const onVolume = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const val = parseFloat(e.target.value)
    setVolume(val); setMuted(val === 0)
    if (videoRef.current) { videoRef.current.volume = val; videoRef.current.muted = val === 0 }
  }, [])

  const toggleMute = useCallback(() => {
    const v = videoRef.current; if (!v) return
    v.muted = !v.muted; setMuted(v.muted)
    if (!v.muted && volume === 0) { v.volume = 0.5; setVolume(0.5) }
  }, [volume])

  const toggleFs = useCallback(() => {
    const el = containerRef.current; if (!el) return
    document.fullscreenElement ? document.exitFullscreen() : el.requestFullscreen?.()
  }, [])

  const setQuality = useCallback((lvl: number) => {
    setCurrentLevel(lvl)
    if (hlsRef.current) hlsRef.current.currentLevel = lvl
  }, [])

  const cycleSpeed = useCallback(() => {
    const idx = SPEEDS.indexOf(playbackRateRef.current)
    const next = SPEEDS[(idx + 1) % SPEEDS.length]
    setPlaybackRate(next)
    playbackRateRef.current = next
    const v = videoRef.current
    if (v) v.playbackRate = next
  }, [])

  // ── Keyboard shortcuts ────────────────────────────────────────────────────
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.target as HTMLElement).tagName === 'INPUT') return
      switch (e.key) {
        case ' ': case 'k': e.preventDefault(); togglePlay(); break
        case 'ArrowRight': seek(10); break
        case 'ArrowLeft': seek(-10); break
        case 'ArrowUp': e.preventDefault(); { const v = videoRef.current; if (v) { const nv = Math.min(1, v.volume + 0.1); v.volume = nv; setVolume(nv); setMuted(false) } } break
        case 'ArrowDown': e.preventDefault(); { const v = videoRef.current; if (v) { const nv = Math.max(0, v.volume - 0.1); v.volume = nv; setVolume(nv) } } break
        case 'f': toggleFs(); break
        case 'm': toggleMute(); break
        case 's': setShowSettings((p) => !p); break
        case 'c': setShowChapters((p) => !p); break
        case 'Escape': setShowSettings(false); setShowChapters(false); break
        case '>': {
          const ci = SPEEDS.indexOf(playbackRateRef.current)
          if (ci < SPEEDS.length - 1) {
            const nr = SPEEDS[ci + 1]; setPlaybackRate(nr); playbackRateRef.current = nr
            const v = videoRef.current; if (v) v.playbackRate = nr
          }
          break
        }
        case '<': {
          const ci = SPEEDS.indexOf(playbackRateRef.current)
          if (ci > 0) {
            const nr = SPEEDS[ci - 1]; setPlaybackRate(nr); playbackRateRef.current = nr
            const v = videoRef.current; if (v) v.playbackRate = nr
          }
          break
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [togglePlay, seek, toggleFs, toggleMute])

  // ── Derived values ────────────────────────────────────────────────────────
  const realCurrentTime = currentTime + streamOffset
  const totalDuration = item?.duration ?? (duration + streamOffset)
  const progress = totalDuration > 0 ? realCurrentTime / totalDuration * 100 : 0
  const hasSub = subTracks.length > 0
  const hasAudio = audioTracks.length > 1
  const hasQuality = levels.length > 0
  const hasSettings = hasSub || hasAudio || hasQuality
  const hasChapters = chapters.length > 0
  // const activeSubTrack = subTracks.find((t) => t.sub_index === activeSub)
  // const activeAudioTrack = audioTracks[activeAudio]

  const sections: ('sub' | 'audio' | 'quality')[] = [
    ...(hasSub ? ['sub' as const] : []),
    ...(hasAudio ? ['audio' as const] : []),
    ...(hasQuality ? ['quality' as const] : []),
  ]
  const currentSection = sections.includes(settingsSection) ? settingsSection : sections[0]

  return (
    <div
      ref={containerRef}
      onMouseMove={resetHide}
      onClick={() => { showSettings && setShowSettings(false); showChapters && setShowChapters(false) }}
      onMouseLeave={() => { if (hideTimer.current) clearTimeout(hideTimer.current); setShowControls(false) }}
      style={{
        position: 'relative', width: '100%', height: '100vh',
        background: '#000', overflow: 'hidden',
        cursor: showControls ? 'default' : 'none',
        userSelect: 'none',
      }}
    >
      {/* ── Video ─────────────────────────────────────────────────── */}
      <video
        ref={videoRef}
        onClick={(e) => { e.stopPropagation(); togglePlay() }}
        autoPlay playsInline
        style={{ width: '100%', height: '100%', objectFit: 'contain', display: 'block' }}
      />

      {/* ── Subtitle overlay ──────────────────────────────────────── */}
      {activeCueText && (
        <div
          style={{
            position: 'absolute',
            bottom: showControls ? 96 : 40,
            left: '50%', transform: 'translateX(-50%)',
            transition: 'bottom 0.25s ease',
            maxWidth: '78%', textAlign: 'center',
            pointerEvents: 'none', zIndex: 18,
          }}
        >
          {activeCueText.split('\n').map((line, i) => (
            <span key={i} style={{ display: 'block' }}>
              <span style={{
                display: 'inline',
                background: 'rgba(0,0,0,0.82)',
                color: '#fff',
                fontSize: '1.15rem',
                lineHeight: 1.55,
                padding: '3px 10px',
                borderRadius: 3,
                boxDecorationBreak: 'clone' as any,
                WebkitBoxDecorationBreak: 'clone',
                fontWeight: 500,
              }}>
                {line}
              </span>
            </span>
          ))}
        </div>
      )}

      {/* ── Loading overlay ───────────────────────────────────────── */}
      {loading && (
        <div style={{
          position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%,-50%)',
          display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 14,
          zIndex: 30, pointerEvents: 'none',
        }}>
          <div style={{
            width: 48, height: 48, borderRadius: R.full,
            border: '3px solid rgba(255,255,255,0.12)',
            borderTopColor: C.accent,
            animation: 'spin 0.9s linear infinite',
          }} />
          <div style={{ color: '#fff', fontWeight: 600, fontSize: T.base }}>Préparation en cours…</div>
          <div style={{ color: 'rgba(255,255,255,0.45)', fontSize: T.xs }}>Transcodage et mise en tampon des segments</div>
        </div>
      )}

      {/* ── Error ─────────────────────────────────────────────────── */}
      {error && (
        <div style={{
          position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%,-50%)',
          background: 'rgba(0,0,0,0.85)', color: '#ff6b6b', padding: '16px 28px',
          borderRadius: R.lg, zIndex: 30, fontSize: T.base, textAlign: 'center',
        }}>{error}</div>
      )}

      {/* ── Pause overlay ─────────────────────────────────────────── */}
      {!playing && !loading && !error && (
        <div
          onClick={(e) => { e.stopPropagation(); togglePlay() }}
          style={{
            position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%,-50%)',
            width: 72, height: 72, borderRadius: R.full,
            background: 'rgba(0,0,0,0.6)', backdropFilter: 'blur(8px)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer', zIndex: 15,
          }}
        >
          <span style={{ color: '#fff', fontSize: '1.6rem', marginLeft: 4 }}>▶</span>
        </div>
      )}

      {/* ── Resume notification ───────────────────────────────────── */}
      {resumePos !== null && (
        <div
          onClick={(e) => e.stopPropagation()}
          style={{
            position: 'absolute', bottom: 96, left: 20, zIndex: 22,
            background: 'rgba(8,8,14,0.9)', backdropFilter: 'blur(12px)',
            border: '1px solid rgba(255,255,255,0.1)',
            borderRadius: R.lg, padding: '10px 14px',
            display: 'flex', alignItems: 'center', gap: 12,
            fontSize: T.xs, color: '#fff',
            animation: 'fadeIn 0.3s ease',
          }}
        >
          <span style={{ color: 'rgba(255,255,255,0.75)' }}>Reprise depuis <strong style={{ color: '#fff' }}>{fmtTime(resumePos)}</strong></span>
          <button
            onClick={() => { startHls(activeAudio, 0); setStreamOffset(0); setResumePos(null) }}
            style={{
              background: 'rgba(255,255,255,0.1)', border: '1px solid rgba(255,255,255,0.18)',
              color: '#fff', borderRadius: R.sm, padding: '3px 10px',
              cursor: 'pointer', fontSize: T.xs, fontWeight: 600,
            }}
          >Recommencer</button>
          <button
            onClick={() => setResumePos(null)}
            style={{
              background: 'none', border: 'none',
              color: 'rgba(255,255,255,0.4)', cursor: 'pointer', fontSize: '1rem',
              padding: '0 2px', lineHeight: 1,
            }}
          >×</button>
        </div>
      )}

      {/* ── Top bar ───────────────────────────────────────────────── */}
      <div style={{
        position: 'absolute', top: 0, left: 0, right: 0, zIndex: 20,
        padding: '16px 20px',
        background: 'linear-gradient(to bottom, rgba(0,0,0,0.75) 0%, transparent 100%)',
        display: 'flex', alignItems: 'center', gap: 12,
        opacity: showControls ? 1 : 0,
        transition: 'opacity 0.25s',
        pointerEvents: showControls ? 'auto' : 'none',
      }}>
        <button
          onClick={() => navigate(-1)}
          style={{ width: 36, height: 36, borderRadius: R.full, background: 'rgba(255,255,255,0.12)', backdropFilter: 'blur(6px)', color: '#fff', fontSize: '1rem', display: 'flex', alignItems: 'center', justifyContent: 'center', border: 'none', cursor: 'pointer', flexShrink: 0 }}
        >←</button>
        <div>
          <div style={{ color: '#fff', fontWeight: 600, fontSize: T.base, lineHeight: 1.2 }}>{item?.title ?? ''}</div>
          {item?.year && <div style={{ color: 'rgba(255,255,255,0.45)', fontSize: T.xs }}>{item.year}</div>}
        </div>
        {hasChapters && (
          <button
            onClick={(e) => { e.stopPropagation(); setShowChapters((p) => !p) }}
            style={{
              marginLeft: 'auto', background: 'rgba(255,255,255,0.12)', border: 'none',
              color: '#fff', borderRadius: R.sm, padding: '4px 10px', cursor: 'pointer',
              fontSize: T.xs, fontWeight: 600,
            }}
          >Chapitres</button>
        )}
      </div>

      {/* ── Chapters panel ────────────────────────────────────────── */}
      {showChapters && hasChapters && (
        <div
          onClick={(e) => e.stopPropagation()}
          style={{
            position: 'absolute', right: 16, top: 60, zIndex: 25,
            width: 240, maxHeight: 300, overflowY: 'auto',
            background: 'rgba(10,10,18,0.97)',
            backdropFilter: 'blur(20px)',
            border: '1px solid rgba(255,255,255,0.1)',
            borderRadius: R.lg,
            boxShadow: '0 16px 60px rgba(0,0,0,0.8)',
          }}
        >
          <div style={{ padding: '10px 14px', borderBottom: '1px solid rgba(255,255,255,0.08)', fontSize: T.xs, fontWeight: 700, color: C.text3, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
            Chapitres
          </div>
          {chapters.map((ch, i) => (
            <button
              key={i}
              onClick={() => jumpToChapter(ch.position)}
              style={{
                width: '100%', textAlign: 'left', padding: '8px 14px',
                background: 'none', border: 'none', cursor: 'pointer',
                color: realCurrentTime >= ch.position && (chapters[i + 1] ? realCurrentTime < chapters[i + 1].position : true) ? '#fff' : C.text2,
                fontSize: T.sm, fontWeight: realCurrentTime >= ch.position && (chapters[i + 1] ? realCurrentTime < chapters[i + 1].position : true) ? 600 : 400,
                transition: 'background 0.1s',
              }}
              onMouseEnter={(e) => { e.currentTarget.style.background = 'rgba(255,255,255,0.06)' }}
              onMouseLeave={(e) => { e.currentTarget.style.background = 'none' }}
            >
              <span style={{ color: C.text3, fontSize: T.xs, marginRight: 8 }}>{fmtTime(ch.position)}</span>
              {ch.name}
            </button>
          ))}
        </div>
      )}

      {/* ── Settings panel ────────────────────────────────────────── */}
      {showSettings && hasSettings && (
        <div
          onClick={(e) => e.stopPropagation()}
          style={{
            position: 'absolute', right: 16, bottom: 84, zIndex: 25,
            width: 272, background: 'rgba(10,10,18,0.97)',
            backdropFilter: 'blur(20px)',
            border: '1px solid rgba(255,255,255,0.1)',
            borderRadius: R.lg,
            boxShadow: '0 16px 60px rgba(0,0,0,0.8)',
            overflow: 'hidden',
          }}
        >
          {sections.length > 1 && (
            <div style={{ display: 'flex', borderBottom: '1px solid rgba(255,255,255,0.08)' }}>
              {sections.map((sec) => (
                <button key={sec} onClick={() => setSettingsSection(sec)} style={{
                  flex: 1, padding: '10px 6px', background: 'none', border: 'none', cursor: 'pointer',
                  color: currentSection === sec ? '#fff' : 'rgba(255,255,255,0.38)',
                  fontSize: '0.7rem', fontWeight: 700, letterSpacing: '0.05em', textTransform: 'uppercase',
                  borderBottom: `2px solid ${currentSection === sec ? C.accent : 'transparent'}`,
                  transition: 'color 0.15s',
                }}>
                  {sec === 'sub' ? 'Sous-titres' : sec === 'audio' ? 'Audio' : 'Qualité'}
                </button>
              ))}
            </div>
          )}

          <div style={{ padding: '6px 0 8px', maxHeight: 320, overflowY: 'auto' }}>
            {currentSection === 'sub' && (
              <>
                {subLoading && (
                  <div style={{ padding: '8px 16px', color: 'rgba(255,255,255,0.35)', fontSize: T.xs }}>
                    Chargement…
                  </div>
                )}
                <PanelRow label="Désactivés" active={activeSub === 'off'} onClick={() => activateSubtitle('off')} />
                {subTracks.map((t) => (
                  <PanelRow
                    key={t.sub_index}
                    label={subLabel(t)}
                    active={activeSub === t.sub_index}
                    badge={t.forced ? 'Forcé' : t.default ? 'Défaut' : undefined}
                    onClick={() => activateSubtitle(t.sub_index)}
                  />
                ))}
              </>
            )}
            {currentSection === 'audio' && audioTracks.map((t) => (
              <PanelRow
                key={t.sub_index}
                label={audioLabel(t)}
                active={activeAudio === t.sub_index}
                badge={t.default ? 'Défaut' : undefined}
                meta={`${t.codec.toUpperCase()} · ${t.channels}ch`}
                onClick={() => switchAudio(t.sub_index)}
              />
            ))}
            {currentSection === 'quality' && (
              <>
                <PanelRow label="Auto" active={currentLevel === -1} onClick={() => setQuality(-1)} />
                {levels.map((l) => (
                  <PanelRow key={l.index} label={l.label} active={currentLevel === l.index} onClick={() => setQuality(l.index)} />
                ))}
              </>
            )}
          </div>
        </div>
      )}

      {/* ── Bottom controls ───────────────────────────────────────── */}
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          position: 'absolute', bottom: 0, left: 0, right: 0, zIndex: 20,
          background: 'linear-gradient(to top, rgba(0,0,0,0.85) 0%, transparent 100%)',
          opacity: showControls ? 1 : 0,
          transition: 'opacity 0.25s',
          pointerEvents: showControls ? 'auto' : 'none',
          paddingBottom: 10,
        }}
      >
        {/* Progress bar */}
        <div style={{ position: 'relative', padding: '10px 16px 4px', cursor: 'pointer' }}>
          <div style={{
            position: 'absolute', left: 16, right: 16, top: '50%', height: 3,
            background: 'rgba(255,255,255,0.15)', borderRadius: 2,
            transform: 'translateY(-50%)',
          }}>
            <div style={{
              height: '100%', width: `${progress}%`,
              background: '#e50914', borderRadius: 2,
              transition: 'width 0.1s linear',
            }} />
            <div style={{
              position: 'absolute', left: 0, top: 0, height: '100%',
              width: `${buffered}%`, background: 'rgba(255,255,255,0.25)', borderRadius: 2,
            }} />
          </div>
          <input
            type="range"
            min={0} max={100} step={0.1}
            value={progress}
            onChange={onSeekBar}
            style={{
              position: 'relative', zIndex: 2, width: '100%', opacity: 0, cursor: 'pointer',
              height: 20, margin: 0, padding: 0,
            }}
          />
        </div>

        {/* Controls row */}
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          padding: '0 16px 6px',
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <button onClick={togglePlay} style={{
              width: 32, height: 32, borderRadius: R.full,
              background: 'rgba(255,255,255,0.1)', border: 'none', cursor: 'pointer',
              color: '#fff', fontSize: '0.85rem', display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>{playing ? '⏸' : '▶'}</button>
            <button onClick={() => seek(-10)} style={{
              width: 28, height: 28, borderRadius: R.full,
              background: 'none', border: 'none', cursor: 'pointer',
              color: 'rgba(255,255,255,0.7)', fontSize: '0.75rem',
            }}>⏪</button>
            <button onClick={() => seek(10)} style={{
              width: 28, height: 28, borderRadius: R.full,
              background: 'none', border: 'none', cursor: 'pointer',
              color: 'rgba(255,255,255,0.7)', fontSize: '0.75rem',
            }}>⏩</button>

            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginLeft: 4 }}>
              <button onClick={toggleMute} style={{
                width: 28, height: 28, borderRadius: R.full,
                background: 'none', border: 'none', cursor: 'pointer',
                color: 'rgba(255,255,255,0.7)', fontSize: '0.75rem',
              }}>{muted ? '🔇' : volume < 0.5 ? '🔉' : '🔊'}</button>
              <input
                type="range" min={0} max={1} step={0.05}
                value={muted ? 0 : volume}
                onChange={onVolume}
                style={{ width: 72, accentColor: '#e50914' }}
              />
            </div>

            <span style={{ color: 'rgba(255,255,255,0.6)', fontSize: T.xs, fontVariantNumeric: 'tabular-nums' }}>
              {fmtTime(realCurrentTime)} / {fmtTime(totalDuration)}
            </span>

            {/* Speed control */}
            <button
              onClick={cycleSpeed}
              style={{
                background: 'none', border: '1px solid rgba(255,255,255,0.15)',
                color: 'rgba(255,255,255,0.7)', borderRadius: R.sm,
                padding: '2px 8px', cursor: 'pointer', fontSize: T.xs, fontWeight: 600,
                marginLeft: 4,
              }}
            >
              {playbackRate}x
            </button>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            {hasSettings && (
              <button
                onClick={() => setShowSettings((p) => !p)}
                style={{
                  width: 32, height: 32, borderRadius: R.full,
                  background: showSettings ? 'rgba(255,255,255,0.2)' : 'rgba(255,255,255,0.1)',
                  border: 'none', cursor: 'pointer', color: '#fff', fontSize: '0.85rem',
                  transition: 'background 0.15s',
                }}
              >⚙</button>
            )}
            <button
              onClick={toggleFs}
              style={{
                width: 32, height: 32, borderRadius: R.full,
                background: 'rgba(255,255,255,0.1)', border: 'none', cursor: 'pointer',
                color: '#fff', fontSize: '0.85rem',
              }}
            >⛶</button>
          </div>
        </div>
      </div>
    </div>
  )
}

// ─── Panel row helper ────────────────────────────────────────────────────────
function PanelRow({ label, active, onClick, badge, meta }: {
  label: string
  active?: boolean
  onClick: () => void
  badge?: string
  meta?: string
}) {
  return (
    <button
      onClick={onClick}
      style={{
        width: '100%', textAlign: 'left', padding: '8px 16px',
        background: active ? 'rgba(255,255,255,0.08)' : 'none',
        border: 'none', cursor: 'pointer', color: active ? '#fff' : 'rgba(255,255,255,0.7)',
        fontSize: '0.8rem', fontWeight: active ? 600 : 400,
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        transition: 'background 0.1s',
      }}
      onMouseEnter={(e) => { if (!active) e.currentTarget.style.background = 'rgba(255,255,255,0.04)' }}
      onMouseLeave={(e) => { if (!active) e.currentTarget.style.background = 'none' }}
    >
      <span>{label}</span>
      <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        {meta && <span style={{ color: 'rgba(255,255,255,0.35)', fontSize: '0.7rem' }}>{meta}</span>}
        {badge && (
          <span style={{
            background: 'rgba(255,255,255,0.1)', color: 'rgba(255,255,255,0.6)',
            fontSize: '0.6rem', fontWeight: 700, padding: '2px 6px', borderRadius: 4,
            textTransform: 'uppercase', letterSpacing: '0.03em',
          }}>{badge}</span>
        )}
        {active && <span style={{ color: C.accent, fontSize: '0.75rem' }}>✓</span>}
      </span>
    </button>
  )
}
