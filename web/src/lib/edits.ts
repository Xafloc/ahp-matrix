// Pure edits to a project document. Each function changes p in place, the
// same way the server's Prepare would leave it, so the UI never shows data the
// server is about to drop.
import { snap } from './scale';
import { childrenOf, descendants } from './tree';
import type { Alternative, Criterion, Direction, Evaluator, Method, Project, Scale } from './types';

export const SERIES_SLOTS = 8;

/** A short random id for an item inside a project (like model.NewID). */
export function newId(): string {
  const alphabet = 'abcdefghijklmnopqrstuvwxyz234567';
  const bytes = crypto.getRandomValues(new Uint8Array(8));
  return Array.from(bytes, (b) => alphabet[b % 32]).join('');
}

export const criteriaContext = (parentId = '') => `criteria:${parentId || 'root'}`;
export const altsContext = (criterionId: string) => `alts:${criterionId}`;

// Alternatives

/** Adds an alternative with the first unused series color ('' after eight). */
export function addAlternative(p: Project, name: string): Alternative {
  const used = new Set(p.alternatives.map((a) => a.color));
  let color = '';
  for (let k = 1; k <= SERIES_SLOTS && !color; k++) if (!used.has(`series-${k}`)) color = `series-${k}`;
  const a: Alternative = { id: newId(), name: name.trim(), description: '', color };
  p.alternatives.push(a);
  return a;
}

export function removeAlternative(p: Project, id: string): void {
  p.alternatives = p.alternatives.filter((a) => a.id !== id);
  p.pairwise = p.pairwise.filter((j) => !(j.context.startsWith('alts:') && (j.a === id || j.b === id)));
  p.ratings = p.ratings.filter((r) => r.alternativeId !== id);
  p.values = p.values.filter((v) => v.alternativeId !== id);
}

// Criteria

export function addCriterion(p: Project, name: string, parentId = ''): Criterion {
  const c: Criterion = { id: newId(), name: name.trim(), description: '', notes: '', method: 'pairwise' };
  if (parentId) c.parentId = parentId;
  p.criteria.push(c);
  pruneJudgments(p);
  return c;
}

/** Removes a criterion, everything below it, and every judgment that refers to them. */
export function removeCriterion(p: Project, id: string): void {
  const gone = new Set(descendants(p, id));
  p.criteria = p.criteria.filter((c) => !gone.has(c.id));
  p.ratings = p.ratings.filter((r) => !gone.has(r.criterionId));
  p.values = p.values.filter((v) => !gone.has(v.criterionId));
  pruneJudgments(p);
}

export type Placement = 'before' | 'after' | 'inside';

/**
 * Moves criterion id relative to target: as its previous or next sibling, or
 * as its last child. Returns false (and changes nothing) for a move into its
 * own subtree.
 */
export function moveCriterion(p: Project, id: string, targetId: string, where: Placement): boolean {
  if (descendants(p, id).includes(targetId)) return false;
  const moving = p.criteria.find((c) => c.id === id);
  const target = p.criteria.find((c) => c.id === targetId);
  if (!moving || !target) return false;
  p.criteria = p.criteria.filter((c) => c.id !== id);
  setParent(moving, where === 'inside' ? target.id : (target.parentId ?? ''));
  let at = p.criteria.indexOf(target);
  if (where !== 'before') at += 1;
  if (where === 'inside') {
    // after the target's current last child, so it lands at the end
    const kids = childrenOf(p, target.id);
    if (kids.length) at = p.criteria.indexOf(kids[kids.length - 1]) + 1;
  }
  p.criteria.splice(at, 0, moving);
  pruneJudgments(p);
  return true;
}

/** Makes a criterion the last child of its previous sibling. */
export function indent(p: Project, id: string): boolean {
  const c = p.criteria.find((x) => x.id === id);
  if (!c) return false;
  const sibs = childrenOf(p, c.parentId ?? '');
  const i = sibs.indexOf(c);
  return i > 0 && moveCriterion(p, id, sibs[i - 1].id, 'inside');
}

/** Makes a criterion the next sibling of its parent. */
export function outdent(p: Project, id: string): boolean {
  const c = p.criteria.find((x) => x.id === id);
  return !!c?.parentId && moveCriterion(p, id, c.parentId, 'after');
}

/** Swaps a criterion with its previous (-1) or next (+1) sibling. */
export function shift(p: Project, id: string, dir: -1 | 1): boolean {
  const c = p.criteria.find((x) => x.id === id);
  if (!c) return false;
  const sibs = childrenOf(p, c.parentId ?? '');
  const other = sibs[sibs.indexOf(c) + dir];
  if (!other) return false;
  const i = p.criteria.indexOf(c);
  const j = p.criteria.indexOf(other);
  [p.criteria[i], p.criteria[j]] = [p.criteria[j], p.criteria[i]];
  return true;
}

