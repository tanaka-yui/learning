import { useEffect, useState, useCallback } from 'react';

export interface CartItem {
  productId: string;
  quantity: number;
}

const STORAGE_KEY = 'cart';

export function addToCart(items: CartItem[], productId: string, qty: number): CartItem[] {
  const existing = items.find((it) => it.productId === productId);
  if (existing) {
    return items.map((it) =>
      it.productId === productId ? { ...it, quantity: it.quantity + qty } : it,
    );
  }
  return [...items, { productId, quantity: qty }];
}

export function setQuantityInCart(items: CartItem[], productId: string, quantity: number): CartItem[] {
  if (quantity <= 0) return items.filter((it) => it.productId !== productId);
  return items.map((it) =>
    it.productId === productId ? { ...it, quantity } : it,
  );
}

export function removeFromCart(items: CartItem[], productId: string): CartItem[] {
  return items.filter((it) => it.productId !== productId);
}

export function readCart(): CartItem[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (it): it is CartItem =>
        typeof it === 'object' &&
        it !== null &&
        typeof it.productId === 'string' &&
        typeof it.quantity === 'number',
    );
  } catch {
    return [];
  }
}

export function writeCart(items: CartItem[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(items));
}

export function useCart() {
  const [items, setItems] = useState<CartItem[]>(() => readCart());

  useEffect(() => {
    writeCart(items);
  }, [items]);

  const add = useCallback((productId: string, quantity: number = 1) => {
    setItems((prev) => addToCart(prev, productId, quantity));
  }, []);

  const setQuantity = useCallback((productId: string, quantity: number) => {
    setItems((prev) => setQuantityInCart(prev, productId, quantity));
  }, []);

  const remove = useCallback((productId: string) => {
    setItems((prev) => removeFromCart(prev, productId));
  }, []);

  const clear = useCallback(() => setItems([]), []);

  return { items, add, setQuantity, remove, clear };
}
