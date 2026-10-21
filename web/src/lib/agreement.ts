import { list } from './format';

const TIE = 0.0005; // scores this close count as level

/**
 * One line on whether people's own rankings agree on the winner. People
 * with no scores (nothing answered) are left out and named. Empty when
 * fewer than two people have answered.
 */
export function agreement(people: { name: string; scores: { id: string; score: number }[] | null }[], names: Map<string, string>): string {
  const answered = people.filter((x) => x.scores && x.scores.length);
  if (answered.length < 2) return '';
  const missing = people.filter((x) => !x.scores || !x.scores.length).map((x) => x.name);
  const tops = answered.map((x) => {
    const best = Math.max(...x.scores!.map((s) => s.score));
    return { name: x.name, top: x.scores!.filter((s) => s.score >= best - TIE).map((s) => s.id) };
  });
  const key = (t: string[]) => t.join('|');
  const pick = (t: string[]) => list(t.map((id) => names.get(id) ?? id));

  const groups: { names: string[]; top: string[] }[] = [];
  for (const t of tops) {
    const g = groups.find((x) => key(x.top) === key(t.top));
    if (g) g.names.push(t.name);
    else groups.push({ names: [t.name], top: t.top });
  }

  let line: string;
  if (groups.length === 1 && groups[0].top.length === 1) {
    const who = answered.length === 2 ? 'Both people' : `All ${answered.length} people`;
    line = `${who}${missing.length ? " who've answered" : ''} rank ${pick(groups[0].top)} first.`;
  } else {
    const parts = groups.map((g) => {
      const many = g.names.length > 1;
      return g.top.length === 1 ? `${list(g.names)} ${many ? 'pick' : 'picks'} ${pick(g.top)}` : `${list(g.names)} ${many ? 'put' : 'puts'} ${pick(g.top)} level`;
    });
    line = `People disagree on the winner: ${parts.join('; ')}.`;
  }
  if (missing.length) line += ` ${list(missing)} ${missing.length === 1 ? "hasn't" : "haven't"} answered yet.`;
  return line;
}
