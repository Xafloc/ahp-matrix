import { getRating } from './edits';
import { judgeSteps } from './judge';
import { pairProgress } from './progress';
import type { Evaluator, PersonProgress, Project } from './types';

/**
 * Whether results are hidden from someone so others' answers don't sway
 * theirs: only when others are judging too and they have judging left.
 */
export function hiddenFor(people: PersonProgress[], evaluatorId: string, count: number): boolean {
  if (count < 2) return false;
  const me = people.find((x) => x.id === evaluatorId);
  return !!me && me.total > 0 && !me.finished;
}

/** Where someone's next unanswered comparison or rating is. */
export function nextTask(p: Project, e: Evaluator): { step: 'weigh' | 'score'; sub: string } | null {
  for (const s of judgeSteps(p, e)) {
    const done =
      s.kind === 'rating'
        ? s.items.every((a) => getRating(p, e.id, s.criterionId, a.id) !== undefined)
        : (({ done, total }) => done === total)(pairProgress(p, e.id, s.context, s.items.map((i) => i.id)));
    if (!done) return s.kind === 'weigh' ? { step: 'weigh', sub: s.criterionId || 'root' } : { step: 'score', sub: s.criterionId };
  }
  return null;
}
