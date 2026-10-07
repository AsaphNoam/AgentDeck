// Readable labels for configured entities (FS-02.R69, TS-08.R91). A label is the readable name
// alone; the id is appended only when another entry in the same list shares that name. Option
// values, keys and stored data stay keyed by id.
export type LabelEntry = readonly [id: string, name: string];

export function displayLabel(entries: readonly LabelEntry[], id: string): string {
  const entry = entries.find(([entryId]) => entryId === id);
  if (!entry) return "";
  const duplicates = entries.filter(([, name]) => name === entry[1]).length > 1;
  return duplicates ? `${entry[1]} (${entry[0]})` : entry[1];
}

/** `[id, label]` for every entry, in order, for rendering a select's options. */
export function displayLabels(entries: readonly LabelEntry[]): [string, string][] {
  return entries.map(([id]) => [id, displayLabel(entries, id)]);
}
