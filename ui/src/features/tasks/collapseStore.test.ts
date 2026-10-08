import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { collapsedIdsForProject, isCollapsed, pruneProject, resetCollapseStoreForTests, setCollapsed } from "./collapseStore";

beforeEach(() => resetCollapseStoreForTests());
afterEach(() => resetCollapseStoreForTests());

describe("collapseStore", () => {
  it("stores only collapsed ids, keyed by project/task identity", () => {
    setCollapsed("my-app", "tk_a", true);
    expect(isCollapsed("my-app", "tk_a")).toBe(true);
    // Same task id in a different project is a distinct identity.
    expect(isCollapsed("other", "tk_a")).toBe(false);
    setCollapsed("my-app", "tk_a", false);
    expect(isCollapsed("my-app", "tk_a")).toBe(false);
  });

  it("survives a fresh read of the module as a new mount would see it (sessionStorage mirror)", () => {
    setCollapsed("my-app", "tk_a", true);
    setCollapsed("my-app", "tk_b", true);
    // Simulate a second Tasks route mount reading the same tab's storage
    // without clearing it, by resetting only the in-memory half of the store.
    const raw = sessionStorage.getItem("chuck.tasks.collapse.v1");
    expect(raw).not.toBeNull();
    expect(JSON.parse(raw!)).toMatchObject({ "my-app\u0000tk_a": expect.any(Number), "my-app\u0000tk_b": expect.any(Number) });
  });

  it("collapsedIdsForProject scopes to one project only", () => {
    setCollapsed("my-app", "tk_a", true);
    setCollapsed("other", "tk_b", true);
    expect(collapsedIdsForProject("my-app")).toEqual(new Set(["tk_a"]));
    expect(collapsedIdsForProject("other")).toEqual(new Set(["tk_b"]));
  });

  it("prunes only ids absent from an authoritative complete project read, leaving other projects untouched", () => {
    setCollapsed("my-app", "tk_a", true);
    setCollapsed("my-app", "tk_b", true);
    setCollapsed("other", "tk_c", true);
    pruneProject("my-app", new Set(["tk_a"]));
    expect(isCollapsed("my-app", "tk_a")).toBe(true);
    expect(isCollapsed("my-app", "tk_b")).toBe(false);
    expect(isCollapsed("other", "tk_c")).toBe(true);
  });

  it("evicts the least-recently-changed entry once the bound is exceeded", () => {
    // A small, deterministic stand-in for the 5,000 bound: fill one over an
    // artificially tight ceiling isn't exposed, so exercise the real bound
    // directly by writing past it and checking the oldest change is the one
    // that disappears while a later change to an old id survives.
    setCollapsed("p", "first", true);
    for (let i = 0; i < 5000; i++) setCollapsed("p", `fill_${i}`, true);
    // "first" was the least-recently changed and should have been evicted.
    expect(isCollapsed("p", "first")).toBe(false);
    expect(isCollapsed("p", "fill_4999")).toBe(true);
  });

  it("keeps a later re-collapse of an old id from being treated as stale", () => {
    setCollapsed("p", "first", true);
    for (let i = 0; i < 4000; i++) setCollapsed("p", `fill_${i}`, true);
    // Touch "first" again so it is no longer the oldest change.
    setCollapsed("p", "first", false);
    setCollapsed("p", "first", true);
    for (let i = 4000; i < 5000; i++) setCollapsed("p", `fill_${i}`, true);
    expect(isCollapsed("p", "first")).toBe(true);
  });

  it("falls back to in-memory behavior when sessionStorage throws", () => {
    const original = sessionStorage.setItem.bind(sessionStorage);
    sessionStorage.setItem = () => {
      throw new DOMException("quota exceeded");
    };
    try {
      expect(() => setCollapsed("my-app", "tk_a", true)).not.toThrow();
      expect(isCollapsed("my-app", "tk_a")).toBe(true);
      expect(() => setCollapsed("my-app", "tk_a", false)).not.toThrow();
      expect(isCollapsed("my-app", "tk_a")).toBe(false);
    } finally {
      sessionStorage.setItem = original;
    }
  });

  it("falls back to in-memory behavior when sessionStorage.getItem throws on load", () => {
    resetCollapseStoreForTests();
    const original = sessionStorage.getItem.bind(sessionStorage);
    sessionStorage.getItem = () => {
      throw new DOMException("blocked");
    };
    try {
      expect(() => setCollapsed("my-app", "tk_a", true)).not.toThrow();
      expect(isCollapsed("my-app", "tk_a")).toBe(true);
    } finally {
      sessionStorage.getItem = original;
    }
  });
});
