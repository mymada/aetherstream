import { useEffect, useRef, useState, useCallback } from 'react'
import TVPlayer from './TVPlayer.tsx'
import TVRemote from './TVRemote.tsx'
import './tv.css'

type ConnStatus = 'waiting' | 'paired' | 'playing'

interface WSCommand {
  action: 'play' | 'pause' | 'seek' | 'volume' | 'stop' | 'load'
  url?: string
  title?: string
  position?: number
  volume?: number
}

function getWsUrl() {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}/ws/playback?type=receiver`
}

function generatePairingCode() {
  return Math.random().toString(36).substring(2, 8).toUpperCase()
}

export default function TVPage() {
  const [status, setStatus] = useState<ConnStatus>('waiting')
  const [pairingCode, setPairingCode] = useState('')
  const [src, setSrc] = useState<string | null>(null)
  const [title, setTitle] = useState('')
  const [showDebug, setShowDebug] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const videoRef = useRef<HTMLVideoElement | null>(null)

  // Expose video element for remote controls
  useEffect(() => {
    const video = document.querySelector<HTMLVideoElement>('.tv-player-wrap video')
    if (video) videoRef.current = video
  }, [src, status])

  const sendEvent = useCallback((event: Record<string, unknown>) => {
    const ws = wsRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(event))
    }
  }, [])

  const connect = useCallback(() => {
    if (wsRef.current) {
      try { wsRef.current.close() } catch { /* ignore */ }
    }

    const ws = new WebSocket(getWsUrl())
    wsRef.current = ws

    ws.onopen = () => {
      setStatus((prev) => (prev === 'waiting' ? 'waiting' : prev))
      const code = generatePairingCode()
      setPairingCode(code)
      sendEvent({ type: 'register', role: 'receiver', pairingCode: code })
    }

    ws.onmessage = (ev) => {
      let msg: WSCommand
      try {
        msg = JSON.parse(ev.data)
      } catch {
        return
      }

      const video = videoRef.current
      switch (msg.action) {
        case 'load': {
          if (msg.url) {
            setSrc(msg.url)
            setTitle(msg.title || '')
            setStatus('playing')
            if (msg.position != null && video) {
              video.currentTime = msg.position
            }
          }
          break
        }
        case 'play': {
          video?.play()
          setStatus('playing')
          break
        }
        case 'pause': {
          video?.pause()
          setStatus('paired')
          break
        }
        case 'seek': {
          if (video && msg.position != null) {
            video.currentTime = msg.position
          }
          break
        }
        case 'volume': {
          if (video && msg.volume != null) {
            video.volume = Math.max(0, Math.min(1, msg.volume))
          }
          break
        }
        case 'stop': {
          video?.pause()
          setSrc(null)
          setStatus('paired')
          break
        }
        default:
          break
      }
    }

    ws.onclose = () => {
      setStatus('waiting')
      reconnectTimerRef.current = setTimeout(connect, 3000)
    }

    ws.onerror = () => {
      ws.close()
    }
  }, [sendEvent])

  useEffect(() => {
    connect()
    return () => {
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
      wsRef.current?.close()
    }
  }, [connect])

  const pairingUrl = `${window.location.origin}/app/remote?pair=${pairingCode}`

  const handleStateChange = useCallback((state: 'playing' | 'paused' | 'ended') => {
    if (state === 'playing') setStatus('playing')
    else if (state === 'paused') setStatus('paired')
    else if (state === 'ended') setStatus('paired')
    sendEvent({ type: 'state', state })
  }, [sendEvent])

  const handlePositionChange = useCallback((position: number, duration: number) => {
    sendEvent({ type: 'position', position, duration })
  }, [sendEvent])

  const handleVolumeChange = useCallback((volume: number) => {
    sendEvent({ type: 'volume', volume })
  }, [sendEvent])

  const remoteHandlers = {
    onPlay: () => videoRef.current?.play(),
    onPause: () => videoRef.current?.pause(),
    onSeekForward: () => {
      const v = videoRef.current
      if (v) v.currentTime = Math.min(v.duration || Infinity, v.currentTime + 10)
    },
    onSeekBackward: () => {
      const v = videoRef.current
      if (v) v.currentTime = Math.max(0, v.currentTime - 10)
    },
    onVolumeUp: () => {
      const v = videoRef.current
      if (v) v.volume = Math.min(1, v.volume + 0.1)
    },
    onVolumeDown: () => {
      const v = videoRef.current
      if (v) v.volume = Math.max(0, v.volume - 0.1)
    },
    onStop: () => {
      videoRef.current?.pause()
      setSrc(null)
      setStatus('paired')
    },
  }

  return (
    <div className="tv-page">
      {(!src || status === 'waiting' || status === 'paired') && (
        <div className={`tv-overlay ${!src || status !== 'playing' ? 'visible' : 'hidden'}`}>
          <div className="tv-status waiting">
            {status === 'waiting' && 'En attente de connexion...'}
            {status === 'paired' && 'Appareil appairé — prêt à lire'}
            {status === 'playing' && !src && 'Chargement...'}
          </div>

          {/* QR Code placeholder (SVG) */}
          <div className="tv-qr-placeholder" aria-label="QR code pairing">
            <svg viewBox="0 0 200 200" width="200" height="200">
              <rect width="200" height="200" fill="#fff" />
              <rect x="10" y="10" width="60" height="60" fill="#000" />
              <rect x="20" y="20" width="40" height="40" fill="#fff" />
              <rect x="30" y="30" width="20" height="20" fill="#000" />
              <rect x="130" y="10" width="60" height="60" fill="#000" />
              <rect x="140" y="20" width="40" height="40" fill="#fff" />
              <rect x="150" y="30" width="20" height="20" fill="#000" />
              <rect x="10" y="130" width="60" height="60" fill="#000" />
              <rect x="20" y="140" width="40" height="40" fill="#fff" />
              <rect x="30" y="150" width="20" height="20" fill="#000" />
              <rect x="90" y="10" width="20" height="20" fill="#000" />
              <rect x="90" y="50" width="20" height="20" fill="#000" />
              <rect x="90" y="90" width="20" height="20" fill="#000" />
              <rect x="10" y="90" width="20" height="20" fill="#000" />
              <rect x="50" y="90" width="20" height="20" fill="#000" />
              <rect x="130" y="90" width="20" height="20" fill="#000" />
              <rect x="170" y="90" width="20" height="20" fill="#000" />
              <rect x="90" y="130" width="20" height="20" fill="#000" />
              <rect x="130" y="130" width="20" height="20" fill="#000" />
              <rect x="170" y="130" width="20" height="20" fill="#000" />
              <rect x="90" y="170" width="20" height="20" fill="#000" />
              <rect x="130" y="170" width="20" height="20" fill="#000" />
              <rect x="170" y="170" width="20" height="20" fill="#000" />
            </svg>
          </div>

          <div style={{ fontSize: '20px', opacity: 0.8 }}>
            Scannez ce QR code avec votre téléphone
          </div>
          <div style={{ fontSize: '18px', opacity: 0.6, wordBreak: 'break-all', maxWidth: '80vw' }}>
            {pairingUrl}
          </div>
          <div style={{ fontSize: '32px', fontWeight: 700, letterSpacing: '4px' }}>
            Code: {pairingCode}
          </div>
        </div>
      )}

      {src && (
        <TVPlayer
          src={src}
          title={title}
          onStateChange={handleStateChange}
          onPositionChange={handlePositionChange}
          onVolumeChange={handleVolumeChange}
        />
      )}

      {/* Debug remote panel */}
      <div className="tv-debug-panel">
        <button
          className="tv-btn tv-debug-toggle"
          onClick={() => setShowDebug((s) => !s)}
          aria-label="Toggle debug remote"
        >
          {showDebug ? '✕' : '⚙'}
        </button>
        {showDebug && <TVRemote {...remoteHandlers} />}
      </div>
    </div>
  )
}
