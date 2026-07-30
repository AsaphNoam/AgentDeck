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
  skins = [],
  skinCss = {},
} = {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "agentdeck-presentation-"));
  fs.mkdirSync(path.join(root, "src", "presentation"), { recursive: true });
  fs.mkdirSync(path.join(root, "src", "styles", "components"), { recursive: true });
  fs.mkdirSync(path.join(root, "src", "styles", "skins"), { recursive: true });
  const skinComponents = skins.length ? {
    ...contract,
    "config-editor": { slots: [], states: [], variants: ["appearance"] },
  } : contract;
  const skinMarkup = skins.length
    ? `<div data-ui="config-editor" data-variant="appearance">${skins.map((skin) => `<div data-preview-skin="${skin}" />`).join("")}<div data-preview-skin="core" /></div>`
    : "";
  const allowlist = skins.length ? `export const BUILT_IN_SKINS = ${JSON.stringify(skins)} as const;` : "";
  fs.writeFileSync(path.join(root, "src", "Component.tsx"), `${allowlist}${component}${skinMarkup}`);
  fs.writeFileSync(path.join(root, "src", "presentation", "VisualMatrix.tsx"), "export function VisualMatrix() { return null; }");
  fs.writeFileSync(path.join(root, "src", "presentation", "contract.json"), JSON.stringify({
    version: 2,
    skins,
    tokens: ["--ad-public"],
    components: skinComponents,
    decorative_slots: [],
  }));
  fs.writeFileSync(path.join(root, "presentation-exceptions.json"), JSON.stringify(exceptions));
  fs.writeFileSync(path.join(root, "src", "styles", "tokens.css"), "@layer ad-tokens { :root { --ad-public: #fff; } }");
  fs.writeFileSync(path.join(root, "src", "styles", "components", "fixture.css"), `@layer ad-components { ${css} }`);
  for (const skin of skins) {
    const source = skinCss[skin] ?? `@layer ad-skins {
      :root { --ad-${skin}-canvas: #def; }
      :root[data-skin="${skin}"] { --ad-public: var(--ad-${skin}-canvas); }
      [data-ui="config-editor"][data-variant="appearance"] [data-preview-skin="${skin}"] { color: var(--ad-${skin}-canvas); }
    }`;
    fs.writeFileSync(path.join(root, "src", "styles", "skins", `${skin}.css`), source);
  }
  const skinImports = skins.map((skin) => `@import "./skins/${skin}.css";`).join("\n");
  fs.writeFileSync(path.join(root, "src", "styles", "index.css"), `@layer ${LAYERS};\n@import \"./tokens.css\";\n@import \"./components/fixture.css\";\n${skinImports}`);
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

test("accepts a declared, statically imported, scoped built-in skin", () => {
  assert.deepEqual(auditPresentation(fixture({ skins: ["sky-grove"] })), []);
});

test("rejects literal classes without selectors", () => {
  expectFailure(fixture({ component: '<div className="missing" data-ui="surface" />' }), 'literal class "missing"');
});

test("rejects browser-native prompt and confirm calls in first-party UI", () => {
  expectFailure(fixture({ component: 'window.prompt("name"); export function Component() { return <div className="known" data-ui="surface" />; }' }), "native-dialog");
  expectFailure(fixture({ component: 'confirm("delete"); export function Component() { return <div className="known" data-ui="surface" />; }' }), "native-dialog");
  expectFailure(fixture({ component: 'window["confirm"]("delete"); export function Component() { return <div className="known" data-ui="surface" />; }' }), "native-dialog");
  expectFailure(fixture({ component: 'globalThis.confirm("delete"); export function Component() { return <div className="known" data-ui="surface" />; }' }), "native-dialog");
  expectFailure(fixture({ component: 'const ask = window.prompt; ask("name"); export function Component() { return <div className="known" data-ui="surface" />; }' }), "native-dialog");
  expectFailure(fixture({ component: 'const { confirm: ask } = globalThis; ask("delete"); export function Component() { return <div className="known" data-ui="surface" />; }' }), "native-dialog");
});

test("permits a documented native-dialog exception", () => {
  assert.deepEqual(auditPresentation(fixture({
    component: 'window.prompt("fixture only"); export function Component() { return <div className="known" data-ui="surface" />; }',
    exceptions: [{ file: "src/Component.tsx", rule: "native-dialog", reason: "This fixture deliberately verifies the source guard." }],
  })), []);
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
  const root = fixture();
  fs.appendFileSync(path.join(root, "src", "styles", "tokens.css"), "\n@layer ad-tokens { :root { --ad-public: #000; } }");
  expectFailure(root, "exactly one Core definition");
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
  expectFailure(fixture({ css: ".known { --ad-public: var(--ad-public); color: var(--ad-public); }" }), "override is outside a declared ad-skins stylesheet");
});

