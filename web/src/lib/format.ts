/** 0.4567 → "45.7%"; values under 10% keep one decimal, larger ones too, for comparability. */
export const percent = (x: number, digits = 1) => `${(x * 100).toFixed(digits)}%`;

/** "1 criterion", "6 criteria". */
export const count = (n: number, one: string, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;

/** A CSS color for an alternative's stored color: a series slot ("series-3") or a hex value. */
export function colorOf(color: string): string {
  if (/^series-[1-8]$/.test(color)) return `var(--${color})`;
  if (/^#[0-9a-f]{3,8}$/i.test(color)) return color;
  return 'var(--series-none)';
}

/** "just now", "5 minutes ago", "yesterday", "3 Mar". */
export function relativeTime(iso: string, now = Date.now()): string {
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return '';
  const s = Math.round((now - t) / 1000);
  if (s < 60) return 'just now';
  const m = Math.round(s / 60);
  if (m < 60) return `${m} minute${m === 1 ? '' : 's'} ago`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h} hour${h === 1 ? '' : 's'} ago`;
  const d = Math.round(h / 24);
  if (d === 1) return 'yesterday';
  if (d < 7) return `${d} days ago`;
  return new Date(t).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: d > 300 ? 'numeric' : undefined });
}
