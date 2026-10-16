// Mirrors the JSON written by internal/model (project.go, compute.go) and the
// summaries from internal/server (projects.go). Keep the field names in sync.

export type Scale = 'saaty9' | 'classic5';
export type Method = 'pairwise' | 'rating' | 'value';
export type Direction = 'higher' | 'lower';

export interface Alternative {
  id: string;
  name: string;
  description: string;
  color: string;
}

export interface Criterion {
  id: string;
  parentId?: string;
  name: string;
  description: string;
  notes: string;
  method?: Method;
  direction?: Direction;
  unit?: string;
}

export interface Evaluator {
  id: string;
  name: string;
  weight: number;
}

export interface PairwiseJudgment {
  evaluatorId: string;
  context: string;
  a: string;
  b: string;
  value: number;
  note?: string;
}

export interface Rating {
  evaluatorId: string;
  criterionId: string;
  alternativeId: string;
  value: number;
}

export interface Value {
  criterionId: string;
  alternativeId: string;
  value: number;
}

export interface Project {
  schemaVersion: number;
  id: string;
  version: number;
  name: string;
  description: string;
  createdAt: string;
  updatedAt: string;
  scale: Scale;
  alternatives: Alternative[];
  criteria: Criterion[];
  evaluators: Evaluator[];
  pairwise: PairwiseJudgment[];
  ratings: Rating[];
  values: Value[];
}

export interface AltScore {
  id: string;
  score: number;
  rank: number;
}

export interface CriterionResult {
  id: string;
  parentId?: string;
  local: number;
  global: number;
  subtree: number[];
  stability: number | null;
}

export interface PairSuggestion {
  a: string;
  b: string;
  current: number;
  suggested: number;
}

export interface MatrixReport {
  context: string;
  items: string[];
  weights: number[] | null;
  cr: number;
  threshold: number;
  consistent: boolean;
  complete: boolean;
  connected: boolean;
  missing: number;
  pairs: number;
  components?: string[][];
  suggestions: PairSuggestion[];
  /** Combined matrix in items order: matrix[i][j] is how strongly item i is preferred over j; 0 if unjudged. */
  matrix: number[][];
}

export interface Issue {
  code: string;
  message: string;
  criterionId?: string;
  context?: string;
}

export interface Disagreement {
  context: string;
  a: string;
  b: string;
  spread: number;
  values: Record<string, number>;
}

export interface Results {
  evaluator: string;
  scores: AltScore[];
  criteria: CriterionResult[];
  matrices: MatrixReport[];
  issues: Issue[];
  disagreements: Disagreement[];
  completeness: number;
  consistent: boolean;
}

export interface RankedName {
  name: string;
  color: string;
  score: number;
}

export interface ProjectSummary {
  id?: string;
  file: string;
  name: string;
  description: string;
  updatedAt: string;
  alternatives: number;
  criteria: number;
  ranking: RankedName[];
  completeness: number;
  consistent: boolean;
  error?: string;
}

/** A bundled sample decision or template (internal/samples). */
export interface StarterEntry {
  id: string;
  name: string;
  description: string;
  alternatives: number;
  criteria: number;
}

/** What importing a legacy workbook found (internal/xlsx). */
export interface ImportReport {
  alternatives: number;
  criteria: number;
  judgments: number;
  skipped: string[];
}

/** What merging a returned file did (model.MergeReport). */
export interface MergeReport {
  added: string[];
  replaced: string[];
  skipped: string[];
  judgments: number;
  dropped: number;
}

export const ALL_EVALUATORS = 'all';
