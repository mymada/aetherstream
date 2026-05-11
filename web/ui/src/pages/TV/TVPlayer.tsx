import { useRef, useEffect, useState, useCallback } from 'react'
import Hls from 'hls.js'
import './tv.css'

export interface TVPlayerProps {
  src: string | null
  title?: string
  onStateChange?: (state: 'playing' | 'paused' | 'ended') => void
  onPositionChange?: (position: number, duration: number) => void
  onVolumeChange?: (volume: number) => void
}

export default function TVPlayer({
  src,
  title = '',
  onStateChange,
  onPositionChange,
  onVolumeChange,
}: TVPlayerProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const hlsRef = useRef<Hls | null>(null)
  const [isPlaying, setIsPlaying] = useState(false)
  const [position, setPosition] = useState(0)
  const [duration, setDuration] = useState(0)
  const [volume, setVolume] = useState(1)
  const [showOverlay, setShowOverlay] = useState(false)
  const overlayTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const seekingRef = useRef(false)
  const seekTargetRef = useRef(0)

  const showInfo = useCallback(() => {
    setShowOverlay(true)
    if (overlayTimerRef.current) clearTimeout(overlayTimerRef.current)
    overlayTimerRef.current = setTimeout(() => {
      setShowOverlay(false)
    }, 3000)
  }, [])

  // Setup HLS
  useEffect(() => {
    const video = videoRef.current
    if (!video || !src) return

    if (hlsRef.current) {
      hlsRef.current.destroy()
      hlsRef.current = null
    }

    if (Hls.isSupported()) {
      const hls = new Hls()
      hls.loadSource(src)
      hls.attachMedia(video)
      hlsRef.current = hls
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      video.src = src
    }

    return () => {
      hlsRef.current?.destroy()
      hlsRef.current = null
    }
  }, [src])

  // Video event handlers
  useEffect(() => {
    const video = videoRef.current
    if (!video) return

    const onPlay = () => {
      setIsPlaying(true)
      onStateChange?.('playing')
      showInfo()
    }
    const onPause = () => {
      setIsPlaying(false)
      onStateChange?.('paused')
      showInfo()
    }
    const onEnded = () => {
      setIsPlaying(false)
      onStateChange?.('ended')
      showInfo()
    }
    const onTimeUpdate = () => {
      if (!seekingRef.current) {
        setPosition(video.currentTime)
        onPositionChange?.(video.currentTime, video.duration || 0)
      }
    }
    const handleVolumeChange = () => {
      setVolume(video.volume)
      onVolumeChange?.(video.volume)
      showInfo()
    }
    const onLoadedMetadata = () => {
      setDuration(video.duration || 0)
    }

    video.addEventListener('play', onPlay)
    video.addEventListener('pause', onPause)
    video.addEventListener('ended', onEnded)
    video.addEventListener('timeupdate', onTimeUpdate)
    video.addEventListener('volumechange', handleVolumeChange)
    video.addEventListener('loadedmetadata', onLoadedMetadata)

    return () => {
      video.removeEventListener('play', onPlay)
      video.removeEventListener('pause', onPause)
      video.removeEventListener('ended', onEnded)
      video.removeEventListener('timeupdate', onTimeUpdate)
      video.removeEventListener('volumechange', handleVolumeChange)
      video.removeEventListener('loadedmetadata', onLoadedMetadata)
    }
  }, [onStateChange, onPositionChange, onVolumeChange, showInfo])

  // Keyboard navigation
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const video = videoRef.current
      if (!video) return

      switch (e.key) {
        case 'ArrowLeft': {
          e.preventDefault()
          seekingRef.current = true
          const newPos = Math.max(0, video.currentTime - 10)
          seekTargetRef.current = newPos
          setPosition(newPos)
          showInfo()
          break
        }
        case 'ArrowRight': {
          e.preventDefault()
          seekingRef.current = true
          const newPos = Math.min(video.duration || Infinity, video.currentTime + 10)
          seekTargetRef.current = newPos
          setPosition(newPos)
          showInfo()
          break
        }
        case 'ArrowUp': {
          e.preventDefault()
          video.volume = Math.min(1, video.volume + 0.1)
          showInfo()
          break
        }
        case 'ArrowDown': {
          e.preventDefault()
          video.volume = Math.max(0, video.volume - 0.1)
          showInfo()
          break
        }
        case 'Enter':
        case ' ': {
          e.preventDefault()
          if (video.paused) video.play()
          else video.pause()
          break
        }
        case 'Escape': {
          e.preventDefault()
          video.pause()
          break
        }
        default:
          break
      }
    }

    const onKeyUp = (e: KeyboardEvent) => {
      if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
        const video = videoRef.current
        if (video && seekingRef.current) {
          video.currentTime = seekTargetRef.current
          seekingRef.current = false
        }
      }
    }

    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('keyup', onKeyUp)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('keyup', onKeyUp)
    }
  }, [showInfo])

  const formatTime = (t: number) => {
    if (!isFinite(t) || t < 0) return '0:00'
    const m = Math.floor(t / 60)
    const s = Math.floor(t % 60)
    const h = Math.floor(m / 60)
    if (h > 0) return `${h}:${String(m % 60).padStart(2, '0')}:${String(s).padStart(2, '0')}`
    return `${m}:${String(s).padStart(2, '0')}`
  }

  const progressPct = duration > 0 ? (position / duration) * 100 : 0

  return (
    <div className="tv-player-wrap">
      <video ref={videoRef} autoPlay playsInline />
      <div className={`tv-info-overlay ${showOverlay ? 'active' : ''}`}>
        <div className="tv-info-title">{title || 'AetherStream'}</div>
        <div className="tv-info-row">
          <span>{isPlaying ? '▶ Lecture' : '⏸ Pause'}</span>
          <div className="tv-progress-bar">
            <div
              className={`tv-progress-fill ${seekingRef.current ? 'seeking' : ''}`}
              style={{ width: `${progressPct}%` }}
            />
          </div>
          <span>{formatTime(position)} / {formatTime(duration)}</span>
          <span>🔊 {Math.round(volume * 100)}%</span>
        </div>
      </div>
    </div>
  )
}
