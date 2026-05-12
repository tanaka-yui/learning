import { apiFetch } from './http';

export interface Product {
  id: string;
  name: string;
  description: string;
  price_cents: number;
}

export async function listProducts() {
  const { data } = await apiFetch<{ products: Product[] }>('/api/products');
  return data.products;
}

export async function getProduct(id: string) {
  const { data } = await apiFetch<Product>('/api/products/' + encodeURIComponent(id));
  return data;
}
