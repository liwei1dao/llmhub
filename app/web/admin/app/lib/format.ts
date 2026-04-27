export function fmtCents(cents: number, currency = 'CNY'): string {
  return (cents / 100).toLocaleString('zh-CN', { style: 'currency', currency });
}
export function fmtDateTime(iso: string | null | undefined): string {
  if (!iso) return '-';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString('zh-CN', { hour12: false });
}
export function fmtNumber(n: number): string {
  return n.toLocaleString('zh-CN');
}
