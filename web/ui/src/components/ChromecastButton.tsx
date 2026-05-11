import React, { useEffect, useState, useCallback } from 'react'

// Google Cast Sender SDK types (loaded dynamically)
declare global {
  interface Window {
    cast?: any
    chrome?: any
  }
}

interface ChromecastButtonProps {
  mediaUrl: string
  title?: string
  thumbnail?: string
  style?: React.CSSProperties
}

export default function ChromecastButton({ mediaUrl, title, thumbnail, style }: ChromecastButtonProps) {
  const [isAvailable, setIsAvailable] = useState(false)
  const [isConnected, setIsConnected] = useState(false)
  const [isCasting, setIsCasting] = useState(false)

  // Load Cast Sender SDK
  useEffect(() => {
    if (window.cast || document.getElementById('cast-sender-script')) return

    const script = document.createElement('script')
    script.id = 'cast-sender-script'
    script.src = 'https://www.gstatic.com/cv/js/sender/v1/cast_sender.js?loadCastFramework=1'
    script.onload = () => {
      // Wait for cast framework to initialize
      const checkAvailable = setInterval(() => {
        if (window.cast?.framework) {
          clearInterval(checkAvailable)
          setIsAvailable(true)
          initializeCast()
        }
      }, 500)
      // Timeout after 10s
      setTimeout(() => clearInterval(checkAvailable), 10000)
    }
    document.head.appendChild(script)

    return () => {
      // Don't remove script on unmount, it's shared
    }
  }, [])

  const initializeCast = useCallback(() => {
    const castContext = window.cast.framework.CastContext.getInstance()
    castContext.setOptions({
      receiverApplicationId: window.chrome?.cast?.media?.DEFAULT_MEDIA_RECEIVER_APP_ID || 'CC1AD845',
      autoJoinPolicy: window.chrome?.cast?.AutoJoinPolicy?.ORIGIN_SCOPED || 'origin_scoped',
    })

    // Listen for session state changes
    castContext.addEventListener(
      window.cast.framework.CastContextEventType.SESSION_STATE_CHANGED,
      (event: any) => {
        switch (event.sessionState) {
          case window.cast.framework.SessionState.SESSION_STARTED:
          case window.cast.framework.SessionState.SESSION_RESUMED:
            setIsConnected(true)
            break
          case window.cast.framework.SessionState.SESSION_ENDED:
            setIsConnected(false)
            setIsCasting(false)
            break
        }
      }
    )
  }, [])

  const handleCast = useCallback(() => {
    if (!window.cast?.framework) return

    const castContext = window.cast.framework.CastContext.getInstance()
    const session = castContext.getCurrentSession()

    if (session) {
      // Already connected — load media
      loadMedia(session)
    } else {
      // Request session
      castContext.requestSession().then((s: any) => {
        if (s) {
          loadMedia(s)
        }
      }).catch(() => {})
    }
  }, [mediaUrl, title, thumbnail])

  const loadMedia = useCallback((session: any) => {
    if (!window.chrome?.cast?.media) return

    const mediaInfo = new window.chrome.cast.media.MediaInfo(mediaUrl, 'video/mp4')
    mediaInfo.metadata = new window.chrome.cast.media.GenericMediaMetadata()
    mediaInfo.metadata.title = title || 'AetherStream'
    if (thumbnail) {
      mediaInfo.metadata.images = [new window.chrome.cast.Image(thumbnail)]
    }

    const request = new window.chrome.cast.media.LoadRequest(mediaInfo)
    request.autoplay = true

    session.loadMedia(request).then(() => {
      setIsCasting(true)
    }).catch(() => {
      setIsCasting(false)
    })
  }, [mediaUrl, title, thumbnail])

  const handleStopCast = useCallback(() => {
    if (!window.cast?.framework) return
    const castContext = window.cast.framework.CastContext.getInstance()
    const session = castContext.getCurrentSession()
    if (session) {
      session.endSession(true)
    }
    setIsCasting(false)
    setIsConnected(false)
  }, [])

  if (!isAvailable) {
    return null // Don't show button if Cast SDK not available
  }

  const btnStyle: React.CSSProperties = {
    padding: '8px 14px',
    borderRadius: '8px',
    background: isConnected ? '#4CAF50' : 'rgba(255,255,255,0.15)',
    color: '#fff',
    border: 'none',
    fontSize: '0.9rem',
    cursor: 'pointer',
    display: 'flex',
    alignItems: 'center',
    gap: '6px',
    minWidth: '36px',
    justifyContent: 'center',
    ...style,
  }

  return (
    <button
      style={btnStyle}
      onClick={isCasting ? handleStopCast : handleCast}
      title={isCasting ? 'Stop casting' : isConnected ? 'Cast to TV' : 'Connect to Chromecast'}
    >
      {isCasting ? '⏹' : isConnected ? '📺' : '📡'}
      <span>{isCasting ? 'Stop' : 'Cast'}</span>
    </button>
  )
}
