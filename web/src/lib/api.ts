import type { ImportReport, MergePreview, MergeReport, Project, ProjectSummary, Results, StarterEntry } from './types';

/** An error response from the API: {"error": {"code", "message", "details"}}. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly details?: unknown,
  ) {
    super(message);
  }
}

async function call<T>(method: string, path: string, body?: BodyInit, json = true): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined && json) headers['Content-Type'] = 'application/json';
  let res: Response;
  try {
    res = await fetch(path, { method, headers, body });
  } catch {
    throw new ApiError(0, 'offline', "Can't reach AHP Matrix. Is it still running?");
  }
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  const data = text ? JSON.parse(text) : undefined;
  if (!res.ok) {
    const e = data?.error ?? {};
    throw new ApiError(res.status, e.code ?? 'error', e.message ?? res.statusText, e.details);
  }
  return data as T;
}

/** Where a new project starts: blank, a copy of a sample, or a template's criteria. */
export interface Starter {
  sample?: string;
  template?: string;
}

export type MergePolicy = 'replace' | 'skip';

export interface Client {
  listProjects(): Promise<ProjectSummary[]>;
  createProject(name: string, from?: Starter): Promise<Project>;
  getProject(id: string): Promise<Project>;
  saveProject(p: Project): Promise<Project>;
  deleteProject(id: string): Promise<void>;
  duplicateProject(id: string): Promise<Project>;
  results(id: string, evaluator: string): Promise<Results>;
  /** Imports an .ahp.json file, or a workbook made with the Priority Matrix template (.xlsx). */
  importFile(file: File): Promise<{ project: Project; report?: ImportReport }>;
  /** Merges the judgments in a returned file. Without a policy, people already present cause an ApiError 'evaluator-exists'. */
  mergeFile(id: string, file: File, existing?: MergePolicy): Promise<{ project: Project; report: MergeReport }>;
  previewMerge(id: string, file: File): Promise<MergePreview>;
  samples(): Promise<StarterEntry[]>;
  templates(): Promise<StarterEntry[]>;
}

const upload = (file: File) => {
  const form = new FormData();
  form.append('file', file);
  return form;
};

export const api: Client = {
  async listProjects() {
    return (await call<{ projects: ProjectSummary[] }>('GET', '/api/projects')).projects;
  },
  createProject: (name, from = {}) =>
    call('POST', '/api/projects', JSON.stringify({ name, fromSample: from.sample, fromTemplate: from.template })),
  getProject: (id) => call('GET', `/api/projects/${encodeURIComponent(id)}`),
  saveProject: (p) => call('PUT', `/api/projects/${encodeURIComponent(p.id)}`, JSON.stringify(p)),
  deleteProject: (id) => call('DELETE', `/api/projects/${encodeURIComponent(id)}`),
  duplicateProject: (id) => call('POST', `/api/projects/${encodeURIComponent(id)}/duplicate`),
  results: (id, evaluator) =>
    call('GET', `/api/projects/${encodeURIComponent(id)}/results?evaluator=${encodeURIComponent(evaluator)}`),
  async importFile(file) {
    if (/\.xlsx$/i.test(file.name)) return call('POST', '/api/import/xlsx', upload(file), false);
    return { project: await call<Project>('POST', '/api/import/ahp', upload(file), false) };
  },
  mergeFile: (id, file, existing) =>
    call('POST', `/api/projects/${encodeURIComponent(id)}/merge${existing ? `?existing=${existing}` : ''}`, upload(file), false),
  previewMerge: (id, file) => call('POST', `/api/projects/${encodeURIComponent(id)}/merge/preview`, upload(file), false),
  samples: () => call('GET', '/api/samples'),
  templates: () => call('GET', '/api/templates'),
};

export type ExportKind = 'ahp.json' | 'xlsx' | 'csv';

/**
 * A download link. For ahp.json, an evaluator id gives that person's copy to
 * send them; for xlsx and csv it picks whose judgments the results use.
 */
export function exportUrl(id: string, kind: ExportKind = 'ahp.json', evaluator?: string): string {
  const q = evaluator ? `?evaluator=${encodeURIComponent(evaluator)}` : '';
  return `/api/projects/${encodeURIComponent(id)}/export.${kind}${q}`;
}