test("rejects a production skin layer with content", () => {
  const root = fixture();
  fs.appendFileSync(path.join(root, "src", "styles", "components", "fixture.css"), "\n@layer ad-skins { .known { color: var(--ad-public); } }");
  expectFailure(root, "production ad-skins rules belong only in a declared skin stylesheet");
});

test("rejects unknown and unscoped production skin CSS", () => {
  const unknown = fixture();
  fs.writeFileSync(path.join(unknown, "src", "styles", "skins", "mystery.css"), '@layer ad-skins { :root[data-skin="mystery"] { color: red; } }');
  expectFailure(unknown, "skin stylesheet mystery is not declared");

  const unscoped = fixture({
    skins: ["sky-grove"],
    skinCss: { "sky-grove": '@layer ad-skins { :root { --ad-sky-grove-canvas: #def; } .known { color: var(--ad-sky-grove-canvas); } }' },
  });
  expectFailure(unscoped, "is not scoped to sky-grove");
});

test("rejects undocumented skin ids and incomplete Settings preview scopes", () => {
  const wrongRoot = fixture({
    skins: ["sky-grove"],
    skinCss: { "sky-grove": '@layer ad-skins { :root { --ad-sky-grove-canvas: #def; } :root[data-skin="mystery"] { --ad-public: var(--ad-sky-grove-canvas); } [data-preview-skin="sky-grove"] { color: var(--ad-sky-grove-canvas); } }' },
  });
  expectFailure(wrongRoot, 'must use :root[data-skin="sky-grove"]');
  expectFailure(wrongRoot, "must be scoped to the sky-grove Settings appearance preview");
});

test("keeps private skin tokens in their matching file and rejects stale palette values", () => {
  const outside = fixture({ css: ".known { color: var(--ad-sky-grove-canvas); }", skins: ["sky-grove"] });
  expectFailure(outside, "may be used only in sky-grove.css");

  const unused = fixture({
    skins: ["sky-grove"],
    skinCss: { "sky-grove": `@layer ad-skins {
      :root { --ad-sky-grove-canvas: #def; --ad-sky-grove-unused: #abc; }
      :root[data-skin="sky-grove"] { --ad-public: var(--ad-sky-grove-canvas); }
      [data-ui="config-editor"][data-variant="appearance"] [data-preview-skin="sky-grove"] { color: var(--ad-sky-grove-canvas); }
    }` },
  });
  expectFailure(unused, "--ad-sky-grove-unused is private but unused");
});

test("requires manifest skins, frontend allowlist, files, and imports to match", () => {
  const noImport = fixture({ skins: ["sky-grove"] });
  fs.writeFileSync(path.join(noImport, "src", "styles", "index.css"), `@layer ${LAYERS};\n@import "./tokens.css";\n@import "./components/fixture.css";`);
  expectFailure(noImport, "must be statically imported exactly once");

  const allowlist = fixture({ skins: ["sky-grove"] });
  const componentPath = path.join(allowlist, "src", "Component.tsx");
  fs.writeFileSync(componentPath, fs.readFileSync(componentPath, "utf8").replace('["sky-grove"] as const', '["mystery"] as const'));
  expectFailure(allowlist, "BUILT_IN_SKINS must exactly match");
});

test("rejects network-loaded CSS assets and fixture leakage", () => {
  const network = fixture({ skins: ["sky-grove"] });
  fs.appendFileSync(path.join(network, "src", "styles", "skins", "sky-grove.css"), '\n@import "https://example.com/theme.css";');
  expectFailure(network, "network stylesheet import");

  const leakedFixture = fixture();
  fs.writeFileSync(path.join(leakedFixture, "src", "presentation", "Leak.tsx"), 'import "./contract-fixture.css";');
  expectFailure(leakedFixture, "contract fixture may be imported only by VisualMatrix");
});

test("requires the visual matrix development gate", () => {
  const root = fixture();
  fs.writeFileSync(path.join(root, "src", "routes.tsx"), 'import("./presentation/VisualMatrix");');
  expectFailure(root, "visual matrix must remain development-only");
});
