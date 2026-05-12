import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { listProducts, Product } from '../api/products';
import { ErrorBanner } from '../components/ErrorBanner';
import { useCart } from '../hooks/useCart';
import { formatPrice } from '../lib/format';

export default function Cart() {
  const cart = useCart();
  const [products, setProducts] = useState<Product[] | null>(null);
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    listProducts().then(setProducts).catch(setError);
  }, []);

  if (error) return <ErrorBanner error={error} />;
  if (cart.items.length === 0) {
    return (
      <div>
        <h1>カート</h1>
        <p className="muted">カートは空です。</p>
        <Link to="/" className="btn btn-secondary">商品一覧へ</Link>
      </div>
    );
  }
  if (!products) return <p>Loading...</p>;

  const lookup = new Map(products.map((p) => [p.id, p]));
  const rows = cart.items.map((it) => {
    const p = lookup.get(it.productId);
    const subtotal = p ? p.price_cents * it.quantity : 0;
    return { it, p, subtotal };
  });
  const total = rows.reduce((s, r) => s + r.subtotal, 0);

  return (
    <div>
      <h1>カート</h1>
      <table>
        <thead>
          <tr><th>商品</th><th>単価</th><th>数量</th><th>小計</th><th></th></tr>
        </thead>
        <tbody>
          {rows.map(({ it, p, subtotal }) => (
            <tr key={it.productId}>
              <td>{p?.name ?? it.productId}</td>
              <td>{p ? formatPrice(p.price_cents) : '-'}</td>
              <td>
                <input
                  type="number" min={0} value={it.quantity}
                  onChange={(e) => cart.setQuantity(it.productId, parseInt(e.target.value, 10) || 0)}
                  style={{ width: 64 }}
                />
              </td>
              <td>{formatPrice(subtotal)}</td>
              <td><button className="btn btn-secondary" onClick={() => cart.remove(it.productId)}>Remove</button></td>
            </tr>
          ))}
        </tbody>
        <tfoot>
          <tr><th colSpan={3}>合計</th><th colSpan={2}>{formatPrice(total)}</th></tr>
        </tfoot>
      </table>
      <p>
        <Link to="/checkout" className="btn">注文確定へ</Link>
      </p>
    </div>
  );
}
