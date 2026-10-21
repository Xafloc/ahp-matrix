import type { Progress } from './progress';
import { groups, leaves } from './tree';
import { list } from './format';
import type { Criterion, Evaluator, PacketInfo, PersonProgress, Project, Tasks } from './types';

/** What a person is asked to judge, as the People step offers it. */
export type Role = 'all' | 'weigh' | 'score' | 'custom';

export const ROLES: { id: Role; label: string }[] = [
  { id: 'all', label: 'Everything' },
  { id: 'weigh', label: 'Weigh criteria' },
  { id: 'score', label: 'Score options' },
  { id: 'custom', label: 'Custom' },
];

export function roleOf(tasks: Tasks | undefined): Role {
  if (!tasks || (tasks.weigh && tasks.scoreAll)) return 'all';
  const some = (tasks.score?.length ?? 0) > 0;
  if (tasks.weigh && !tasks.scoreAll && !some) return 'weigh';
  if (!tasks.weigh && tasks.scoreAll) return 'score';
  return 'custom';
}

/**
 * The tasks for a role. Custom starts from what the person already had, with
 * "every leaf" spelled out so it can be unticked one at a time.
 */
export function tasksFor(role: Role, leaves: string[], current?: Tasks): Tasks | undefined {
  switch (role) {
    case 'all':
      return undefined;
    case 'weigh':
      return { weigh: true, scoreAll: false };
    case 'score':
      return { weigh: false, scoreAll: true };
    case 'custom': {
      const weigh = current ? current.weigh : true;
      const scoresAll = !current || current.scoreAll;
      return { weigh, scoreAll: false, score: scoresAll ? [...leaves] : [...(current?.score ?? [])] };
    }
  }
}

export type Status =
  | { kind: 'you' }
  | { kind: 'not-invited' }
  | { kind: 'invited'; at: string }
  | { kind: 'returned'; at: string }
  | { kind: 'out-of-date'; at: string };

/** Where a person's copy is. The first person is the one running the decision. */
export function statusOf(e: Evaluator, progress: PersonProgress | undefined, first: boolean): Status {
  if (first) return { kind: 'you' };
  if (!e.invite) return { kind: 'not-invited' };
  if (progress?.outOfDate) return { kind: 'out-of-date', at: e.invite.sentAt };
  if (e.invite.returnedAt) return { kind: 'returned', at: e.invite.returnedAt };
  return { kind: 'invited', at: e.invite.sentAt };
}

/** A short nudge to paste into an email or chat. */
export function reminder(decision: string, person: string, progress: Progress, from: string): string {
  const left = progress.total - progress.done;
  const where =
    progress.total === 0 || progress.done === 0
      ? 'Could you make your comparisons when you have a few minutes?'
      : left === 0
        ? "It looks like you've finished: could you send the file back?"
        : `You've got ${left} of ${progress.total} comparisons left.`;
  return `Hi ${person}, a reminder about "${decision}". ${where} Open the file I sent with Import file in AHP Matrix, then export it from the library and send it back to me. Thanks, ${from}`;
}

/** Whether someone with these tasks compares the criteria. */
export const weighs = (tasks: Tasks | undefined) => !tasks || tasks.weigh;

/**
 * Whether someone with these tasks scores the options on leaf c. Measured
 * values go with scoring everything: they're entered once, not judged.
 */
export function assigns(tasks: Tasks | undefined, c: Criterion): boolean {
  if (!tasks || tasks.scoreAll) return true;
  return c.method !== 'value' && (tasks.score ?? []).includes(c.id);
}

/** Where to open the decision for someone: their first assigned task. */
export function firstTask(p: Project, e: Evaluator): { step: 'weigh' | 'score' | 'results'; sub: string } {
  const g = groups(p);
  if (weighs(e.tasks) && g.length) return { step: 'weigh', sub: g[0].parentId || 'root' };
  const mine = leaves(p).filter((c) => assigns(e.tasks, c));
  const leaf = mine.find((c) => c.method !== 'value') ?? mine[0];
  return leaf ? { step: 'score', sub: leaf.id } : { step: 'results', sub: '' };
}

/** Who sent a copy, if they named themselves (new decisions start with "Me"). */
export function sender(packet: PacketInfo | undefined): string {
  const from = packet?.from.trim() ?? '';
  return from === 'Me' ? '' : from;
}

/** "weigh the criteria and score the options on Speed and Cost". */
export function describeTasks(p: Project, tasks: Tasks | undefined): string {
  const parts: string[] = [];
  if (weighs(tasks)) parts.push('weigh the criteria');
  if (!tasks || tasks.scoreAll) parts.push('score the options on every criterion');
  else {
    const names = leaves(p)
      .filter((c) => tasks.score?.includes(c.id))
      .map((c) => c.name);
    if (names.length) parts.push(`score the options on ${list(names)}`);
  }
  return parts.length ? parts.join(' and ') : 'nothing yet';
}
