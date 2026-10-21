import { altsContext, criteriaContext, getJudgment, getRating, getValue } from './edits';
import { pairCount } from './pairs';
import { assigns, weighs } from './people';
import { groups, leaves } from './tree';
import type { Criterion, Project } from './types';

export interface Progress {
  done: number;
  total: number;
}

/** How many of the comparisons in one pairwise context an evaluator has made. */
export function pairProgress(p: Project, evaluatorId: string, context: string, items: string[]): Progress {
  let done = 0;
  for (let i = 0; i < items.length; i++)
    for (let j = i + 1; j < items.length; j++) if (getJudgment(p, evaluatorId, context, items[i], items[j]) !== undefined) done++;
  return { done, total: pairCount(items.length) };
}

/** Progress on one leaf criterion, by its scoring method. Measured values are shared, not per evaluator. */
export function leafProgress(p: Project, evaluatorId: string, c: Criterion): Progress {
  const alts = p.alternatives.map((a) => a.id);
  if (c.method === 'rating') return { done: alts.filter((a) => getRating(p, evaluatorId, c.id, a) !== undefined).length, total: alts.length };
  if (c.method === 'value') return { done: alts.filter((a) => getValue(p, c.id, a) !== undefined).length, total: alts.length };
  return pairProgress(p, evaluatorId, altsContext(c.id), alts);
}

const tasksOf = (p: Project, evaluatorId: string) => p.evaluators.find((e) => e.id === evaluatorId)?.tasks;

/** Progress weighing criteria: every sibling group's comparisons, if the person weighs them. */
export function weighProgress(p: Project, evaluatorId: string): Progress {
  if (!weighs(tasksOf(p, evaluatorId))) return { done: 0, total: 0 };
  return sum(groups(p).map((g) => pairProgress(p, evaluatorId, criteriaContext(g.parentId), g.items.map((c) => c.id))));
}

/** Progress scoring alternatives on every leaf the person scores. */
export function scoreProgress(p: Project, evaluatorId: string): Progress {
  const tasks = tasksOf(p, evaluatorId);
  return sum(leaves(p).filter((c) => assigns(tasks, c)).map((c) => leafProgress(p, evaluatorId, c)));
}

function sum(ps: Progress[]): Progress {
  return ps.reduce((t, x) => ({ done: t.done + x.done, total: t.total + x.total }), { done: 0, total: 0 });
}
