export function formatPrice(cents: number): string {
  return '¥' + cents.toLocaleString('en-US');
}

export function shortTraceId(id: string): string {
  if (!id) return '';
  if (id.length <= 6) return id;
  return id.slice(0, 4) + '…' + id.slice(-2);
}
