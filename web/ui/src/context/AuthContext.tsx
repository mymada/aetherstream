import { createContext, useContext, useState, useCallback, ReactNode } from 'react'

interface User {
  user_id: string
  device_id?: string
  bandwidth_kbps?: number
  role?: string
}

interface AuthCtx {
  token: string | null
  user: User | null
  login: (token: string) => void
  logout: () => void
}

const AuthContext = createContext<AuthCtx | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(localStorage.getItem('token'))
  const [user, setUser] = useState<User | null>(() => {
    const raw = localStorage.getItem('user')
    return raw ? JSON.parse(raw) : null
  })

  const login = useCallback((newToken: string) => {
    localStorage.setItem('token', newToken)
    setToken(newToken)
    try {
      const payload = JSON.parse(atob(newToken.split('.')[1]))
      const u: User = {
        user_id: payload.sub || payload.user_id || '',
        role: payload.role || 'user',
      }
      localStorage.setItem('user', JSON.stringify(u))
      setUser(u)
    } catch {
      setUser(null)
    }
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    setToken(null)
    setUser(null)
  }, [])

  return (
    <AuthContext.Provider value={{ token, user, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be inside AuthProvider')
  return ctx
}
