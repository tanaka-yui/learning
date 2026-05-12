import { createContext, useContext, useEffect, useState, useCallback, useRef, ReactNode } from 'react';

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

interface CartContextValue {
  items: CartItem[];
  add: (productId: string, quantity?: number) => void;
  setQuantity: (productId: string, quantity: number) => void;
  remove: (productId: string) => void;
  clear: () => void;
}

const CartContext = createContext<CartContextValue | null>(null);

export function CartProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<CartItem[]>(() => readCart());
  const hydrated = useRef(false);

  useEffect(() => {
    // Skip the very first effect run: state was just hydrated from localStorage,
    // re-writing immediately is wasted work and can clobber storage if interrupted.
    if (!hydrated.current) {
      hydrated.current = true;
      return;
    }
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

  return (
    <CartContext.Provider value={{ items, add, setQuantity, remove, clear }}>
      {children}
    </CartContext.Provider>
  );
}

export function useCart(): CartContextValue {
  const ctx = useContext(CartContext);
  if (!ctx) throw new Error('useCart must be used inside CartProvider');
  return ctx;
}
