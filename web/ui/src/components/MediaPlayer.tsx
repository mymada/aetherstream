import React, { useEffect, useRef, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
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
  },
  title: { fontWeight: 600, fontSize: '1rem' },
  videoWrap: {
    flex: 1,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: '16px',
  },
  video: {
    width: '100%',
    maxWidth: '1200px',
    maxHeight: '70vh',
    borderRadius: '8px',
    background: '#000',
  },
}

export default function MediaPlayer() {
  const { id } = useParams()
  const navigate = useNavigate()
  const videoRef = useRef<HTMLVideoElement>(null)
  const [item, setItem] = useState<any>(null)

  useEffect(() => {
    if (!id) return
    getItem(id).then(setItem).catch(() => {})
  }, [id])

  return (
    <div style={styles.container}>
      <div style={styles.topBar}>
        <button style={styles.backBtn} onClick={() => navigate(-1)}>Back</button>
        <div style={styles.title}>{item?.title || 'Loading...'}</div>
      </div>
      <div style={styles.videoWrap}>
        {id && (
          <video
            ref={videoRef}
            style={styles.video}
            controls
            autoPlay
            src={getStreamUrl(id)}
          />
        )}
      </div>
    </div>
  )
}
