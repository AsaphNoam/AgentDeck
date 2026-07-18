import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import stylelint from "stylelint";

const uiRoot = path.resolve(path.dirname(new URL(import.meta.url).pathname), "..");
const configFile = path.join(uiRoot, "stylelint.config.mjs");
const codeFilename = path.join(uiRoot, "src", "styles", "components", "__contract-test.css");

async function warningsFor(code) {
  const result = await stylelint.lint({ code, codeFilename, configFile });
  return result.results.flatMap((entry) => entry.warnings);
}

async function rejectsRule(code, rule) {
  const warnings = await warningsFor(code);
  assert.ok(warnings.some((warning) => warning.rule === rule), `Expected ${rule}:\n${warnings.map((warning) => `${warning.rule}: ${warning.text}`).join("\n")}`);
}

test("Stylelint rejects malformed CSS", async () => {
  const warnings = await warningsFor("@layer ad-components { .broken { color: red; ");
  assert.ok(warnings.length > 0);
});

test("Stylelint rejects duplicate properties", async () => {
  await rejectsRule("@layer ad-components { .known { color: red; color: blue; } }", "declaration-block-no-duplicate-properties");
});

test("Stylelint rejects duplicate selectors", async () => {
  await rejectsRule("@layer ad-components { .known { color: red; } .known { color: blue; } }", "no-duplicate-selectors");
});

test("Stylelint rejects id selectors", async () => {
  await rejectsRule("@layer ad-components { #known { color: red; } }", "selector-max-id");
});

test("Stylelint rejects excessive specificity", async () => {
  await rejectsRule("@layer ad-components { main section article div .a .b .c .d .e .f { color: red; } }", "selector-max-specificity");
});

test("Stylelint rejects important declarations", async () => {
  await rejectsRule("@layer ad-components { .known { color: red !important; } }", "declaration-no-important");
});

test("Stylelint rejects unknown cross-file custom properties", async () => {
  await rejectsRule("@layer ad-components { .known { color: var(--ad-does-not-exist); } }", "no-unknown-custom-properties");
});

test("Stylelint rejects rules outside cascade layers", async () => {
  await rejectsRule(".known { color: red; }", "rule-nesting-at-rule-required-list");
});
