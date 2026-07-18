import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { auditPresentation } from "./check-presentation-contract.mjs";

function fixture({
  component = '<div className="known" data-ui="surface" />',
  css = ".known { color: var(--ad-public); }",
  exceptions = [],
  contract = { surface: { slots: [], states: [], variants: [] } },
} = {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "agentdeck-presentation-"));
  fs.mkdirSync(path.join(root, "src", "presentation"), { recursive: true });
  fs.mkdirSync(path.join(root, "src", "styles", "components"), { recursive: true });
  fs.writeFileSync(path.join(root, "src", "Component.tsx"), component);
  fs.writeFileSync(path.join(root, "src", "presentation", "VisualMatrix.tsx"), "export function VisualMatrix() { return null; }");
  fs.writeFileSync(path.join(root, "src", "presentation", "contract.json"), JSON.stringify({
    version: 1,
    tokens: ["--ad-public"],
    components: contract,
    decorative_slots: [],
  }));
  fs.writeFileSync(path.join(root, "presentation-exceptions.json"), JSON.stringify(exceptions));
  fs.writeFileSync(path.join(root, "src", "styles", "tokens.css"), "@layer ad-tokens { :root { --ad-public: #fff; } }");
  fs.writeFileSync(path.join(root, "src", "styles", "components", "fixture.css"), `@layer ad-components { ${css} }`);
  fs.writeFileSync(path.join(root, "src", "styles", "index.css"), `@layer ${LAYERS};\n@import \"./tokens.css\";\n@import \"./components/fixture.css\";\n@layer ad-skins {}`);
  fs.writeFileSync(path.join(root, "src", "routes.tsx"), 'const routes = import.meta.env.DEV ? import("./presentation/VisualMatrix") : [];');
  fs.writeFileSync(path.join(root, "package.json"), JSON.stringify({ scripts: {
    "check:styles": "stylelint src/**/*.css --max-warnings 0 && node --test scripts/*.test.mjs && node scripts/check-presentation-contract.mjs",
    pretest: "npm run check:styles",
    prebuild: "npm run check:styles",
  } }));
  return root;
}

const LAYERS = "ad-reset, ad-tokens, ad-base, ad-components, ad-features, ad-integrations, ad-skins";

function expectFailure(root, fragment) {
  const failures = auditPresentation(root);
  assert.ok(failures.some((failure) => failure.includes(fragment)), `Expected ${fragment}:\n${failures.join("\n")}`);
}

test("accepts a complete minimal contract", () => {
  assert.deepEqual(auditPresentation(fixture()), []);
});

test("rejects literal classes without selectors", () => {
  expectFailure(fixture({ component: '<div className="missing" data-ui="surface" />' }), 'literal class "missing"');
});

test("rejects undocumented presentation hooks", () => {
  expectFailure(fixture({ component: '<div className="known" data-ui="mystery" />' }), "undocumented data-ui mystery");
});

test("keeps slots associated with their owning component", () => {
  expectFailure(fixture({
    component: '<div className="known" data-ui="surface" data-slot="wrong" />',
    contract: { surface: { slots: ["body"], states: [], variants: [] } },
  }), "does not declare data-slot wrong");
});

test("rejects broad dynamic hook values", () => {
  expectFailure(fixture({
    component: 'export function Component({ state }: { state: string }) { return <div className="known" data-ui="surface" data-state={state} />; }',
    contract: { surface: { slots: [], states: ["open"], variants: [] } },
  }), "finite string-literal union");
});

test("rejects raw visual values outside their sources", () => {
  expectFailure(fixture({ css: ".known { color: #f00; border-radius: var(--ad-public); }" }), "raw-color");
});

test("rejects duplicate public token definitions", () => {
  expectFailure(fixture({ css: ".known { --ad-public: red; color: var(--ad-public); }" }), "exactly one core definition");
});

test("rejects undefined token references", () => {
  expectFailure(fixture({ css: ".known { color: var(--ad-missing); border-color: var(--ad-public); }" }), "undefined token --ad-missing");
});

test("requires an exact exception for dynamic native inline styles", () => {
  expectFailure(fixture({ component: 'export function Component({ x }: { x: number }) { return <div className="known" data-ui="surface" style={{ left: x }} />; }' }), "inline-style");
});

test("accepts a justified dynamic inline-style exception", () => {
  const component = 'export function Component({ x }: { x: number }) { return <div className="known" data-ui="surface" style={{ left: x }} />; }';
  assert.deepEqual(auditPresentation(fixture({
    component,
    exceptions: [{ file: "src/Component.tsx", rule: "inline-style", reason: "Left is live pointer position data" }],
  })), []);
});

test("does not let a dynamic exception hide a static visual literal", () => {
  expectFailure(fixture({
    component: 'export function Component({ x }: { x: number }) { return <div className="known" data-ui="surface" style={{ left: x, color: "red" }} />; }',
    exceptions: [{ file: "src/Component.tsx", rule: "inline-style", reason: "Left is live pointer position data" }],
  }), "inline-style-literal");
});

test("rejects stale exceptions", () => {
  expectFailure(fixture({ exceptions: [{ file: "src/Component.tsx", rule: "inline-style", reason: "No style remains in this fixture" }] }), "stale exception");
});

test("rejects unknown exception rules", () => {
  expectFailure(fixture({ exceptions: [{ file: "src/Component.tsx", rule: "ignore-everything", reason: "This rule must never be accepted" }] }), "unknown exception rule");
});

test("rejects stale manifest entries", () => {
  expectFailure(fixture({ contract: {
    surface: { slots: [], states: [], variants: [] },
    ghost: { slots: [], states: [], variants: [] },
  } }), "data-ui ghost has no implementation");
});

test("rejects public token overrides outside the skin layer", () => {
  expectFailure(fixture({ css: ".known { --ad-public: var(--ad-public); color: var(--ad-public); }" }), "override is outside ad-skins");
});

test("rejects a production skin layer with content", () => {
  const root = fixture();
  fs.appendFileSync(path.join(root, "src", "styles", "components", "fixture.css"), "\n@layer ad-skins { .known { color: var(--ad-public); } }");
  expectFailure(root, "production ad-skins layer must be empty");
});

test("requires the visual matrix development gate", () => {
  const root = fixture();
  fs.writeFileSync(path.join(root, "src", "routes.tsx"), 'import("./presentation/VisualMatrix");');
  expectFailure(root, "visual matrix must remain development-only");
});
