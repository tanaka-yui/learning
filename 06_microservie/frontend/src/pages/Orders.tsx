import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { listOrders, Order } from '../api/orders';
import { ErrorBanner } from '../components/ErrorBanner';
import { formatPrice } from '../lib/format';

export default function Orders() {
  const [orders, setOrders] = useState<Order[] | null>(null);
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    listOrders().then(setOrders).catch(setError);
  }, []);

  if (error) return <ErrorBanner error={error} />;
  if (!orders) return <p>Loading...</p>;
  if (orders.length === 0) return <p className="muted">注文履歴はありません。</p>;

  return (
    <div>
      <h1>注文履歴</h1>
      <table>
        <thead>
          <tr><th>Order ID</th><th>Status</th><th>Total</th><th></th></tr>
        </thead>
        <tbody>
          {orders.map((o) => (
            <tr key={o.id}>
              <td><code>{o.id}</code></td>
              <td>{o.status}</td>
              <td>{formatPrice(o.total_cents)}</td>
              <td><Link to={`/orders/${o.id}`}>詳細</Link></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
