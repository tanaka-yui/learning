import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { getProduct, Product } from '../api/products';
import { ErrorBanner } from '../components/ErrorBanner';
import { useCart } from '../hooks/useCart';
import { formatPrice } from '../lib/format';

export default function ProductDetail() {
  const { id } = useParams();
  const [product, setProduct] = useState<Product | null>(null);
  const [error, setError] = useState<unknown>(null);
  const cart = useCart();

  useEffect(() => {
    if (!id) return;
    getProduct(id).then(setProduct).catch(setError);
  }, [id]);

  if (error) return <ErrorBanner error={error} />;
  if (!product) return <p>Loading...</p>;

  return (
    <div>
      <h1>{product.name}</h1>
      <p>{product.description}</p>
      <p><strong>{formatPrice(product.price_cents)}</strong></p>
      <button className="btn" onClick={() => cart.add(product.id)}>Add to cart</button>
    </div>
  );
}
