import type { Project } from './types';

/** One validation failure from the server (model.Problem). */
export interface Problem {
  field: string;
  message: string;
}

/**
 * Turns the server's validation problems into sentences that say what to fix,
 * naming the item where it has a name and its position where it doesn't.
 */
export function explain(problems: Problem[], p: Project): string[] {
  return problems.map(({ field, message }) => {
    if (field === 'name') return 'The decision needs a name.';
    const m = field.match(/^(alternatives|criteria|evaluators)\[(\d+)\]\.(\w+)$/);
    if (m) {
      const [, list, index, prop] = m;
      const i = Number(index);
      const noun = { alternatives: 'Option', criteria: 'Criterion', evaluators: 'Person' }[list];
      const name = (p[list as 'alternatives' | 'criteria' | 'evaluators'][i]?.name ?? '').trim() || `${noun} ${i + 1}`;
      if (prop === 'name') return `${name} needs a name.`;
      if (prop === 'weight') return `${name}'s weight must be more than 0.`;
    }
    return `${field} ${message}.`;
  });
}
