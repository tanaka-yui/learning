import { useEffect, useState } from 'react';
import { listProducts, Product } from '../api/products';
import { ProductCard } from '../components/ProductCard';
import { ErrorBanner } from '../components/ErrorBanner';
import { useCart } from '../hooks/useCart';

export default function Products() {
  const [products, setProducts] = useState<Product[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  const cart = useCart();

  useEffect(() => {
    listProducts().then(setProducts).catch(setError);
  }, []);

  if (error) return <ErrorBanner error={error} />;
  if (!products) return <p>Loading...</p>;

  return (
    <div>
      <h1>商品一覧</h1>
      <div className="product-grid">
        {products.map((p) => (
          <ProductCard key={p.id} product={p} onAdd={() => cart.add(p.id)} />
        ))}
      </div>
    </div>
  );
}