function setParent(c: Criterion, parentId: string) {
  if (parentId) c.parentId = parentId;
  else delete c.parentId;
}

export function setMethod(p: Project, id: string, method: Method): void {
  const c = p.criteria.find((x) => x.id === id);
  if (!c) return;
  c.method = method;
  if (method === 'value') c.direction ??= 'higher';
}

export function setDirection(p: Project, id: string, direction: Direction): void {
  const c = p.criteria.find((x) => x.id === id);
  if (c) c.direction = direction;
}

/**
 * Drops pairwise judgments whose context no longer exists or whose items are
 * no longer siblings in it (mirrors the server's prune). Judgments of
 * alternatives survive while their criterion exists, even if it gains children.
 */
export function pruneJudgments(p: Project): void {
  const crit = new Map(p.criteria.map((c) => [c.id, c]));
  const alts = new Set(p.alternatives.map((a) => a.id));
  const evals = new Set(p.evaluators.map((e) => e.id));
  p.pairwise = p.pairwise.filter((j) => {
    if (!evals.has(j.evaluatorId) || j.a === j.b) return false;
    const [kind, id] = j.context.split(':');
    if (kind === 'alts') return crit.has(id) && alts.has(j.a) && alts.has(j.b);
    const parent = id === 'root' ? '' : id;
    if (parent && !crit.has(parent)) return false;
    return (crit.get(j.a)?.parentId ?? '') === parent && (crit.get(j.b)?.parentId ?? '') === parent && crit.has(j.a) && crit.has(j.b);
  });
}

// Judgments

/** How strongly a is preferred over b, or undefined if the pair isn't judged. */
export function getJudgment(p: Project, evaluatorId: string, context: string, a: string, b: string): number | undefined {
  for (const j of p.pairwise) {
    if (j.evaluatorId !== evaluatorId || j.context !== context) continue;
    if (j.a === a && j.b === b) return j.value;
    if (j.a === b && j.b === a) return 1 / j.value;
  }
  return undefined;
}

/** Records (or with undefined, clears) how strongly a is preferred over b. */
export function setJudgment(p: Project, evaluatorId: string, context: string, a: string, b: string, value: number | undefined): void {
  p.pairwise = p.pairwise.filter(
    (j) => !(j.evaluatorId === evaluatorId && j.context === context && ((j.a === a && j.b === b) || (j.a === b && j.b === a))),
  );
  if (value !== undefined) p.pairwise.push({ evaluatorId, context, a, b, value });
}

export function getRating(p: Project, evaluatorId: string, criterionId: string, alternativeId: string): number | undefined {
  return p.ratings.find((r) => r.evaluatorId === evaluatorId && r.criterionId === criterionId && r.alternativeId === alternativeId)?.value;
}

export function setRating(p: Project, evaluatorId: string, criterionId: string, alternativeId: string, value: number | undefined): void {
  p.ratings = p.ratings.filter((r) => !(r.evaluatorId === evaluatorId && r.criterionId === criterionId && r.alternativeId === alternativeId));
  if (value !== undefined) p.ratings.push({ evaluatorId, criterionId, alternativeId, value });
}

export function getValue(p: Project, criterionId: string, alternativeId: string): number | undefined {
  return p.values.find((v) => v.criterionId === criterionId && v.alternativeId === alternativeId)?.value;
}

export function setValue(p: Project, criterionId: string, alternativeId: string, value: number | undefined): void {
  p.values = p.values.filter((v) => !(v.criterionId === criterionId && v.alternativeId === alternativeId));
  if (value !== undefined && Number.isFinite(value)) p.values.push({ criterionId, alternativeId, value });
}

// Evaluators and scale

export function addEvaluator(p: Project, name: string): Evaluator {
  const e: Evaluator = { id: newId(), name: name.trim(), weight: 1 };
  p.evaluators.push(e);
  return e;
}

/** Removes an evaluator and their judgments. The last evaluator can't be removed. */
export function removeEvaluator(p: Project, id: string): boolean {
  if (p.evaluators.length <= 1) return false;
  p.evaluators = p.evaluators.filter((e) => e.id !== id);
  p.pairwise = p.pairwise.filter((j) => j.evaluatorId !== id);
  p.ratings = p.ratings.filter((r) => r.evaluatorId !== id);
  return true;
}

/** Switches scale, rounding every judgment to the nearest value on the new one. */
export function changeScale(p: Project, scale: Scale): void {
  p.scale = scale;
  for (const j of p.pairwise) j.value = snap(j.value, scale);
}
