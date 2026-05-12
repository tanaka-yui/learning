import { apiFetch } from './http';

export interface OrderItem {
  product_id: string;
  quantity: number;
  unit_price_cents: number;
}

export interface Order {
  id: string;
  user_id: string;
  status: string;
  total_cents: number;
  items: OrderItem[];
}

export async function listOrders() {
  const { data } = await apiFetch<{ orders: Order[] }>('/api/orders');
  return data.orders;
}

export async function getOrder(id: string) {
  return apiFetch<Order>('/api/orders/' + encodeURIComponent(id));
}
