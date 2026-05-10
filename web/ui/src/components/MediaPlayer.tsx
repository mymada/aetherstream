import React, { useEffect, useRef, useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import Hls from 'hls.js'
import { getItem, getStreamUrl } from '../api.ts'

const styles: Record<string, React.CSSProperties> = {
  container: {
    minHeight: '100vh',
    display: 'flex',
    flexDirection: 'column',
    background: '#000',
  },
  topBar: {
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
    padding: '12px 16px',
    background: 'var(--bg-secondary)',
    borderBottom: '1px solid var(--border)',
  },
  backBtn: {
    padding: '6px 12px',
    borderRadius: '6px',
    background: 'var(--bg-tertiary)',
    color: 'var(--text-primary)',
    border: '1px solid var(--border)',
    fontSize: '0.9rem',
    cursor: 'pointer',
  },
  title: { fontWeight: 600, fontSize: '1rem' },
  videoWrap: {
    flex: 1,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: '16px',
    position: 'relative',
  },
  video: {
    width: '100%',
    maxWidth: '1200px',
    maxHeight: '70vh',
    borderRadius: '8px',
    background: '#000',
  },
  controlsOverlay: {
    position: 'absolute',
    bottom: '24px',
    left: '50%',
    transform: 'translateX(-50%)',
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
    padding: '10px 16px',
    borderRadius: '8px',
    background: 'rgba(0,0,0,0.7)',
    backdropFilter: 'blur(4px)',
    zIndex: 10,
  },
  ctrlBtn: {
    padding: '6px 10px',
    borderRadius: '6px',
    background: 'rgba(255,255,255,0.15)',
    color: '#fff',
    border: 'none',
    fontSize: '0.85rem',
    cursor: 'pointer',
    minWidth: '36px',
    textAlign: 'center',
  },
  volumeSlider: {
    width: '80px',
    accentColor: '#fff',
  },
  qualitySelect: {
    padding: '6px 8px',
    borderRadius: '6px',
    background: 'rgba(255,255,255,0.15)',
    color: '#fff',
    border: 'none',
    fontSize: '0.85rem',
    cursor: 'pointer',
  },
  errorMsg: {
    color: '#ff6b6b',
    padding: '16px',
    textAlign: 'center',
  },
}

export default function MediaPlayer() {
  const { id } = useParams()
  const navigate = useNavigate()
  const videoRef = useRef<HTMLVideoElement>(null)
  const hlsRef = useRef<Hls | null>(null)
  const [item, setItem] = useState<any>(null)
  const [isPlaying, setIsPlaying] = useState(false)
  const [volume, setVolume] = useState(1)
  const [levels, setLevels] = useState<{ index: number; label: string }[]>([])
  const [currentLevel, setCurrentLevel] = useState(-1)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    getItem(id).then(setItem).catch(() => {})
  }, [id])

  const destroyHls = useCallback(() => {
    if (hlsRef.current) {
      hlsRef.current.destroy()
      hlsRef.current = null
    }
  }, [])

  useEffect(() => {
    const video = videoRef.current
    if (!video || !id) return

    const src = getStreamUrl(id)
    const canPlayNative = video.canPlayType('application/vnd.apple.mpegurl') !== ''

    if (canPlayNative) {
      video.src = src
      setIsPlaying(true)
    } else if (Hls.isSupported()) {
      const hls = new Hls({
        enableWorker: true,
        lowLatencyMode: false,
        backBufferLength: 90,
      })
      hlsRef.current = hls

      hls.on(Hls.Events.MEDIA_ATTACHED, () => {
        hls.loadSource(src)
      })

      hls.on(Hls.Events.MANIFEST_PARSED, (_event, data) => {
        const lvlList = data.levels.map((lvl, idx) => {
          const height = lvl.height || 0
          const bw = Math.round((lvl.bitrate || 0) / 1000)
          const label = height ? `${height}p (${bw}k)` : `Level ${idx + 1}`
          return { index: idx, label }
        })
        setLevels(lvlList)
        setCurrentLevel(-1)
        video.play().catch(() => {})
        setIsPlaying(true)
      })

      hls.on(Hls.Events.LEVEL_SWITCHED, (_event, data) => {
        setCurrentLevel(data.level)
      })

      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (data.fatal) {
          switch (data.type) {
            case Hls.ErrorTypes.NETWORK_ERROR:
              setError('Network error while loading stream.')
              hls.startLoad()
              break
            case Hls.ErrorTypes.MEDIA_ERROR:
              setError('Media error. Attempting recovery...')
              hls.recoverMediaError()
              break
            default:
              setError('Fatal playback error.')
              destroyHls()
              break
          }
        }
      })

      hls.attachMedia(video)
    } else {
      setError('HLS is not supported in this browser.')
    }

    return () => {
      destroyHls()
      if (video) {
        video.pause()
        video.removeAttribute('src')
        video.load()
      }
    }
  }, [id, destroyHls])

  const togglePlay = useCallback(() => {
    const video = videoRef.current
    if (!video) return
    if (video.paused) {
      video.play().then(() => setIsPlaying(true)).catch(() => {})
    } else {
      video.pause()
      setIsPlaying(false)
    }
  }, [])

  const handleVolumeChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const v = parseFloat(e.target.value)
    setVolume(v)
    if (videoRef.current) {
      videoRef.current.volume = v
    }
  }, [])

  const toggleFullscreen = useCallback(() => {
    const video = videoRef.current
    if (!video) return
    if (!document.fullscreenElement) {
      video.requestFullscreen?.().catch(() => {})
    } else {
      document.exitFullscreen?.().catch(() => {})
    }
  }, [])

  const handleQualityChange = useCallback((e: React.ChangeEvent<HTMLSelectElement>) => {
    const level = parseInt(e.target.value, 10)
    setCurrentLevel(level)
    if (hlsRef.current) {
      hlsRef.current.currentLevel = level
    }
  }, [])

  return (
    <div style={styles.container}>
      <div style={styles.topBar}>
        <button style={styles.backBtn} onClick={() => navigate(-1)}>Back</button>
        <div style={styles.title}>{item?.title || 'Loading...'}</div>
      </div>
      <div style={styles.videoWrap}>
        {error && <div style={styles.errorMsg}>{error}</div>}
        {id && (
          <video
            ref={videoRef}
            style={styles.video}
            autoPlay
            playsInline
            onClick={togglePlay}
          />
        )}
        {id && !error && (
          <div style={styles.controlsOverlay}>
            <button style={styles.ctrlBtn} onClick={togglePlay}>
              {isPlaying ? 'Pause' : 'Play'}
            </button>
            <input
              type="range"
              min={0}
              max={1}
              step={0.05}
              value={volume}
              onChange={handleVolumeChange}
              style={styles.volumeSlider}
              title={`Volume ${Math.round(volume * 100)}%`}
            />
            {levels.length > 0 && (
              <select
                style={styles.qualitySelect}
                value={currentLevel}
                onChange={handleQualityChange}
                title="Quality"
              >
                <option value={-1}>Auto</option>
                {levels.map((l) => (
                  <option key={l.index} value={l.index}>
                    {l.label}
                  </option>
                ))}
              </select>
            )}
            <button style={styles.ctrlBtn} onClick={toggleFullscreen} title="Fullscreen">
              FS
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
