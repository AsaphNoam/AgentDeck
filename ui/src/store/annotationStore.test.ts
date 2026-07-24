import { beforeEach, describe, expect, it } from "vitest";
import { useAnnotationStore } from "./annotationStore";

beforeEach(() => {
  localStorage.clear();
  useAnnotationStore.setState({ bySource: {}, overallBySource: {} });
});

describe("annotationStore", () => {
  it("keeps a source-scoped editable tray and clears it only on discard", () => {
    const store = useAnnotationStore.getState();
    expect(store.add("a_source", { seq: 7, excerpt: "selected line", instruction: "check this" })).toBe(true);
    store.updateInstruction("a_source", 0, "test this boundary");
    store.setOverall("a_source", "Send a focused review");
    expect(useAnnotationStore.getState().bySource.a_source[0].instruction).toBe("test this boundary");
    expect(useAnnotationStore.getState().overallBySource.a_source).toBe("Send a focused review");
    store.discard("a_source");
    expect(useAnnotationStore.getState().bySource.a_source).toBeUndefined();
    expect(useAnnotationStore.getState().overallBySource.a_source).toBeUndefined();
  });
});
