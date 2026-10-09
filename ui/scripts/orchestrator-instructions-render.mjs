// Rendered evidence for the shared orchestrator instructions field on the
// pipeline template editor (optional `orchestrator_instructions` string beside
// "Standing orchestrator role" in `.pipeline-editor-basics`). Drives Vite + the
// real UI source in Chromium with every /api call and the event stream stubbed
// (same pattern as scripts/think-tank-stage-render.mjs), in Core, Sky & Grove
// and Studio at 1440px, plus one 1024px editor screenshot.
//
//   node scripts/orchestrator-instructions-render.mjs [outDir]
//
// Writes screenshots and report.json to outDir. Exits 1 if any check fails.
import { mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createServer } from "vite";
import { chromium } from "playwright";

const OUT_DIR = process.argv[2] || join(tmpdir(), `orchestrator-instructions-render-${Date.now()}`);
mkdirSync(OUT_DIR, { recursive: true });
const PORT = 5201;
const BASE = `http://localhost:${PORT}`;
const AT = "2026-10-09T09:00:00Z";
const SKINS = ["", "sky-grove", "studio"];
const skinLabel = (skin) => skin || "core";

const TEMPLATE_ID = "release-note";
const VALID_TEXT = "Name branches pipeline-STAGE.\nGroup agents by stage.";
const TOO_LONG_TEXT = "x".repeat(16001);

const checks = [];
function record(name, pass, detail) {
  checks.push({ name, pass: Boolean(pass), detail: detail === undefined ? undefined : String(detail) });
  console.log(`${pass ? "PASS" : "FAIL"} ${name}${detail !== undefined ? ` — ${detail}` : ""}`);
}

// ---- Fixtures --------------------------------------------------------------

function baseTemplate() {
  return {
    version: 2,
    title: "Release note",
    orchestrator_role: "implementer",
    inputs: [],
    stages: [
      { id: "draft", title: "Draft note", objective: "Draft the release note.", coordination: "standing", dedicated_role: "", approval_after_success: false, inputs: [], outputs: [] },
      { id: "publish", title: "Publish note", objective: "Publish the release note.", coordination: "standing", dedicated_role: "", approval_after_success: false, inputs: [], outputs: [] },
    ],
  };
}

// Mutable per-process store, reset per browser context so each journey starts
// from the same saved template.
let TEMPLATES;
let PUTS;
let STARTS;
let UNHANDLED;
function resetStore() {
  TEMPLATES = { [TEMPLATE_ID]: { id: TEMPLATE_ID, template: baseTemplate(), valid: true, diagnostics: [] } };
  PUTS = [];
  STARTS = [];
  UNHANDLED = [];
}

function runDetail(runID) {
  const template = TEMPLATES[TEMPLATE_ID].template;
  const slot = { backend: "codex", model: "gpt", effort: "", fast: false };
  const owner = { agent_id: "a_owner", name: "Owner", state: "idle", route: "live", runtime: slot };
  const stageTask = (stage, index) => ({
    task_id: `tk_${stage.id}`, run_id: runID, stage_id: stage.id, stage_index: index, attempt_number: 1,
    state: index === 0 ? "running" : "pending", assignment_text: "", execution_kind: "agent",
    standing_owner: owner, work: [], created_at: AT, updated_at: AT,
  });
  return {
    run: {
      run_id: runID, template_id: TEMPLATE_ID, template_snapshot: template, display_name: "Ship the note",
      project: "demo", goal: "Ship the note", inputs: {}, orchestrator: slot, dedicated_assignments: {},
      state: "running", revision: 1, pending_action: "", current_stage_id: template.stages[0].id,
      current_task_id: `tk_${template.stages[0].id}`, orchestrator_agent_id: "a_owner", current_attempt_id: "",
      current_agent_id: "a_owner", attention_reason: "", final_outcome: "", created_at: AT, updated_at: AT,
    },
    template, inputs: {}, orchestrator: slot, dedicated_assignments: {}, assignments: { standing: slot },
    think_tank_assignments: {}, stage_tasks: template.stages.map(stageTask), values: [], diagnostics: [],
    controls: {
      continue: { eligible: false, reason: "" }, retry: { eligible: false, reason: "" },
      replace: { eligible: false, reason: "" }, stop: { eligible: true, reason: "" }, repair_cleanup: { eligible: false, reason: "" },
    },
  };
}

