import { describe, it, expect, beforeEach } from 'vitest';
import {
  addToCart,
  setQuantityInCart,
  removeFromCart,
  readCart,
  writeCart,
} from './useCart';

describe('cart reducers', () => {
  it('addToCart appends new item', () => {
    expect(addToCart([], 'p-001', 2)).toEqual([{ productId: 'p-001', quantity: 2 }]);
  });

  it('addToCart increments existing item', () => {
    const before = [{ productId: 'p-001', quantity: 1 }];
    expect(addToCart(before, 'p-001', 3)).toEqual([{ productId: 'p-001', quantity: 4 }]);
  });

  it('setQuantityInCart updates value', () => {
    const before = [{ productId: 'p-001', quantity: 1 }];
    expect(setQuantityInCart(before, 'p-001', 5)).toEqual([{ productId: 'p-001', quantity: 5 }]);
  });

  it('setQuantityInCart with 0 removes', () => {
    const before = [{ productId: 'p-001', quantity: 1 }];
    expect(setQuantityInCart(before, 'p-001', 0)).toEqual([]);
  });

  it('removeFromCart filters', () => {
    const before = [{ productId: 'a', quantity: 1 }, { productId: 'b', quantity: 1 }];
    expect(removeFromCart(before, 'a')).toEqual([{ productId: 'b', quantity: 1 }]);
  });
});

describe('cart storage', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('readCart returns [] when no storage', () => {
    expect(readCart()).toEqual([]);
  });

  it('writeCart then readCart round-trips', () => {
    writeCart([{ productId: 'p-001', quantity: 1 }]);
    expect(readCart()).toEqual([{ productId: 'p-001', quantity: 1 }]);
  });

  it('readCart recovers from broken JSON', () => {
    localStorage.setItem('cart', '{{not json');
    expect(readCart()).toEqual([]);
  });

  it('readCart filters non-conforming items', () => {
    localStorage.setItem('cart', JSON.stringify([{ productId: 'a', quantity: 1 }, { bogus: true }]));
    expect(readCart()).toEqual([{ productId: 'a', quantity: 1 }]);
  });
});
