import { altsContext, criteriaContext } from './edits';
import { slug } from './format';
import { pairCount } from './pairs';
import { assigns, weighs } from './people';
import { groups, leaves } from './tree';
import type { Evaluator, Project } from './types';

/** The newest file format this page understands (model.SchemaVersion). */
const SCHEMA = 2;

/** One screen of the judging page. */
export interface JudgeStep {
  key: string;
  kind: 'weigh' | 'pairwise' | 'rating';
  title: string;
  heading: string; // the question, in full
  context: string; // pairwise context; for ratings, the criterion's alts context
  criterionId: string; // the leaf scored, or the group's parent ('' for the top level)
  items: { id: string; name: string }[];
}

/** What someone is asked to do, in order: weigh each criteria group, then score each leaf. */
export function judgeSteps(p: Project, e: Evaluator): JudgeStep[] {
  const steps: JudgeStep[] = [];
  if (weighs(e.tasks))
    for (const g of groups(p)) {
      const parent = p.criteria.find((c) => c.id === g.parentId);
      steps.push({
        key: `weigh:${g.parentId || 'root'}`,
        kind: 'weigh',
        title: parent ? `Within ${parent.name}, which matters more?` : 'Which matters more?',
        heading: parent ? `Within ${parent.name}, which matters more, and by how much?` : 'Which matters more, and by how much?',
        context: criteriaContext(g.parentId),
        criterionId: g.parentId,
        items: g.items.map((c) => ({ id: c.id, name: c.name })),
      });
    }
  const alts = p.alternatives.map((a) => ({ id: a.id, name: a.name }));
  for (const c of p.alternatives.length >= 2 ? leaves(p) : []) {
    if (c.method === 'value' || !assigns(e.tasks, c)) continue; // measured values are the sender's
    const rating = c.method === 'rating';
    steps.push({
      key: `score:${c.id}`,
      kind: rating ? 'rating' : 'pairwise',
      title: rating ? `Rate each option on ${c.name}` : `Which is better on ${c.name}?`,
      heading: rating ? `Rate each option on ${c.name}` : `Which is better on ${c.name}, and by how much?`,
      context: altsContext(c.id),
      criterionId: c.id,
      items: alts,
    });
  }
  return steps;
}

/** Roughly how long the steps take: about ten seconds an answer, at least a minute. */
export function minutes(p: Project, e: Evaluator): number {
  const answers = judgeSteps(p, e).reduce((n, s) => n + (s.kind === 'rating' ? s.items.length : pairCount(s.items.length)), 0);
  return Math.max(1, Math.round((answers * 10) / 60));
}

const missing = 'This page is missing its decision. Ask the person who sent it for a new invitation.';

/** The decision a judging page was made with, from its embedded JSON. */
export function readPacket(text: string | null | undefined): { project: Project } | { error: string } {
  if (!text?.trim()) return { error: missing };
  try {
    const p = JSON.parse(text) as Project;
    if (!p || typeof p !== 'object' || !Array.isArray(p.evaluators) || p.evaluators.length === 0) return { error: missing };
    if ((p.schemaVersion ?? 1) > SCHEMA)
      return { error: 'This invitation was made by a newer AHP Matrix than this page understands. Ask the person who sent it for a new invitation.' };
    return { project: p };
  } catch {
    return { error: missing };
  }
}

/** The file the answers are saved as, matching the app's "Send your answers back". */
export function answersFile(p: Project): string {
  return `${slug(p.name)}-answers-from-${slug(p.evaluators[0]?.name ?? '')}.ahp.json`;
}
