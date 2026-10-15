import type { Project, ProjectSummary, Results } from './types';

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

export interface Client {
  listProjects(): Promise<ProjectSummary[]>;
  createProject(name: string): Promise<Project>;
  getProject(id: string): Promise<Project>;
  saveProject(p: Project): Promise<Project>;
  deleteProject(id: string): Promise<void>;
  duplicateProject(id: string): Promise<Project>;
  results(id: string, evaluator: string): Promise<Results>;
  importFile(file: File): Promise<Project>;
}

export const api: Client = {
  async listProjects() {
    return (await call<{ projects: ProjectSummary[] }>('GET', '/api/projects')).projects;
  },
  createProject: (name) => call('POST', '/api/projects', JSON.stringify({ name })),
  getProject: (id) => call('GET', `/api/projects/${encodeURIComponent(id)}`),
  saveProject: (p) => call('PUT', `/api/projects/${encodeURIComponent(p.id)}`, JSON.stringify(p)),
  deleteProject: (id) => call('DELETE', `/api/projects/${encodeURIComponent(id)}`),
  duplicateProject: (id) => call('POST', `/api/projects/${encodeURIComponent(id)}/duplicate`),
  results: (id, evaluator) =>
    call('GET', `/api/projects/${encodeURIComponent(id)}/results?evaluator=${encodeURIComponent(evaluator)}`),
  importFile(file) {
    const form = new FormData();
    form.append('file', file);
    return call('POST', '/api/import/ahp', form, false);
  },
};

export const exportUrl = (id: string) => `/api/projects/${encodeURIComponent(id)}/export.ahp.json`;
