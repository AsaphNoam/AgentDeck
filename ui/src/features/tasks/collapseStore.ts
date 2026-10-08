/** collapseStore is the feature-owned sessionStorage map of per-parent
 *  collapse choices (TS-08.R93). Keyed by project/task identity so the same
 *  task id in different projects never collides. Shared module state keeps
 *  every Tasks route mount in the tab in sync with the same choices; the
 *  sessionStorage mirror is what makes a refresh or a route change keep them
 *  (FS-16.R46). Storage errors fall back to in-memory-only behavior rather
 *  than breaking the page. */

const STORAGE_KEY = "chuck.tasks.collapse.v1";
const MAX_ENTRIES = 5000;

// identity -> sequence number of its last change, for least-recently-changed
// eviction on overflow.
let entries = new Map<string, number>();
let sequence = 0;
let loaded = false;
let storageOK = true;

function identity(project: string, taskID: string): string {
  return `${project}\u0000${taskID}`;
}

function split(key: string): { project: string; taskID: string } {
  const at = key.indexOf("\u0000");
  return { project: key.slice(0, at), taskID: key.slice(at + 1) };
}

function readStorage(): Record<string, number> | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    return raw ? (JSON.parse(raw) as Record<string, number>) : {};
  } catch {
    storageOK = false;
    return null;
  }
}

function load(): void {
  if (loaded) return;
  loaded = true;
  const stored = readStorage();
  if (!stored) return;
  entries = new Map(Object.entries(stored));
  sequence = entries.size === 0 ? 0 : Math.max(...entries.values()) + 1;
}

function persist(): void {
  if (!storageOK) return;
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(Object.fromEntries(entries)));
  } catch {
    storageOK = false;
  }
}

function evictOverflow(): void {
  while (entries.size > MAX_ENTRIES) {
    let oldestKey: string | null = null;
    let oldestSeq = Infinity;
    for (const [key, seq] of entries) {
      if (seq < oldestSeq) {
        oldestSeq = seq;
        oldestKey = key;
      }
    }
    if (oldestKey === null) break;
    entries.delete(oldestKey);
  }
}

export function isCollapsed(project: string, taskID: string): boolean {
  load();
  return entries.has(identity(project, taskID));
}

/** setCollapsed stores only collapsed ids; expanding simply removes the id. */
export function setCollapsed(project: string, taskID: string, collapsed: boolean): void {
  load();
  const key = identity(project, taskID);
  if (collapsed) {
    entries.set(key, sequence++);
    evictOverflow();
  } else {
    entries.delete(key);
  }
  persist();
}

/** collapsedIdsForProject snapshots the collapsed task ids for one project,
 *  for the pure rowVisibility projection to consume. */
export function collapsedIdsForProject(project: string): Set<string> {
  load();
  const out = new Set<string>();
  for (const key of entries.keys()) {
    const split_ = split(key);
    if (split_.project === project) out.add(split_.taskID);
  }
  return out;
}

/** pruneProject drops stored choices for a project's tasks that are no
 *  longer retained, but only from an authoritative, complete read of that
 *  project's tasks or its deletion — never from a partial or filtered view
 *  (TS-08.R93), so a still-loading or filtered page can never read as deletion. */
export function pruneProject(project: string, liveTaskIDs: ReadonlySet<string>): void {
  load();
  let changed = false;
  for (const key of [...entries.keys()]) {
    const split_ = split(key);
    if (split_.project === project && !liveTaskIDs.has(split_.taskID)) {
      entries.delete(key);
      changed = true;
    }
  }
  if (changed) persist();
}

/** resetCollapseStoreForTests clears all in-memory and persisted state. */
export function resetCollapseStoreForTests(): void {
  entries = new Map();
  sequence = 0;
  loaded = false;
  storageOK = true;
  try {
    sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // Storage may already be unavailable; nothing further to clean up.
  }
}
