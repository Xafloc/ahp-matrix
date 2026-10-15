import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { expect, test } from 'vitest';

// WCAG contrast of the design tokens, read straight from app.css, in both themes.
const css = readFileSync(fileURLToPath(new URL('../app.css', import.meta.url)), 'utf8');

function tokens(block: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const m of block.matchAll(/--([\w-]+):\s*(#[0-9a-f]{6})/gi)) out[m[1]] = m[2];
  return out;
}
const light = tokens(css.slice(0, css.indexOf('@media (prefers-color-scheme: dark)')));
const dark = tokens(css.slice(css.indexOf(":root[data-theme='dark']")));

function luminance(hex: string) {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}
function contrast(a: string, b: string) {
  const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
}

// [text, background]: every pairing the UI uses for small text.
const pairs: [string, string][] = [
  ['ink', 'fog'], ['ink', 'paper'], ['ink-2', 'fog'], ['ink-2', 'paper'], ['ink-3', 'fog'], ['ink-3', 'paper'],
  ['brass-ink', 'paper'], ['rust', 'paper'], ['rust', 'rust-wash'], ['good', 'paper'], ['paper', 'brass-ink'], ['paper', 'ink'],
];

test.each([['light', light], ['dark', dark]] as const)('%s theme text meets WCAG AA (4.5:1)', (_name, t) => {
  for (const [fg, bg] of pairs) {
    expect(contrast(t[fg], t[bg]), `${fg} on ${bg}`).toBeGreaterThanOrEqual(4.5);
  }
});
