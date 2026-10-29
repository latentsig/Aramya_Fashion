import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

export type Customer = { id: string; email: string; displayName: string };
type AuthValue = { user: Customer | null; loading: boolean; login: (email: string, password: string) => Promise<void>; register: (email: string, password: string, displayName: string) => Promise<void>; logout: () => Promise<void> };
const AuthContext = createContext<AuthValue | null>(null);
const apiBase = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

async function secureToken(): Promise<string> {
  const response = await fetch(`${apiBase}/api/v1/auth/csrf`, { credentials: 'include', headers: { Accept: 'application/json' } });
  const payload = await response.json().catch(() => null) as { csrfToken?: string } | null;
  if (!response.ok || !payload?.csrfToken) throw new Error('Could not prepare a secure sign-in request. Please retry.');
  return payload.csrfToken;
}

async function post<T>(path: string, body?: unknown): Promise<T | null> {
  const token = await secureToken();
  const response = await fetch(`${apiBase}/api/v1/auth${path}`, {
    method: 'POST', credentials: 'include',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json', 'X-CSRF-Token': token },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });
  const payload = await response.json().catch(() => null) as { user?: Customer; error?: { message?: string } } | null;
  if (!response.ok) throw new Error(payload?.error?.message ?? 'Your request could not be completed. Please try again.');
  return payload as T | null;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<Customer | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const response = await fetch(`${apiBase}/api/v1/auth/me`, { credentials: 'include', headers: { Accept: 'application/json' } });
      if (!response.ok) { setUser(null); return; }
      const payload = await response.json() as { user: Customer };
      setUser(payload.user);
    } catch { setUser(null); }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);

  const value = useMemo<AuthValue>(() => ({
    user,
    loading,
    login: async (email, password) => {
      const result = await post<{ user: Customer }>('/login', { email, password });
      if (!result?.user) throw new Error('The server did not return a customer session.');
      setUser(result.user);
      window.dispatchEvent(new Event('aramya:auth-change'));
    },
    register: async (email, password, displayName) => {
      const result = await post<{ user: Customer }>('/register', { email, password, displayName });
      if (!result?.user) throw new Error('The server did not return a customer session.');
      setUser(result.user);
      window.dispatchEvent(new Event('aramya:auth-change'));
    },
    logout: async () => {
      await post<null>('/logout');
      setUser(null);
      window.dispatchEvent(new Event('aramya:auth-change'));
    },
  }), [user, loading]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth must be used inside AuthProvider');
  return value;
}
