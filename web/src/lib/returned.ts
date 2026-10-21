import type { Client } from './api';
import { count } from './format';
import type { MergeReport, PacketInfo, Project, ProjectSummary } from './types';

/** What a decision file says about where it came from. */
export interface FileInfo {
  id: string;
  origin?: string;
  name: string;
  packet?: PacketInfo;
}

/** Reads just enough of a .ahp.json file to tell where it came from; null if it isn't one. */
export async function peek(file: File): Promise<FileInfo | null> {
  if (/\.xlsx$/i.test(file.name)) return null;
  try {
    const raw: unknown = JSON.parse(await file.text());
    if (!raw || typeof raw !== 'object') return null;
    const { id, origin, name, packet } = raw as Record<string, unknown>;
    if (typeof id !== 'string') return null;
    const info: FileInfo = { id, name: typeof name === 'string' ? name : '' };
    if (typeof origin === 'string' && origin) info.origin = origin;
    if (packet && typeof packet === 'object') info.packet = packet as PacketInfo;
    return info;
  } catch {
    return null;
  }
}

/**
 * The decision in the library that a returned copy belongs to: a copy that
 * was sent to someone, matching a decision of yours by id or by the id it was
 * re-imported from. Never a copy you were sent yourself.
 */
export function mergeTarget(info: FileInfo, library: ProjectSummary[]): ProjectSummary | null {
  if (!info.packet) return null;
  return library.find((s) => !s.error && !s.received && (s.id === info.id || (info.origin && s.id === info.origin))) ?? null;
}

/** One line on what a merge did. */
export function mergedText(report: MergeReport): string {
  const who = [...report.added, ...report.replaced];
  if (who.length === 0) return 'Nothing merged: the current answers were kept.';
  const left = report.dropped.length;
  return (
    `Merged ${count(report.judgments, 'judgment')} from ${list(who)}.` +
    (left ? ` ${count(left, 'answer')} about things no longer in this decision ${left === 1 ? 'was' : 'were'} left out.` : '')
  );
}

/** "Priya", "Priya and Bob", "Priya, Bob and Ann". */
export function list(names: string[]): string {
  return names.length <= 1 ? (names[0] ?? '') : `${names.slice(0, -1).join(', ')} and ${names[names.length - 1]}`;
}

/** Puts back the project as it was before a merge; version is the merged project's. */
export function undoMerge(client: Pick<Client, 'saveProject'>, before: Project, version: number): Promise<Project> {
  return client.saveProject({ ...before, version });
}
