import './fonts.css';
import '../app.css';
import { mount } from 'svelte';
import Judge from './Judge.svelte';

try {
  const theme = localStorage.getItem('theme');
  if (theme === 'light' || theme === 'dark') document.documentElement.dataset.theme = theme;
} catch {
  // storage can be unavailable (a page opened from disk, private browsing); the OS setting applies
}

mount(Judge, { target: document.getElementById('app')!, props: { packet: document.getElementById('ahp-packet')?.textContent ?? null } });
