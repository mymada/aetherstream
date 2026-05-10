import { useState, useEffect, useCallback } from 'react';

const TOKEN_KEY = 'aetherstream_token';

export function useAuth() {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_KEY));

  useEffect(() => {
    const handler = () => setToken(localStorage.getItem(TOKEN_KEY));
    window.addEventListener('storage', handler);
    return () => window.removeEventListener('storage', handler);
  }, []);

  const login = useCallback((newToken: string) => {
    localStorage.setItem(TOKEN_KEY, newToken);
    setToken(newToken);
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
    setToken(null);
  }, []);

  return { token, login, logout, isAuthenticated: !!token };
}
