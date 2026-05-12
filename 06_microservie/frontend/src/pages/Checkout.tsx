import { useState } from 'react';
import { Link } from 'react-router-dom';
import { postCheckout } from '../api/checkout';
import { useCart } from '../hooks/useCart';
import { ErrorBanner } from '../components/ErrorBanner';
import { TraceIdChip } from '../components/TraceIdChip';

interface Success {
  orderId: string;
  status: string;
  traceId: string;
}

export default function Checkout() {
  const cart = useCart();
  const [error, setError] = useState<unknown>(null);
  const [success, setSuccess] = useState<Success | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onConfirm() {
    setError(null);
    setSubmitting(true);
    try {
      const result = await postCheckout(
        cart.items.map((it) => ({ product_id: it.productId, quantity: it.quantity })),
      );
      setSuccess({ orderId: result.data.order_id, status: result.data.status, traceId: result.traceId });
      cart.clear();
    } catch (err) {
      setError(err);
    } finally {
      setSubmitting(false);
    }
  }

  if (success) {
    return (
      <div>
        <h1>注文が確定しました</h1>
        <p>Order ID: <code>{success.orderId}</code></p>
        <p>Status: {success.status}</p>
        <p><TraceIdChip traceId={success.traceId} /></p>
        <p><Link to="/orders" className="btn">注文履歴を見る</Link></p>
      </div>
    );
  }

  if (cart.items.length === 0) {
    return (
      <div>
        <h1>注文確定</h1>
        <p className="muted">カートが空です。</p>
        <Link to="/" className="btn btn-secondary">商品一覧へ</Link>
      </div>
    );
  }

  return (
    <div>
      <h1>注文確定</h1>
      <ErrorBanner error={error} />
      <p>{cart.items.length} 種類の商品を注文します。</p>
      <button className="btn" onClick={onConfirm} disabled={submitting}>
        {submitting ? '送信中...' : '注文する'}
      </button>
    </div>
  );
}
