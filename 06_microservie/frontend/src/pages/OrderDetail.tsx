import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { getOrder, Order } from '../api/orders';
import { ErrorBanner } from '../components/ErrorBanner';
import { TraceIdChip } from '../components/TraceIdChip';
import { formatPrice } from '../lib/format';

export default function OrderDetail() {
  const { id } = useParams();
  const [order, setOrder] = useState<Order | null>(null);
  const [traceId, setTraceId] = useState('');
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    if (!id) return;
    getOrder(id)
      .then((r) => { setOrder(r.data); setTraceId(r.traceId); })
      .catch(setError);
  }, [id]);

  if (error) return <ErrorBanner error={error} />;
  if (!order) return <p>Loading...</p>;

  return (
    <div>
      <h1>注文詳細</h1>
      <p>Order ID: <code>{order.id}</code></p>
      <p>Status: {order.status}</p>
      <table>
        <thead><tr><th>商品 ID</th><th>数量</th><th>単価</th><th>小計</th></tr></thead>
        <tbody>
          {order.items.map((it) => (
            <tr key={it.product_id}>
              <td><code>{it.product_id}</code></td>
              <td>{it.quantity}</td>
              <td>{formatPrice(it.unit_price_cents)}</td>
              <td>{formatPrice(it.unit_price_cents * it.quantity)}</td>
            </tr>
          ))}
        </tbody>
        <tfoot><tr><th colSpan={3}>合計</th><th>{formatPrice(order.total_cents)}</th></tr></tfoot>
      </table>
      <p><TraceIdChip traceId={traceId} /></p>
    </div>
  );
}
