import { matrixReport } from './ahp';
import { altsContext, criteriaContext } from './edits';
import { groups, leaves } from './tree';
import type { MatrixReport, Project } from './types';

/** What the compare screens need from a session (ProjectSession and LocalSession both fit). */
export interface CompareSession {
  project: Project;
  scoringAs: string;
  results: Record<string, { matrices: MatrixReport[] } | undefined>;
  watch(evaluator: string): void;
  changed(): void;
}

type Store = { getItem(key: string): string | null; setItem(key: string, value: string): void };

function browserStorage(): Store | null {
  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

/**
 * A copy someone was sent, judged with no server: consistency is computed in
 * the page, and progress is kept in the browser's storage where it can be.
 */
export class LocalSession implements CompareSession {
  project: Project = $state()!;
  scoringAs = $state('');
  results: Record<string, { matrices: MatrixReport[] } | undefined> = $state({});

  constructor(
    project: Project,
    private store: Store | null = browserStorage(),
  ) {
    this.project = project;
    this.scoringAs = project.packet?.for ?? project.evaluators[0]?.id ?? '';
    this.recompute();
  }

  /** Resumes saved progress on this same invitation, or starts from the packet. */
  static restore(packet: Project, store: Store | null = browserStorage()): LocalSession {
    try {
      const saved = store?.getItem(key(packet));
      if (saved) {
        const p = JSON.parse(saved) as Project;
        if (p.id === packet.id && p.packet?.structure === packet.packet?.structure) return new LocalSession(p, store);
      }
    } catch {
      // unreadable or refused: start fresh
    }
    return new LocalSession(packet, store);
  }

  watch(): void {}

  changed(): void {
    this.recompute();
    try {
      this.store?.setItem(key(this.project), JSON.stringify($state.snapshot(this.project)));
    } catch {
      // storage full or refused: keep going without resuming
    }
  }

  private recompute(): void {
    const p = this.project;
    const ev = this.scoringAs;
    const matrices = [
      ...groups(p).map((g) => matrixReport(p, ev, criteriaContext(g.parentId), g.items.map((c) => c.id))),
      ...leaves(p)
        .filter((c) => (c.method ?? 'pairwise') === 'pairwise')
        .map((c) =>
          matrixReport(
            p,
            ev,
            altsContext(c.id),
            p.alternatives.map((a) => a.id),
          ),
        ),
    ];
    this.results = { [ev]: { matrices } };
  }
}

const key = (p: Project) => `ahp-judge:${p.id}:${p.packet?.for ?? ''}`;
