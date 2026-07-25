import { beforeEach, describe, expect, it } from "vitest";
import { pruneTrays, useAnnotationStore } from "./annotationStore";

beforeEach(() => {
  localStorage.clear();
  useAnnotationStore.setState({ bySource: {}, overallBySource: {}, editedAt: {} });
});

describe("annotationStore", () => {
  it("keeps a source-scoped editable tray and clears it only on discard", () => {
    const store = useAnnotationStore.getState();
    expect(store.add("a_source", { seq: 7, excerpt: "selected line", instruction: "check this" })).toBe(true);
    store.updateInstruction("a_source", 0, "test this boundary");
    store.setOverall("a_source", "Send a focused review");
    expect(useAnnotationStore.getState().bySource.a_source[0].instruction).toBe("test this boundary");
    expect(useAnnotationStore.getState().overallBySource.a_source).toBe("Send a focused review");
    expect(useAnnotationStore.getState().editedAt.a_source).toBeGreaterThan(0);
    store.discard("a_source");
    expect(useAnnotationStore.getState().bySource.a_source).toBeUndefined();
    expect(useAnnotationStore.getState().overallBySource.a_source).toBeUndefined();
    expect(useAnnotationStore.getState().editedAt.a_source).toBeUndefined();
  });

  // FS-13.R16: nothing on the server owns a tray, so the stored set is bounded
  // on reload instead of growing for every agent the browser ever opened.
  it("expires and caps stored trays when they are rehydrated", () => {
    const now = Date.UTC(2026, 6, 25);
    const day = 24 * 60 * 60 * 1000;
    const draft = [{ seq: 1, excerpt: "line", instruction: "look" }];
    const bySource: Record<string, typeof draft> = { a_old: draft, a_empty: [], a_untimed: draft };
    const editedAt: Record<string, number> = { a_old: now - 31 * day, a_empty: now };
    for (let i = 0; i < 25; i++) {
      bySource[`a_${i}`] = draft;
      editedAt[`a_${i}`] = now - i * 1000;
    }

    const pruned = pruneTrays({ bySource, overallBySource: { a_old: "stale overall" }, editedAt }, now);

    expect(Object.keys(pruned.bySource)).toHaveLength(20);
    expect(pruned.bySource.a_old).toBeUndefined();
    expect(pruned.overallBySource.a_old).toBeUndefined();
    expect(pruned.bySource.a_empty).toBeUndefined();
    // A tray stored before trays carried a timestamp counts as touched now, so
    // upgrading the app never throws away a draft in progress.
    expect(pruned.bySource.a_untimed).toEqual(draft);
    expect(pruned.editedAt.a_untimed).toBe(now);
    // The most recently edited sources are the ones kept.
    expect(pruned.bySource.a_0).toEqual(draft);
    expect(pruned.bySource.a_24).toBeUndefined();
  });
});
