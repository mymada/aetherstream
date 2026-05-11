import './tv.css'

export interface TVRemoteProps {
  onPlay?: () => void
  onPause?: () => void
  onSeekForward?: () => void
  onSeekBackward?: () => void
  onVolumeUp?: () => void
  onVolumeDown?: () => void
  onStop?: () => void
}

export default function TVRemote({
  onPlay,
  onPause,
  onSeekForward,
  onSeekBackward,
  onVolumeUp,
  onVolumeDown,
  onStop,
}: TVRemoteProps) {
  return (
    <div className="tv-remote">
      <button className="tv-btn primary" onClick={onPlay} aria-label="Play">
        ▶
      </button>
      <button className="tv-btn primary" onClick={onPause} aria-label="Pause">
        ⏸
      </button>
      <button className="tv-btn" onClick={onSeekBackward} aria-label="Seek backward">
        ⏪ -10s
      </button>
      <button className="tv-btn" onClick={onSeekForward} aria-label="Seek forward">
        ⏩ +10s
      </button>
      <button className="tv-btn" onClick={onVolumeDown} aria-label="Volume down">
        🔉 -
      </button>
      <button className="tv-btn" onClick={onVolumeUp} aria-label="Volume up">
        🔊 +
      </button>
      <button className="tv-btn" onClick={onStop} aria-label="Stop">
        ⏹ Stop
      </button>
    </div>
  )
}
