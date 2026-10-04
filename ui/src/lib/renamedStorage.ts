// Browser-local keys renamed with the product (TS-08.R58). Each pre-rename key
// is copied to its new name only when the new name is still empty, then
// removed, so unsent drafts and trays survive and newer state is never
// overwritten. It runs on every load; a second run finds nothing to move.
const renamedKeys: Array<[from: string, to: string]> = [
  ["agentdeck-chat-drafts", "chuck-chat-drafts"],
  ["agentdeck-annotation-tray", "chuck-annotation-tray"],
  ["agentdeck.pipeline-builder-agent", "chuck.pipeline-builder-agent"],
];

export function copyForwardRenamedStorage(storage: Storage = window.localStorage) {
  for (const [from, to] of renamedKeys) {
    try {
      const old = storage.getItem(from);
      if (old === null) continue;
      if (storage.getItem(to) === null) storage.setItem(to, old);
      storage.removeItem(from);
    } catch {
      // Unavailable or full storage must not block the app; the old key stays
      // for the next load to retry.
    }
  }
}

// Imported first by the desktop entry so the copy lands before any persisted
// store hydrates from its new key.
try {
  copyForwardRenamedStorage();
} catch {
  // localStorage itself can be inaccessible (privacy modes).
}