function api(skin) {
  return async (route) => {
    const req = route.request();
    const url = new URL(req.url());
    const path = url.pathname;
    if (path === "/api/config") return route.fulfill({ json: { appearance_skin: skin, onboarding_complete: true, default_project: "demo" } });
    if (path === "/api/backends") return route.fulfill({ json: { version: 2, backends: { codex: { name: "Codex", type: "codex-acp", default: true, default_model: "gpt", models: { gpt: { name: "GPT", model: "gpt", fast: false } } } } } });
    if (path === "/api/projects") return route.fulfill({ json: { demo: { title: "Demo", cwd: "/tmp/demo", color: [90, 130, 210], add_dirs: [], context_prompt: "", archived: false } } });
    if (path === "/api/roles") return route.fulfill({ json: { implementer: { title: "Implementer", system_prompt: "", skip_permissions: false } } });
    if (path === "/api/layout") return route.fulfill({ json: { order: [], density: { perRow: 2, gap: 16 } } });
    if (path === "/api/pipeline-proposals") return route.fulfill({ json: { pending: [], declined: [] } });
    if (path === "/api/pipelines/validate") return route.fulfill({ json: TEMPLATES[TEMPLATE_ID] });
    if (path === "/api/pipelines" && req.method() === "GET") return route.fulfill({ json: Object.values(TEMPLATES) });
    const putMatch = path.match(/^\/api\/pipelines\/([^/]+)$/);
    if (putMatch && req.method() === "PUT") {
      const id = decodeURIComponent(putMatch[1]);
      const body = JSON.parse(req.postData() || "{}");
      PUTS.push({ id, body });
      const tooLong = typeof body.orchestrator_instructions === "string" && body.orchestrator_instructions.length > 16000;
      if (tooLong) {
        return route.fulfill({
          status: 422,
          json: { error: { code: "validation", message: "Template is invalid.", details: { diagnostics: [
            { field: "orchestrator_instructions", code: "too_long", message: "Shared orchestrator instructions must be at most 16000 characters." },
          ] } } },
        });
      }
      TEMPLATES[id] = { id, template: body, valid: true, diagnostics: [] };
      return route.fulfill({ json: TEMPLATES[id] });
    }
    if (path === "/api/pipeline-runs" && req.method() === "GET") return route.fulfill({ json: [], headers: { "X-Total-Count": "0" } });
    if (path === "/api/pipeline-runs" && req.method() === "POST") {
      const body = JSON.parse(req.postData() || "{}");
      STARTS.push(body);
      return route.fulfill({ status: 201, json: { run: runDetail("pr_release"), replay: false, workspace_conflicts: [] } });
    }
    const runMatch = path.match(/^\/api\/pipeline-runs\/([^/]+)$/);
    if (runMatch && runMatch[1] === "pr_release") return route.fulfill({ json: runDetail("pr_release") });
    if (path === "/api/tasks") return route.fulfill({ json: { tasks: [] } });
    if (path === "/api/think-tanks") return route.fulfill({ json: { version: 1, rooms: [], clipped: false } });
    if (path === "/api/config-sources") return route.fulfill({ json: [] });
    UNHANDLED.push(`${req.method()} ${path}`);
    return route.fulfill({ status: 599, json: { error: { code: "unhandled_stub", path } } });
  };
}

