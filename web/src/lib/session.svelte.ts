import { ApiError, api, type Client } from './api';
import { explain, type Problem } from './problems';
import { ALL_EVALUATORS, type Project, type Results } from './types';

export type SaveState = 'saved' | 'pending' | 'saving' | 'conflict' | 'error';

/**
 * One open project. Edits change `project` in place and call `changed()`;
 * the session saves 600 ms after the last edit, keeps the local copy as the
 * source of truth (only the version comes back from the server), and fetches
 * fresh results after each save.
 */
export class ProjectSession {
  project: Project = $state()!;
  saveState: SaveState = $state('saved');
  error = $state('');
  /** Evaluator whose judgments the compare screens edit. */
  scoringAs = $state('');
  /** Latest results, keyed by evaluator id or ALL_EVALUATORS. */
  results: Record<string, Results> = $state({});

  private timer: ReturnType<typeof setTimeout> | undefined;
  private inFlight: Promise<void> | null = null;
  private watched = new Set<string>([ALL_EVALUATORS]);

  constructor(
    project: Project,
    private client: Client = api,
    private delayMs = 600,
  ) {
    this.project = project;
    this.scoringAs = project.evaluators[0]?.id ?? '';
  }

  static async open(id: string, client: Client = api): Promise<ProjectSession> {
    const s = new ProjectSession(await client.getProject(id), client);
    await s.refreshResults();
    return s;
  }

  /** Call after every edit to project. */
  changed(): void {
    if (this.saveState === 'conflict') return; // wait for the person to choose
    this.saveState = 'pending';
    clearTimeout(this.timer);
    this.timer = setTimeout(() => void this.flush(), this.delayMs);
  }

  /** Saves now if there are unsaved edits; resolves when saving is done. */
  async flush(): Promise<void> {
    clearTimeout(this.timer);
    if (this.inFlight) await this.inFlight;
    if (this.saveState !== 'pending') return;
    this.inFlight = this.save();
    try {
      await this.inFlight;
    } finally {
      this.inFlight = null;
    }
  }

  private async save(): Promise<void> {
    this.saveState = 'saving';
    const sent = $state.snapshot(this.project) as Project;
    try {
      const saved = await this.client.saveProject(sent);
      this.project.version = saved.version;
      this.project.updatedAt = saved.updatedAt;
      // Edits made while saving left the state 'pending'; save those too.
      if (this.saveState === 'saving') this.saveState = 'saved';
      else this.timer = setTimeout(() => void this.flush(), this.delayMs);
      this.error = '';
      await this.refreshResults();
    } catch (e) {
      if (e instanceof ApiError && e.code === 'conflict') {
        this.saveState = 'conflict';
        this.error = 'This project was changed in another window.';
      } else if (e instanceof ApiError && e.code === 'invalid' && Array.isArray(e.details)) {
        this.saveState = 'error';
        this.error = `${explain(e.details as Problem[], this.project).join(' ')} Your changes will save as soon as that’s fixed.`;
      } else {
        this.saveState = 'error';
        this.error = e instanceof Error ? e.message : String(e);
      }
    }
  }

  /** Also keep results for this evaluator fresh after each save. */
  watch(evaluator: string): void {
    if (this.watched.has(evaluator)) return;
    this.watched.add(evaluator);
    void this.refreshResults([evaluator]);
  }

  async refreshResults(evaluators: string[] = [...this.watched]): Promise<void> {
    const known = new Set([ALL_EVALUATORS, ...this.project.evaluators.map((e) => e.id)]);
    await Promise.all(
      evaluators
        .filter((ev) => known.has(ev))
        .map(async (ev) => {
          try {
            this.results[ev] = await this.client.results(this.project.id, ev);
          } catch {
            // Results are advisory; the next save tries again.
          }
        }),
    );
  }

  /** Conflict resolution: discard local edits and load the stored project. */
  async reload(): Promise<void> {
    clearTimeout(this.timer);
    this.project = await this.client.getProject(this.project.id);
    if (!this.project.evaluators.some((e) => e.id === this.scoringAs)) this.scoringAs = this.project.evaluators[0]?.id ?? '';
    this.saveState = 'saved';
    this.error = '';
    await this.refreshResults();
  }

  /** Conflict resolution: keep local edits and replace the stored project. */
  async overwrite(): Promise<void> {
    const stored = await this.client.getProject(this.project.id);
    this.project.version = stored.version;
    this.saveState = 'pending';
    await this.flush();
  }

  /** Retry after an error. */
  retry(): Promise<void> {
    this.saveState = 'pending';
    return this.flush();
  }
}
