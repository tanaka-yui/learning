import { Link, Outlet, useNavigate } from 'react-router-dom';
import { useAuth } from '../hooks/useAuth';
import { useCart } from '../hooks/useCart';

export function Layout() {
  const auth = useAuth();
  const cart = useCart();
  const navigate = useNavigate();
  const cartCount = cart.items.reduce((sum, it) => sum + it.quantity, 0);

  const onSignOut = async () => {
    await auth.signOut();
    navigate('/');
  };

  return (
    <div>
      <nav className="nav">
        <Link to="/">Shop</Link>
        <Link to="/cart">
          Cart {cartCount > 0 && <span className="badge">{cartCount}</span>}
        </Link>
        {auth.state.status === 'authenticated' && <Link to="/orders">Orders</Link>}
        <span className="spacer" />
        {auth.state.status === 'authenticated' ? (
          <>
            <span className="muted">{auth.state.user.email}</span>
            <button className="btn btn-secondary" onClick={onSignOut}>Sign out</button>
          </>
        ) : auth.state.status === 'unauthenticated' ? (
          <>
            <Link to="/signin">Sign in</Link>
            <Link to="/signup">Sign up</Link>
          </>
        ) : (
          <span className="muted">…</span>
        )}
      </nav>
      <main className="layout">
        <Outlet />
      </main>
    </div>
  );
}
