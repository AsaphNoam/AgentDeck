const storageKey = "agentdeck-chat-drafts";
const maxDrafts = 20;

type Draft = { text: string; editedAt: number };
type StoredDrafts = Record<string, Draft>;

export function getChatDraft(agentId: string): string {
  const drafts = readDrafts();
  return drafts[agentId]?.text ?? "";
}

export function setChatDraft(agentId: string, text: string) {
  const drafts = readDrafts();
  if (text === "") {
    delete drafts[agentId];
  } else {
    const newest = Math.max(Date.now(), ...Object.values(drafts).map((draft) => draft.editedAt + 1));
    drafts[agentId] = { text, editedAt: newest };
  }
  writeDrafts(drafts);
}

export function discardChatDraft(agentId: string) {
  const drafts = readDrafts();
  delete drafts[agentId];
  writeDrafts(drafts);
}

function readDrafts(): StoredDrafts {
  try {
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) return {};
    const parsed: unknown = JSON.parse(raw);
    if (!isStoredDrafts(parsed)) return {};
    const drafts = pruneDrafts(parsed.drafts);
    // Rehydration enforces the same retention bound as writes.
    if (Object.keys(drafts).length !== Object.keys(parsed.drafts).length) writeDrafts(drafts);
    return drafts;
  } catch {
    return {};
  }
}

function writeDrafts(drafts: StoredDrafts) {
  try {
    window.localStorage.setItem(storageKey, JSON.stringify({ drafts: pruneDrafts(drafts) }));
  } catch {
    // Browser storage is optional: persistence failure must never block typing or Send.
  }
}

function pruneDrafts(drafts: StoredDrafts): StoredDrafts {
  return Object.fromEntries(
    Object.entries(drafts)
      .sort(([leftId, left], [rightId, right]) => right.editedAt - left.editedAt || leftId.localeCompare(rightId))
      .slice(0, maxDrafts),
  );
}

function isStoredDrafts(value: unknown): value is { drafts: StoredDrafts } {
  if (!value || typeof value !== "object" || !("drafts" in value)) return false;
  const drafts = value.drafts;
  return !!drafts && typeof drafts === "object" && Object.values(drafts).every(isDraft);
}

function isDraft(value: unknown): value is Draft {
  return !!value && typeof value === "object" &&
    "text" in value && typeof value.text === "string" && value.text !== "" &&
    "editedAt" in value && typeof value.editedAt === "number" && Number.isFinite(value.editedAt);
}
