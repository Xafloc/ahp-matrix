// Folds dist-judge/judge.html and its one script and stylesheet into a single
// self-contained dist/judge.html, which the server fills in for each invitation.
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const from = 'dist-judge';
let html = readFileSync(join(from, 'judge.html'), 'utf8');
const read = (ref) => readFileSync(join(from, ref.replace(/^\//, '')), 'utf8');

html = html.replace(/<script type="module" crossorigin src="([^"]+)"><\/script>/g, (_, src) => `<script type="module">${read(src).replace(/<\/(script)/gi, '<\\/$1')}</script>`);
html = html.replace(/<link rel="stylesheet" crossorigin href="([^"]+)">/g, (_, href) => `<style>${read(href)}</style>`);

const left = html.match(/\s(?:src|href)=["'](?!data:|#)[^"']+/g);
if (left) throw new Error(`judge.html still refers to other files: ${left.join(', ')}`);
if ((html.match(/<script type="application\/json" id="ahp-packet"><\/script>/g) ?? []).length !== 1) throw new Error('judge.html needs exactly one #ahp-packet placeholder');
writeFileSync(join('dist', 'judge.html'), html);
console.log(`dist/judge.html: ${Math.round(html.length / 1024)} KB`);
