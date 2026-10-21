export type Step = 'setup' | 'people' | 'weigh' | 'score' | 'results' | 'sensitivity';
export const STEPS: Step[] = ['setup', 'people', 'weigh', 'score', 'results', 'sensitivity'];

export type Route = { name: 'library' } | { name: 'project'; id: string; step: Step; sub: string; pair?: string } | { name: 'report'; id: string };

/** Parses a hash path: "/", "/p/<id>/<step>[/<sub>[/<a>|<b>]]" or "/p/<id>/report". */
export function parse(path: string): Route {
  const parts = path.replace(/^#?\/?/, '').split('/').map(decodeURIComponent);
  if (parts[0] === 'p' && parts[1]) {
    if (parts[2] === 'report') return { name: 'report', id: parts[1] };
    const step = (STEPS as string[]).includes(parts[2]) ? (parts[2] as Step) : 'setup';
    const route: Route = { name: 'project', id: parts[1], step, sub: parts[3] ?? '' };
    if (parts[4]) route.pair = parts[4];
    return route;
  }
  return { name: 'library' };
}

export const href = (id: string, step: Step | 'report' = 'setup', sub = '', pair = '') =>
  `#/p/${encodeURIComponent(id)}/${step}${sub ? `/${encodeURIComponent(sub)}` : ''}${sub && pair ? `/${encodeURIComponent(pair)}` : ''}`;

class Router {
  route: Route = $state(parse(typeof location === 'undefined' ? '' : location.hash));
  constructor() {
    if (typeof window !== 'undefined') window.addEventListener('hashchange', () => (this.route = parse(location.hash)));
  }
  go(hash: string) {
    location.hash = hash;
  }
}

export const router = new Router();
