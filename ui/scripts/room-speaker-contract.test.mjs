import { readFileSync } from "node:fs";
import assert from "node:assert/strict";
import { test } from "node:test";

// TS-08.R99 independently bounds the palette: 32 fixed participant identities
// and a separate judge. A deleted source must never cause a palette reindex.
test("every bounded room speaker has a distinct Core role and a matching hook", () => {
  const slots = [...Array.from({ length: 32 }, (_, n) => String(n)), "judge"];
  const contract = JSON.parse(readFileSync("src/presentation/contract.json", "utf8"));
  assert.deepEqual(contract.components["think-tank"].speaker_slots, slots);
  const tokens = readFileSync("src/styles/tokens.css", "utf8");
  const css = readFileSync("src/styles/features/think-tank.css", "utf8");
  const rawColors = slots.map((slot) => {
    const role = `--ad-think-tank-speaker-${slot}`;
    assert.ok(contract.tokens.includes(role), role);
    assert.ok(tokens.includes(`${role}: var(--ad-core-think-tank-speaker-${slot})`), role);
    assert.ok(css.includes(`[data-speaker-slot="${slot}"]`), slot);
    assert.ok(css.includes(`var(${role})`), role);
    const raw = tokens.match(new RegExp(`--ad-core-think-tank-speaker-${slot}: ([^;]+);`))?.[1];
    assert.ok(raw, slot);
    return raw;
  });
  assert.equal(new Set(rawColors).size, 33);
});
