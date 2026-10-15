import '@fontsource-variable/bricolage-grotesque';
import '@fontsource-variable/hanken-grotesk';
import './app.css';
import { mount } from 'svelte';
import App from './App.svelte';

try {
  const theme = localStorage.getItem('theme');
  if (theme === 'light' || theme === 'dark') document.documentElement.dataset.theme = theme;
} catch {
  // storage can be unavailable; the OS setting applies
}

mount(App, { target: document.getElementById('app')! });
