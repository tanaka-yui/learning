import { FormEvent, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { signIn } from '../api/auth';
import { useAuth } from '../hooks/useAuth';
import { ErrorBanner } from '../components/ErrorBanner';

export default function SignIn() {
  const [email, setEmail] = useState('alice@example.com');
  const [password, setPassword] = useState('password');
  const [error, setError] = useState<unknown>(null);
  const [submitting, setSubmitting] = useState(false);
  const auth = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await signIn(email, password);
      await auth.refresh();
      const next = params.get('next');
      navigate(next ?? '/');
    } catch (err) {
      setError(err);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <h1>Sign in</h1>
      <ErrorBanner error={error} />
      <form onSubmit={onSubmit}>
        <label>Email
          <input className="input" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </label>
        <label>Password
          <input className="input" type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </label>
        <p />
        <button className="btn" type="submit" disabled={submitting}>
          {submitting ? '...' : 'Sign in'}
        </button>
      </form>
    </div>
  );
}
