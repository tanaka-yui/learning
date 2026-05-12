import { describe, it, expect } from 'vitest';
import { formatPrice, shortTraceId } from './format';

describe('formatPrice', () => {
  it('formats cents to JPY-like string', () => {
    expect(formatPrice(0)).toBe('¥0');
    expect(formatPrice(500)).toBe('¥500');
    expect(formatPrice(123456)).toBe('¥123,456');
  });
});

describe('shortTraceId', () => {
  it('returns first 4 + ellipsis + last 2', () => {
    expect(shortTraceId('4bf92f3577b34da6a3ce929d0e0e4736')).toBe('4bf9…36');
  });

  it('returns empty when input is empty', () => {
    expect(shortTraceId('')).toBe('');
  });

  it('returns input unchanged when too short to shorten', () => {
    expect(shortTraceId('abc')).toBe('abc');
  });
});