async function newPage(browser, skin, width) {
  const context = await browser.newContext({ viewport: { width, height: 1000 } });
  await context.addInitScript(() => {
    delete window.SharedWorker;
    window.EventSource = class extends EventTarget {
      constructor() { super(); setTimeout(() => { this.onopen?.(); this.dispatchEvent(new MessageEvent("state_update", { data: JSON.stringify({ agent_id: "__hydrated__", data: { hydrated: true } }) })); }); }
      close() {}
    };
  });
  await context.route((u) => u.pathname.startsWith("/api/"), api(skin));
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (err) => errors.push(err.message));
  return { context, page, errors };
}

// ---- Journeys ---------------------------------------------------------------

async function editorJourney(browser, skin, width, { shootEditor = true } = {}) {
  const label = `${skinLabel(skin)}@${width}`;
  resetStore();
  const { context, page, errors } = await newPage(browser, skin, width);

  // 1. Open the existing template's editor; type multi-line guidance.
  await page.goto(`${BASE}/pipelines/templates/${TEMPLATE_ID}`);
  const textarea = page.getByLabel("Shared orchestrator instructions");
  const shown = await textarea.waitFor({ timeout: 8000 }).then(() => true, () => false);
  record(`${label}: editor renders the shared orchestrator instructions field`, shown);
  if (!shown) { record(`${label}: no page errors`, errors.length === 0, errors.join(" | ")); await context.close(); return; }

  await textarea.fill(VALID_TEXT);
  record(`${label}: field holds the typed multi-line text`, (await textarea.inputValue()) === VALID_TEXT);

  const panel = page.locator(".pipeline-editor-basics");
  const fieldBox = await textarea.boundingBox();
  const panelBox = await panel.boundingBox();
  record(`${label}: field visible within the viewport`, Boolean(fieldBox) && fieldBox.width > 0 && fieldBox.height > 0);
  record(`${label}: field does not overflow the basics panel horizontally`,
    Boolean(fieldBox && panelBox) && fieldBox.x >= panelBox.x - 1 && fieldBox.x + fieldBox.width <= panelBox.x + panelBox.width + 1,
    JSON.stringify({ fieldBox, panelBox }));

  const roleSelect = page.getByLabel("Standing orchestrator role");
  const roleBox = await roleSelect.boundingBox();
  const sameRow = Boolean(roleBox && fieldBox) && roleBox.y < fieldBox.y + fieldBox.height && fieldBox.y < roleBox.y + roleBox.height;
  record(`${label}: instructions field sits adjacent to the role select (same basics row)`, sameRow, JSON.stringify({ roleBox, fieldBox }));

  if (shootEditor) await page.screenshot({ path: join(OUT_DIR, `editor-${label}.png`), fullPage: true });

  // 2. Save; check the PUT body carries the exact text and no stage objective holds it.
  const beforePuts = PUTS.length;
  await page.getByRole("button", { name: "Save template" }).click();
  await page.waitForTimeout(500);
  const firstPut = PUTS[beforePuts];
  record(`${label}: save PUT body carries the exact typed instructions`, firstPut?.body.orchestrator_instructions === VALID_TEXT, firstPut?.body.orchestrator_instructions);
  record(`${label}: no stage objective absorbed the instructions text`,
    Boolean(firstPut) && !firstPut.body.stages.some((stage) => typeof stage.objective === "string" && stage.objective.includes(VALID_TEXT)));

  // 3. Reload/reopen the editor route; the field must show the exact saved text.
  await page.reload();
  const reopened = page.getByLabel("Shared orchestrator instructions");
  await reopened.waitFor({ timeout: 8000 });
  record(`${label}: reopened editor shows the exact saved text`, (await reopened.inputValue()) === VALID_TEXT);

  // 4. Enter 16,001 characters, save; expect the 422 diagnostic for the field.
  await reopened.fill(TOO_LONG_TEXT);
  record(`${label}: draft holds the 16,001-char text before saving`, (await reopened.inputValue()).length === 16001);
  const beforeBadPuts = PUTS.length;
  await page.getByRole("button", { name: "Save template" }).click();
  await page.locator(".pipeline-diagnostics").waitFor({ timeout: 8000 }).catch(() => {});
  await page.waitForTimeout(300);
  const badPut = PUTS[beforeBadPuts];
  record(`${label}: over-limit save sent the full 16,001-char body`, badPut?.body.orchestrator_instructions.length === 16001);
  const diagnosticsList = page.locator(".pipeline-diagnostics");
  const diagnosticsText = await diagnosticsList.innerText().catch(() => "");
  record(`${label}: diagnostic names orchestrator_instructions`, /orchestrator_instructions/.test(diagnosticsText), diagnosticsText);
  record(`${label}: diagnostic field rendered in <code>`, await diagnosticsList.locator("code", { hasText: "orchestrator_instructions" }).count() === 1);
  record(`${label}: textarea still holds the 16,001-char draft after the failed save`, (await page.getByLabel("Shared orchestrator instructions").inputValue()).length === 16001);
  await page.screenshot({ path: join(OUT_DIR, `diagnostic-${label}.png`), fullPage: true });

  // 5. Restore valid text, save, then run the normal start flow for this template.
  const field = page.getByLabel("Shared orchestrator instructions");
  await field.fill(VALID_TEXT);
  const beforeRestorePuts = PUTS.length;
  await page.getByRole("button", { name: "Save template" }).click();
  await page.waitForTimeout(500);
  record(`${label}: restore save PUT body carries the valid text`, PUTS[beforeRestorePuts]?.body.orchestrator_instructions === VALID_TEXT);

  await page.goto(`${BASE}/pipelines/runs`);
  await page.getByRole("button", { name: "Start run" }).first().click();
  await page.getByLabel("Template").selectOption(TEMPLATE_ID);
  await page.getByLabel("Run goal").fill("Ship the note");
  const review = page.getByRole("button", { name: "Review" });
  await review.waitFor();
  await page.waitForFunction(() => [...document.querySelectorAll("button")].some((b) => b.textContent === "Review" && !b.disabled), null, { timeout: 5000 }).catch(() => {});
  await page.screenshot({ path: join(OUT_DIR, `start-${label}.png`), fullPage: true });
  await review.click();
  const beforeStarts = STARTS.length;
  await page.getByRole("button", { name: "Start run" }).last().click();
  await page.waitForURL(/\/pipelines\/runs\/pr_release/, { timeout: 5000 }).catch(() => {});
  record(`${label}: start request was sent`, STARTS.length === beforeStarts + 1, STARTS.length);
  const started = STARTS[beforeStarts];
  record(`${label}: start request carries no orchestrator_instructions override`, Boolean(started) && !("orchestrator_instructions" in started), JSON.stringify(started ? Object.keys(started) : []));

  record(`${label}: no unhandled /api calls`, UNHANDLED.length === 0, UNHANDLED.join(" | "));
  record(`${label}: journey without page errors`, errors.length === 0, errors.join(" | "));
  await context.close();
}

async function main() {
  const server = await createServer({ server: { port: PORT, strictPort: true }, logLevel: "error" });
  await server.listen();
  const browser = await chromium.launch().catch(() => chromium.launch({ executablePath: "/opt/pw-browsers/chromium" }));
  try {
    for (const skin of SKINS) {
      await editorJourney(browser, skin, 1440);
    }
    // One extra, cheap editor-only screenshot at 1024px (core skin) to check the
    // narrower layout doesn't clip or overflow the field.
    await editorJourney(browser, "", 1024, { shootEditor: true });
  } finally {
    await browser.close();
    await server.close();
  }
  writeFileSync(join(OUT_DIR, "report.json"), JSON.stringify(checks, null, 2));
  console.log(`\nwrote ${checks.length} checks, ${checks.filter((c) => !c.pass).length} failing`);
  console.log(`out dir: ${OUT_DIR}`);
  if (checks.some((c) => !c.pass)) process.exitCode = 1;
}

main().catch((err) => { console.error(err); process.exitCode = 1; });
