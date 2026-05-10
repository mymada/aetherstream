const API_BASE = window.location.origin;

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

export async function getSystemInfo() {
  return api<{ version: string; uptime: string; libraries_count: number; items_count: number }>('/system/info');
}

export async function getLibraries() {
  return api<{ id: string; name: string; type: string; path: string; item_count: number }[]>('/libraries');
}

export async function getItems(libraryId?: string) {
  const qs = libraryId ? `?library_id=${encodeURIComponent(libraryId)}` : '';
  return api<{ id: string; title: string; type: string; library_id: string; year?: number; duration?: number; thumbnail?: string }[]>(`/items${qs}`);
}

export async function getItem(id: string) {
  return api<{ id: string; title: string; type: string; library_id: string; year?: number; duration?: number; thumbnail?: string }>(`/items/${id}`);
}

export function getStreamUrl(id: string): string {
  return `${API_BASE}/stream/${id}`;
}

export function getThumbnailUrl(id: string): string {
  return `${API_BASE}/thumbnails/${id}`;
}
