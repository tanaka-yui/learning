import { createContext, useContext, useEffect, useState, useCallback, ReactNode } from 'react';
import { me, signOut as apiSignOut } from '../api/auth';
import { ApiError } from '../api/http';

interface User {
  id: string;
  email: string;
}

export type AuthState =
  | { status: 'loading' }
  | { status: 'authenticated'; user: User }
  | { status: 'unauthenticated' };

interface AuthContextValue {
  state: AuthState;
  refresh: () => Promise<void>;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'loading' });

  const refresh = useCallback(async () => {
    try {
      const data = await me();
      setState({ status: 'authenticated', user: { id: data.user_id, email: data.email } });
    } catch (e) {
      if (e instanceof ApiError && e.code === 'UNAUTHORIZED') {
        setState({ status: 'unauthenticated' });
        return;
      }
      console.error('auth probe failed', e);
      setState({ status: 'unauthenticated' });
    }
  }, []);

  const signOut = useCallback(async () => {
    try {
      await apiSignOut();
    } catch (e) {
      console.error('signOut failed', e);
    }
    setState({ status: 'unauthenticated' });
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <AuthContext.Provider value={{ state, refresh, signOut }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider');
  return ctx;
}
