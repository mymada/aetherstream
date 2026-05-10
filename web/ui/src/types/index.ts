export interface SystemInfo {
  version: string;
  uptime: string;
  libraries_count: number;
  items_count: number;
}

export interface Library {
  id: string;
  name: string;
  type: string;
  path: string;
  item_count: number;
}

export interface MediaItem {
  id: string;
  title: string;
  type: string;
  library_id: string;
  year?: number;
  duration?: number;
  thumbnail?: string;
}

export interface LoginCredentials {
  username: string;
  password: string;
}

export interface AuthResponse {
  token: string;
  user?: {
    id: string;
    username: string;
  };
}
