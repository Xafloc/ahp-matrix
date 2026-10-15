import type { Project } from './types';

/** A small project: Technical (Performance, Security) and Cost; alternatives a1–a3; evaluators e1, e2. */
export function fixture(): Project {
  return {
    schemaVersion: 1,
    id: 'P1',
    version: 1,
    name: 'Fixture',
    description: '',
    createdAt: '2026-10-15T12:00:00Z',
    updatedAt: '2026-10-15T12:00:00Z',
    scale: 'saaty9',
    alternatives: [
      { id: 'a1', name: 'Alpha', description: '', color: 'series-1' },
      { id: 'a2', name: 'Beta', description: '', color: 'series-2' },
      { id: 'a3', name: 'Gamma', description: '', color: 'series-3' },
    ],
    criteria: [
      { id: 'c1', name: 'Technical', description: '', notes: '' },
      { id: 'c2', parentId: 'c1', name: 'Performance', description: '', notes: '', method: 'pairwise' },
      { id: 'c3', parentId: 'c1', name: 'Security', description: '', notes: '', method: 'rating' },
      { id: 'c4', name: 'Cost', description: '', notes: '', method: 'value', direction: 'lower' },
    ],
    evaluators: [
      { id: 'e1', name: 'Ann', weight: 1 },
      { id: 'e2', name: 'Bob', weight: 1 },
    ],
    pairwise: [
      { evaluatorId: 'e1', context: 'criteria:root', a: 'c1', b: 'c4', value: 3 },
      { evaluatorId: 'e1', context: 'criteria:c1', a: 'c2', b: 'c3', value: 1 },
      { evaluatorId: 'e1', context: 'alts:c2', a: 'a1', b: 'a2', value: 2 },
    ],
    ratings: [{ evaluatorId: 'e1', criterionId: 'c3', alternativeId: 'a1', value: 5 }],
    values: [{ criterionId: 'c4', alternativeId: 'a1', value: 100 }],
  };
}
