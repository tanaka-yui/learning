import { FormEvent, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { signUp, signIn } from '../api/auth';
import { useAuth } from '../hooks/useAuth';
import { ErrorBanner } from '../components/ErrorBanner';

export default function SignUp() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [submitting, setSubmitting] = useState(false);
  const auth = useAuth();
  const navigate = useNavigate();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await signUp(email, password);
      await signIn(email, password);
      await auth.refresh();
      navigate('/');
    } catch (err) {
      setError(err);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <h1>Sign up</h1>
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
          {submitting ? '...' : 'Sign up'}
        </button>
      </form>
    </div>
  );
}
