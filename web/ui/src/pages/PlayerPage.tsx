import { useEffect, useRef, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import Hls from 'hls.js'
import { apiClient, type MediaItem } from '../api/client'
import { ArrowLeft } from 'lucide-react'

export default function PlayerPage() {
  const { id } = useParams<{ id: string }>()
  const videoRef = useRef<HTMLVideoElement>(null)
  const [item, setItem] = useState<MediaItem | null>(null)
  const [error, setError] = useState('')
  const [subs, setSubs] = useState<{ language: string; type: string }[]>([])

  useEffect(() => {
    if (!id) return
    apiClient.getItem(id).then(setItem).catch((e) => setError(e.message))
    apiClient.listSubtitles(id).then(setSubs).catch(() => null)
  }, [id])

  useEffect(() => {
    if (!id || !videoRef.current) return
    const video = videoRef.current
    const src = `/videos/${id}/hls/master.m3u8`

    if (Hls.isSupported()) {
      const hls = new Hls()
      hls.loadSource(src)
      hls.attachMedia(video)
      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (data.fatal) {
          setError('Stream error: ' + data.type)
        }
      })
      return () => { hls.destroy() }
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      video.src = src
    } else {
      setError('HLS not supported in this browser')
    }
  }, [id])

  return (
    <div>
      <Link to="/libraries" style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 12 }}>
        <ArrowLeft size={18} /> Back to libraries
      </Link>
      <h1 style={{ color: '#66fcf1', marginTop: 0 }}>{item?.name || 'Player'}</h1>
      {error && <div className="error">{error}</div>}
      <div style={{ background: '#000', borderRadius: 8, overflow: 'hidden' }}>
        <video
          ref={videoRef}
          controls
          style={{ width: '100%', maxHeight: '70vh', display: 'block' }}
          crossOrigin="anonymous"
        />
      </div>
      {item && (
        <div className="card" style={{ marginTop: 12 }}>
          <div style={{ fontSize: 14, color: '#888' }}>
            {item.container} • {item.width}x{item.height} • {item.videoCodec}/{item.audioCodec} • {Math.round(item.durationSeconds / 60)} min
          </div>
        </div>
      )}
      {subs.length > 0 && (
        <div className="card" style={{ marginTop: 12 }}>
          <div style={{ fontWeight: 600, marginBottom: 8 }}>Subtitles</div>
          {subs.map(s => (
            <div key={s.language} style={{ fontSize: 13, color: '#888' }}>
              {s.language} ({s.type})
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
