import type { Criterion, Project } from './types';

/** Children of parentId ('' for the top level), in project order. */
export function childrenOf(p: Project, parentId = ''): Criterion[] {
  return p.criteria.filter((c) => (c.parentId ?? '') === parentId);
}

export function isLeaf(p: Project, id: string): boolean {
  return !p.criteria.some((c) => c.parentId === id);
}

/** All criteria in depth-first order with their depth (0 = top level). */
export function flatten(p: Project): { criterion: Criterion; depth: number }[] {
  const out: { criterion: Criterion; depth: number }[] = [];
  const walk = (parentId: string, depth: number) => {
    for (const c of childrenOf(p, parentId)) {
      out.push({ criterion: c, depth });
      walk(c.id, depth + 1);
    }
  };
  walk('', 0);
  return out;
}

/** Leaf criteria in depth-first order: the ones that score alternatives. */
export function leaves(p: Project): Criterion[] {
  return flatten(p)
    .map((f) => f.criterion)
    .filter((c) => isLeaf(p, c.id));
}

/** id and every criterion below it. */
export function descendants(p: Project, id: string): string[] {
  const out = [id];
  for (let i = 0; i < out.length; i++) {
    for (const c of p.criteria) if (c.parentId === out[i]) out.push(c.id);
  }
  return out;
}

/** A sibling group whose members are compared pairwise: the top level or a criterion's children. */
export interface Group {
  parentId: string; // '' for the top level
  items: Criterion[];
}

/** Sibling groups with at least two members, top level first, in depth-first order. */
export function groups(p: Project): Group[] {
  const parents = ['', ...flatten(p).map((f) => f.criterion.id)];
  return parents
    .map((parentId) => ({ parentId, items: childrenOf(p, parentId) }))
    .filter((g) => g.items.length >= 2);
}

/** Names from the top down to id: ["Technical", "Performance"]. */
export function path(p: Project, id: string): string[] {
  const byId = new Map(p.criteria.map((c) => [c.id, c]));
  const out: string[] = [];
  for (let c = byId.get(id); c; c = c.parentId ? byId.get(c.parentId) : undefined) out.unshift(c.name);
  return out;
}
