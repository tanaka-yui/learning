import { apiFetch } from './http';

export interface CheckoutItem {
  product_id: string;
  quantity: number;
}

export interface CheckoutResponse {
  order_id: string;
  status: string;
}

export async function postCheckout(items: CheckoutItem[]) {
  return apiFetch<CheckoutResponse>('/api/checkout', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ items }),
  });
}
