import type { Aggregation } from './types';

/** How several people's judgments become one result (see model.Compute). */
export function combineNote(method: Aggregation = 'judgments'): string {
  if (method === 'priorities')
    return "Each person's ranking is worked out from their own answers, then the rankings are combined with a weighted geometric mean. The criteria weights and breakdowns below show everyone's answers combined. Change the method and each person's weight in the People step.";
  return "Comparisons are combined with a weighted geometric mean, the standard for AHP, so if one person says 3 and another says 1/3 they cancel to equal. 1–10 ratings use a weighted average. Change the method and each person's weight in the People step.";
}
