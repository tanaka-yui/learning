import { apiFetch } from './http';

export interface MeResponse {
  user_id: string;
  email: string;
}

export async function me() {
  const { data } = await apiFetch<MeResponse>('/api/auth/me');
  return data;
}

export async function signIn(email: string, password: string) {
  await apiFetch<undefined>('/api/auth/signin', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ Email: email, Password: password }),
  });
}

export async function signUp(email: string, password: string) {
  const { data } = await apiFetch<{ user_id: string }>('/api/auth/signup', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ Email: email, Password: password }),
  });
  return data;
}

export async function signOut() {
  await apiFetch<undefined>('/api/auth/signout', { method: 'POST' });
}
