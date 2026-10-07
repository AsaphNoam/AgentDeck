// Rendered evidence for the UI polish change (FS-02.A48-A50): drives the REAL chuck server +
// embedded UI in Chromium. Start the isolated stress fixture from the repo root after `make embed`:
//
//   go run -tags sqlite_fts5 ./scripts/stress-fixture -port 4411 -scenario activity_showcase
//
// Usage: node ui/scripts/polish-render.mjs [baseURL] [outDir]
// Writes one expanded-card/composer screenshot per skin, a role-form screenshot and report.json.
import { mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { chromium } from "playwright";

const BASE_URL = process.argv[2] || "http://127.0.0.1:4411";
const OUT_DIR = process.argv[3] || join(tmpdir(), `polish-render-${Date.now()}`);
mkdirSync(OUT_DIR, { recursive: true });
const SKINS = [["core", ""], ["sky-grove", "sky-grove"], ["studio", "studio"]];
const LONG_DRAFT = Array.from({ length: 40 }, (_, i) => `draft line ${i + 1}`).join("\n");
const TEN_LINES = Array.from({ length: 10 }, (_, i) => `prompt line ${i + 1}`).join("\n");

const checks = [];
function record(name, pass, detail) {
  checks.push({ name, pass, detail });
  console.log(`${pass ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`);
}

async function api(path, options) {
  const resp = await fetch(BASE_URL + path, { ...options, headers: { "Content-Type": "application/json" } });
  const text = await resp.text();
  if (!resp.ok) throw new Error(`${path} -> ${resp.status}: ${text}`);
  return text ? JSON.parse(text) : undefined;
}

async function main() {
  const browser = await chromium.launch().catch(() => chromium.launch({ executablePath: "/opt/pw-browsers/chromium" }));
  try {
    const session = await api("/api/sessions", {
      method: "POST",
      body: JSON.stringify({ role: "implementer", project: "stress", backend: "claude", model: "haiku", name: "Polish card", interface: "chat" }),
    });
    const agentId = session.agent.agent_id;
    await api("/api/layout", { method: "PUT", body: JSON.stringify({ order: [], density: { perRow: 2, gap: 16 }, expanded: [agentId] }) });
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await context.newPage();

    for (const [name, skin] of SKINS) {
      await api("/api/config", { method: "PUT", body: JSON.stringify({ appearance_skin: skin }) });
      await page.goto(`${BASE_URL}/project/stress`);
      const card = page.locator('[data-ui="agent-card"][data-variant="expanded"]').first();
      await card.waitFor();
      const composer = card.locator('[data-ui="composer"] textarea');
      await composer.fill(LONG_DRAFT);
      await page.waitForTimeout(150);
      const metrics = await card.evaluate((el) => {
        const header = el.querySelector('[data-slot="header"]');
        const ta = el.querySelector('[data-ui="composer"] textarea');
        const box = (node) => node?.getBoundingClientRect();
        const collapse = box(el.querySelector('[data-slot="collapse-control"]'));
        const identity = box(el.querySelector('[data-slot="identity"]'));
        const badge = box(header?.querySelector(".state-badge, [data-ui='state-badge']"));
        const context = header?.nextElementSibling;
        return {
          resize: getComputedStyle(ta).resize,
          height: ta.getBoundingClientRect().height,
          cap: window.innerHeight * 0.4,
          scrolls: ta.scrollHeight > ta.clientHeight,
          collapseRight: collapse && identity ? collapse.left > identity.right - 1 && Math.abs(collapse.top - identity.top) < 32 : false,
          badgeBeside: badge && identity ? badge.left > identity.right - 1 : false,
          contextBelow: Boolean(context?.matches('[data-slot="context"]')) && box(context).top >= box(header).bottom - 1,
          sendIcon: Boolean(el.querySelector('[data-ui="composer"] button[aria-label="Send"] svg')),
        };
      });
      record(`${name}: composer has no resize grip`, metrics.resize === "none", metrics.resize);
      record(`${name}: long composer draft caps near 40% and scrolls`, metrics.height <= metrics.cap + 2 && metrics.height > metrics.cap * 0.8 && metrics.scrolls, `${metrics.height}px of ${metrics.cap}px`);
      record(`${name}: badge and Collapse top-right of the identity row`, metrics.collapseRight && metrics.badgeBeside, JSON.stringify(metrics));
      record(`${name}: context meter on the row below the header`, metrics.contextBelow);
      record(`${name}: Send renders as an icon`, metrics.sendIcon);
      await card.screenshot({ path: join(OUT_DIR, `card-${name}.png`) });
      await composer.fill("");
    }

    await api("/api/config", { method: "PUT", body: JSON.stringify({ appearance_skin: "" }) });
    await page.goto(`${BASE_URL}/settings`);
    await page.getByRole("button", { name: "New role" }).click();
    const prompt = page.locator('textarea[name="system_prompt"]');
    await prompt.waitFor();
    const empty = await prompt.evaluate((el) => el.getBoundingClientRect().height);
    await prompt.fill(TEN_LINES);
    const grown = await prompt.evaluate((el) => ({ height: el.getBoundingClientRect().height, scrolls: el.scrollHeight > el.clientHeight, resize: getComputedStyle(el).resize }));
    record("role form: ten lines show without scrolling or a grip", !grown.scrolls && grown.height > empty && grown.resize === "none", JSON.stringify(grown));
    await page.screenshot({ path: join(OUT_DIR, "role-form.png") });
    await prompt.fill("");
    const shrunk = await prompt.evaluate((el) => el.getBoundingClientRect().height);
    record("role form: deleting lines shrinks back", Math.abs(shrunk - empty) < 1, `${shrunk}px vs ${empty}px`);
  } finally {
    await browser.close();
    writeFileSync(join(OUT_DIR, "report.json"), JSON.stringify(checks, null, 2));
    console.log(`report: ${join(OUT_DIR, "report.json")}`);
  }
  if (checks.some((check) => !check.pass)) process.exitCode = 1;
}

main().catch((err) => { console.error(err); process.exitCode = 1; });
