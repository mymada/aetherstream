const API_BASE = '';

function getToken(): string | null {
  return localStorage.getItem('aetherstream_token');
}

async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const url = `${API_BASE}${path}`;
  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((options.headers as Record<string, string>) || {}),
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(url, { ...options, headers });
  if (res.status === 401) {
    localStorage.removeItem('aetherstream_token');
    window.location.href = '/login';
    throw new Error('Session expired');
  }
  if (!res.ok) {
    const err = await res.text();
    throw new Error(err || `HTTP ${res.status}`);
  }
  if (res.status === 204) {
    return {} as T;
  }
  return res.json() as Promise<T>;
}

export async function login(credentials: { username: string; password: string }) {
  return api<{ token: string }>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(credentials),
  });
}

export async function register(credentials: { username: string; password: string }) {
  return api<{ id: string; username: string; role: string }>('/auth/register', {
    method: 'POST',
    body: JSON.stringify(credentials),
  });
}

export async function getSystemInfo() {
  return api<{ version: string; uptime: string; libraries_count: number; items_count: number }>('/system/info');
}

export async function getLibraries() {
  return api<{ id: string; name: string; type: string; path: string; item_count: number }[]>('/api/libraries');
}

export async function getItems(libraryId?: string) {
  const qs = libraryId ? `?library_id=${encodeURIComponent(libraryId)}` : '';
  return api<{ id: string; title: string; type: string; library_id: string; year?: number; duration?: number; thumbnail?: string }[]>(`/api/items${qs}`);
}

export async function getItem(id: string) {
  return api<{ id: string; title: string; type: string; library_id: string; year?: number; duration?: number; thumbnail?: string }>(`/api/items/${id}`);
}

export function getStreamUrl(id: string): string {
  const token = getToken();
  const q = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${window.location.origin}/videos/${id}/stream${q}`;
}

export function getHlsUrl(id: string): string {
  const token = getToken();
  const q = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${window.location.origin}/videos/${id}/hls/master.m3u8${q}`;
}

export function getThumbnailUrl(id: string): string {
  return `${window.location.origin}/api/items/${id}/thumbnails/poster`;
}

export type SubtitleTrack = {
  sub_index: number;
  stream_index: number;
  language: string;
  title: string;
  forced: boolean;
  default: boolean;
  codec: string;
}

export type AudioTrack = {
  sub_index: number;
  language: string;
  title: string;
  codec: string;
  channels: number;
  default: boolean;
}

export async function getSubtitleTracks(id: string) {
  return api<SubtitleTrack[]>(`/videos/${id}/subtitles`);
}

export function getSubtitleVttUrl(id: string, subIndex: number): string {
  const token = getToken();
  const q = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${window.location.origin}/videos/${id}/subtitles/${subIndex}/vtt${q}`;
}

export async function getAudioTracks(id: string) {
  return api<AudioTrack[]>(`/videos/${id}/audio`);
}

export function getHlsUrlWithAudio(id: string, audioIndex: number): string {
  const token = getToken();
  const params = new URLSearchParams();
  if (token) params.set('token', token);
  if (audioIndex > 0) params.set('audio', String(audioIndex));
  const q = params.toString() ? `?${params.toString()}` : '';
  return `${window.location.origin}/videos/${id}/hls/master.m3u8${q}`;
}

// ── Task management ──────────────────────────────────────────────────────────

export type JobStatus = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled'

export type Job = {
  id: string
  item_id: string
  item_title: string
  key: string
  audio_index: number
  profiles: string[]
  status: JobStatus
  created_at: string
  started_at?: string
  completed_at?: string
  error?: string
  output_dir: string
  disk_bytes?: number
}

export type TranscodeDir = {
  key: string
  item_id: string
  audio_index: number
  disk_bytes: number
  active: boolean
  mod_time: string
}

export async function getJobs(): Promise<Job[]> {
  const r = await api<Job[] | null>('/api/jobs');
  return r ?? [];
}

export async function cancelJob(id: string) {
  return api<void>(`/api/jobs/${id}`, { method: 'DELETE' });
}

export async function getTranscodeDirs(): Promise<TranscodeDir[]> {
  const r = await api<TranscodeDir[] | null>('/api/transcodes');
  return r ?? [];
}

export async function deleteTranscodeDir(key: string) {
  return api<void>(`/api/transcodes/${encodeURIComponent(key)}`, { method: 'DELETE' });
}
