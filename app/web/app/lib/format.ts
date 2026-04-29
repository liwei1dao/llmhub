// Currency / number helpers shared across console pages.

export function fmtCents(cents: number, currency = 'CNY'): string {
  return (cents / 100).toLocaleString('zh-CN', {
    style: 'currency',
    currency,
    maximumFractionDigits: 2,
  });
}

export function fmtNumber(n: number): string {
  return n.toLocaleString('zh-CN');
}

export function fmtDateTime(iso: string): string {
  if (!iso) return '-';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString('zh-CN', { hour12: false });
}
