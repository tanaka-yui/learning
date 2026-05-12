import { Link } from 'react-router-dom';
import { Product } from '../api/products';
import { formatPrice } from '../lib/format';

export function ProductCard({ product, onAdd }: { product: Product; onAdd: () => void }) {
  return (
    <div className="card">
      <h3><Link to={`/products/${product.id}`}>{product.name}</Link></h3>
      <p className="muted">{product.description}</p>
      <div className="row">
        <strong>{formatPrice(product.price_cents)}</strong>
        <span style={{ flex: 1 }} />
        <button className="btn" onClick={onAdd}>Add</button>
      </div>
    </div>
  );
}
