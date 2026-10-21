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
  /** What they're asked to judge; omitted means everything. */
  tasks?: Tasks;
  /** Set once a copy has been sent to them. */
  invite?: Invite;
}

export interface Tasks {
  /** Compare the criteria in every sibling group. */
  weigh: boolean;
  /** Score the options on every leaf criterion... */
  scoreAll: boolean;
  /** ...or only on these leaves. */
  score?: string[];
}

export interface Invite {
  sentAt: string;
  /** Fingerprint of the decision when the copy was made. */
  structure: string;
  returnedAt?: string;
}

/** Only in a copy sent to someone. */
export interface PacketInfo {
  for: string;
  from: string;
  sentAt: string;
  structure: string;
}

export type Aggregation = 'judgments' | 'priorities';

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
  origin?: string;
  aggregation?: Aggregation;
  packet?: PacketInfo;
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
  /** The consistency ratio if the suggestion replaced the current answer. */
  crAfter: number;
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
  people: PersonProgress[];
  values: Progress;
  method: Aggregation;
}

export interface Progress {
  done: number;
  total: number;
}

export interface PersonProgress extends Progress {
  id: string;
  finished: boolean;
  consistent: boolean;
  structure: string;
  outOfDate: boolean;
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
  /** A copy someone sent to be judged. */
  received?: boolean;
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
  /** Judgments that couldn't come across, in words. */
  dropped: string[];
}

/** What merging a file would do (model.MergePreview). */
export interface MergePreview {
  id: string;
  name: string;
  people: PreviewPerson[];
  dropped: string[];
  outOfDate: boolean;
}

export interface PreviewPerson {
  id: string;
  name: string;
  judgments: number;
  replacesExisting: boolean;
  previouslyReturnedAt?: string;
}

export const ALL_EVALUATORS = 'all';
