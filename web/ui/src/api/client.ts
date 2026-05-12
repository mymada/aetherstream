const API_BASE = '' // served from same origin

function authHeaders(): Record<string, string> {
  const token = localStorage.getItem('token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

async function api<T>(path: string, opts?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    ...opts,
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(body || `${res.status} ${res.statusText}`)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export interface Library {
  id: string
  name: string
  path: string
  mediaType: string
  createdAt: string
}

export interface MediaItem {
  id: string
  libraryId: string
  name: string
  path: string
  mediaType: string
  container: string
  sizeBytes: number
  durationSeconds: number
  width: number
  height: number
  videoCodec: string
  audioCodec: string
  createdAt: string
}

export interface Collection {
  id: string
  name: string
  type: string
  createdAt: string
}

export interface Activity {
  id: string
  userId: string
  action: string
  details: string
  createdAt: string
}

export interface User {
  id: string
  username: string
  role: string
  createdAt: string
}

export interface Chapter {
  name: string
  position: number
}

export interface TrickplayVTT {
  vtt: string
  interval: number
}

export interface ContinueWatchingItem {
  id: string
  name: string
  posterUrl: string
  position: number
  duration: number
}

export interface HealthStatus {
  status: string
  version: string
  uptime: string
  dbConnected: boolean
  ffmpegAvailable: boolean
}

export const apiClient = {
  login: (username: string, password: string) =>
    api<{ token: string }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  systemInfo: () =>
    api<{ name: string; version: string; status: string }>('/system/info'),

  health: () => api<HealthStatus>('/health'),
  ready: () => api<{ status: string }>('/ready'),

  listLibraries: () => api<Library[]>('/api/libraries'),
  createLibrary: (name: string, path: string, mediaType: string) =>
    api<Library>('/api/libraries', {
      method: 'POST',
      body: JSON.stringify({ name, path, media_type: mediaType }),
    }),
  scanLibrary: (id: string) =>
    api<{ status: string }>(`/api/libraries/${id}/scan`, { method: 'POST' }),

  listItems: (libraryId: string) =>
    api<MediaItem[]>(`/api/items?library_id=${encodeURIComponent(libraryId)}`),
  getItem: (id: string) => api<MediaItem>(`/api/items/${id}`),
  listSubtitles: (id: string) =>
    api<{ language: string; type: string }[]>(`/api/items/${id}/subtitles`),

  listChapters: (id: string) => api<Chapter[]>(`/api/items/${id}/chapters`),
  getChapterAt: (id: string, position: number) =>
    api<Chapter>(`/api/items/${id}/chapters/at?position=${position}`),
  getTrickplay: (id: string) => api<TrickplayVTT>(`/api/items/${id}/trickplay`),
  getContinueWatching: () => api<ContinueWatchingItem[]>('/api/continue-watching'),
  saveProgress: (id: string, position: number, duration: number) =>
    api<void>(`/api/items/${id}/progress`, {
      method: 'POST',
      body: JSON.stringify({ position, duration }),
    }),

  listCollections: () => api<Collection[]>('/api/collections'),
  createCollection: (name: string, type?: string) =>
    api<Collection>('/api/collections', {
      method: 'POST',
      body: JSON.stringify({ name, type: type || 'collection' }),
    }),
  getCollection: (id: string) =>
    api<{ collection: Collection; items: MediaItem[] }>(`/api/collections/${id}`),
  addToCollection: (colId: string, itemId: string) =>
    api<void>(`/api/collections/${colId}/items`, {
      method: 'POST',
      body: JSON.stringify({ item_id: itemId }),
    }),

  listActivity: () => api<Activity[]>('/api/activity'),

  listUsers: () => api<User[]>('/api/users'),
  createUser: (username: string, password: string, role?: string) =>
    api<User>('/api/users', {
      method: 'POST',
      body: JSON.stringify({ username, password, role: role || 'user' }),
    }),
  updateUser: (id: string, role: string) =>
    api<User>(`/api/users/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    }),
  deleteUser: (id: string) =>
    api<void>(`/api/users/${id}`, { method: 'DELETE' }),

  session: () => api<User>('/api/session'),
}
