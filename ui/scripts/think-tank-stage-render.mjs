// Rendered evidence for Think Tank pipeline stages, readable shared-workspace
// consent and per-parent task collapse (FS-14.A50/A51, FS-16.A30, FS-21.A31).
// Drives Vite + the real UI source in Chromium with every /api call and the
// event stream stubbed (same pattern as scripts/chat-cleanup-render.mjs), in
// Core, Sky & Grove and Studio at 1024px and 1440px.
//
//   node scripts/think-tank-stage-render.mjs [outDir]
//
// Writes screenshots and report.json to outDir. Exits 1 if any check fails.
import { mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createServer } from "vite";
import { chromium } from "playwright";

const OUT_DIR = process.argv[2] || join(tmpdir(), `think-tank-stage-render-${Date.now()}`);
mkdirSync(OUT_DIR, { recursive: true });
const PORT = 5199;
const BASE = `http://localhost:${PORT}`;
const AT = "2026-10-08T09:00:00Z";
const SKINS = ["", "sky-grove", "studio"];
const WIDTHS = [1024, 1440];
const skinLabel = (skin) => skin || "core";

const checks = [];
function record(name, pass, detail) {
  checks.push({ name, pass: Boolean(pass), detail: detail === undefined ? undefined : String(detail) });
  console.log(`${pass ? "PASS" : "FAIL"} ${name}${detail !== undefined ? ` — ${detail}` : ""}`);
}

// ---- Fixtures --------------------------------------------------------------

const slot = { backend: "codex", model: "gpt", effort: "", fast: false };
const template = {
  version: 2, title: "Decide and build", orchestrator_role: "implementer",
  inputs: [{ name: "spec", description: "Specification", required: false }],
  stages: [
    { id: "draft", title: "Draft options", objective: "Draft options.", coordination: "standing", dedicated_role: "", approval_after_success: false,
      inputs: [], outputs: [{ name: "options", value: "options", description: "Options" }] },
    { id: "debate", title: "Debate", objective: "Choose an option.", coordination: "think_tank", dedicated_role: "", approval_after_success: true,
      think_tank: { participants: [{ id: "pro", role: "implementer", limit: 2, may_leave: false }, { id: "con", role: "implementer", limit: 2, may_leave: false }], openings: false, judge_role: "implementer" },
      inputs: [{ name: "options", value: "options", required: true }], outputs: [{ name: "decision", value: "decision", description: "Judge synthesis" }] },
    { id: "build", title: "Build", objective: "Build it.", coordination: "standing", dedicated_role: "", approval_after_success: false,
      inputs: [{ name: "decision", value: "decision", required: true }], outputs: [] },
  ],
};

const owner = { agent_id: "a_owner", name: "Owner", state: "idle", route: "live", runtime: slot };
const noOwner = { agent_id: "", name: "", state: "unknown", route: "unavailable", runtime: { backend: "", model: "" } };
const draftStage = {
  task_id: "tk_draft", run_id: "", stage_id: "draft", stage_index: 0, attempt_number: 1, state: "finished", assignment_text: "",
  execution_kind: "agent", standing_owner: owner, work: [], created_at: AT, updated_at: AT,
  result: { outcome: "success", summary: "Two options drafted.", details: "", checks: "", outputs: { options: "A: durable queue. B: polling." } },
};
const roomStage = (state, extra = {}) => ({
  task_id: "tk_room", run_id: "", stage_id: "debate", stage_index: 1, attempt_number: 1, state, assignment_text: "",
  execution_kind: "think_tank", standing_owner: noOwner, work: [], created_at: AT, updated_at: AT,
  room: { room_id: "tt_stage", run_id: "", stage_id: "debate", phase: "discussion", judge_status: "waiting", source_entry_seq: 0 }, ...extra,
});
const control = (eligible, reason = "") => ({ eligible, reason });
const RUNS = {
  pr_discussion: { state: "running", pending: "", attention: "", stages: [draftStage, roomStage("running")] },
  pr_accepting: { state: "running", pending: "", attention: "", stages: [draftStage, roomStage("waiting", { attention_reason: "Accepting judge output", room: { room_id: "tt_stage", run_id: "", stage_id: "debate", phase: "ended", judge_status: "completed", source_entry_seq: 0 } })] },
  pr_held: { state: "paused", pending: "accept_room_output", attention: "output acceptance failed: output decision: the synthesis exceeds 64000 characters", retry: true,
    stages: [draftStage, roomStage("interrupted", { attention_reason: "output acceptance failed: output decision: the synthesis exceeds 64000 characters", room: { room_id: "tt_stage", run_id: "", stage_id: "debate", phase: "ended", judge_status: "completed", source_entry_seq: 0 } })] },
  pr_room_failed: { state: "paused", pending: "", attention: "Think Tank needs attention: Setup failed for Debate · con: provider unavailable.",
    stages: [draftStage, roomStage("interrupted", { attention_reason: "Setup failed for Debate · con: provider unavailable.", room: { room_id: "tt_stage", run_id: "", stage_id: "debate", phase: "setup", judge_status: "waiting", source_entry_seq: 0 } })] },
  pr_gate: { state: "paused", pending: "await_approval", attention: "approval_required", cont: true,
    stages: [draftStage, roomStage("finished", { room: { room_id: "tt_stage", run_id: "", stage_id: "debate", phase: "ended", judge_status: "completed", source_entry_seq: 7 },
      result: { outcome: "success", summary: "Think Tank judge synthesis accepted from room tt_stage entry 7.", details: "", checks: "", outputs: { decision: "Choose A. Dissent: con prefers B for simplicity." } } })] },
};
function runDetail(id) {
  const spec = RUNS[id];
  const stages = spec.stages.map((stage) => ({ ...stage, run_id: id, room: stage.room ? { ...stage.room, run_id: id } : undefined }));
  return {
    run: {
      run_id: id, template_id: "decide", template_snapshot: template, display_name: "Decide and build", project: "demo", goal: "Pick a queue and build it",
      inputs: {}, state: spec.state, revision: 4, pending_action: spec.pending, current_stage_id: "debate", current_task_id: "tk_room",
      orchestrator_agent_id: "", current_attempt_id: "", current_agent_id: "", attention_reason: spec.attention, final_outcome: "", created_at: AT, updated_at: AT,
    },
    template, inputs: {}, orchestrator: slot, dedicated_assignments: {}, assignments: { standing: slot },
    think_tank_assignments: { debate: { participants: { pro: slot, con: slot }, judge: slot } },
    stage_tasks: stages, values: [], diagnostics: [],
    controls: {
      continue: control(Boolean(spec.cont), "Approves the accepted judge synthesis."),
      retry: control(Boolean(spec.retry), spec.retry ? "Retry output acceptance of the same published synthesis." : "Open the Think Tank room to resume or retry it."),
      replace: control(false, "Open the Think Tank room to resume or retry it."), stop: control(true, "Stops the run."), repair_cleanup: control(false),
    },
  };
}

const task = (id, name, parent, state, extra = {}) => ({
  task_id: id, project: "demo", display_name: name, instruction: name, target_kind: "launch", role: "implementer", state,
  created_by_kind: "agent", created_by_agent_id: "a_owner", revision: 1, created_at: AT, updated_at: AT,
  lineage: { parent_task_id: parent, pipeline_run_id: "", pipeline_stage_id: "", creation_attempt_id: "" }, ...extra,
});
const TASKS = [
  task("tk_coord", "Coordinate the build", "", "running"),
  task("tk_worker", "Implement queue", "tk_coord", "running"),
  task("tk_review", "Review queue", "tk_worker", "interrupted", { attention_reason: "agent exited" }),
  task("tk_room_task", "Debate", "", "waiting", { target_kind: "think_tank", role: "", created_by_kind: "pipeline", created_by_agent_id: "", attention_reason: "Accepting judge output",
    room: { room_id: "tt_stage", run_id: "pr_accepting", stage_id: "debate", phase: "ended", judge_status: "completed", source_entry_seq: 0 } }),
];

const room = {
  version: 1, room_id: "tt_stage", title: "Debate", goal: "Pipeline stage \"Debate\" of run \"Decide and build\".", origin_project: "demo",
  pipeline: { run_id: "pr_accepting", stage_id: "debate", task_id: "tk_room", pinned: true },
  phase: "ended", control: "running", hold: "", end_reason: "participants_left", judge_status: "completed", participants: ["Debate · pro", "Debate · con"],
  revision: 9, created_at: AT, updated_at: AT, openings: false, active_attempts: [], roster: [], total_remaining: 0, judge_enabled: true, judge_name: "Debate · judge",
  members: [
    { agent_id: "a_pro", name: "Debate · pro", project: "demo", role: "participant", order: 0, limit: 2, completed: 1, may_leave: false, state: "active", setup_state: "ready", exists: true, running: true, agent_status: "idle" },
    { agent_id: "a_con", name: "Debate · con", project: "demo", role: "participant", order: 1, limit: 2, completed: 1, may_leave: false, state: "active", setup_state: "ready", exists: true, running: true, agent_status: "idle" },
    { agent_id: "a_judge", name: "Debate · judge", project: "demo", role: "judge", order: 2, limit: 1, completed: 1, may_leave: false, state: "active", setup_state: "ready", exists: true, running: true, agent_status: "idle" },
  ],
  pending: [], failed: [], judge: { enabled: true, status: "completed", agent_id: "a_judge", error: "" }, deletable: false,
};
const entry = (seq, kind, agent_id, agent_name, body) => ({ seq, kind, agent_id, agent_name, project: "demo", body, attempt_id: agent_id ? `tta_${seq}` : "", created_at: AT });
const ENTRIES = [
  // Same text renderThinkTankStageContext (internal/state) produces.
  entry(1, "stage_context", "", "Pipeline", "Pipeline stage context for run \"Decide and build\" (`pr_accepting`), stage \"Debate\" (`debate`).\n\n**Run goal**\n\nPick a queue and build it\n\n**Stage objective**\n\nChoose an option.\n\n**Input `options`**\n\nA: durable queue. B: polling.\n\n**Required output `decision`**\n\nJudge synthesis\n\nThe judge's published synthesis becomes this output exactly and must be at most 64000 characters. Completion means a synthesis was produced, not that participants agreed.\n"),
  entry(2, "reply", "a_pro", "Debate · pro", "A survives restarts."),
  entry(3, "reply", "a_con", "Debate · con", "B is simpler to operate."),
  entry(4, "synthesis", "a_judge", "Debate · judge", "Choose A. Dissent: con prefers B for simplicity."),
];

let starts = [];
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
    if (path === "/api/pipelines") return route.fulfill({ json: [{ id: "decide", template, valid: true, diagnostics: [] }] });
    if (path === "/api/pipeline-proposals") return route.fulfill({ json: [] });
    if (path === "/api/pipeline-runs" && req.method() === "GET") return route.fulfill({ json: [], headers: { "X-Total-Count": "0" } });
    if (path === "/api/pipeline-runs" && req.method() === "POST") {
      const body = JSON.parse(req.postData() || "{}");
      starts.push({ ...body, acknowledge: url.searchParams.get("acknowledge_shared_workspace") ?? body.acknowledge_shared_workspace });
      const acknowledged = body.acknowledge_shared_workspace === true || url.searchParams.get("acknowledge_shared_workspace") === "true";
      if (!acknowledged) {
        return route.fulfill({ status: 409, json: { error: { code: "conflict", message: "another active agent or pipeline run shares this project",
          details: { code: "shared_workspace_confirmation_required", conflicts: [{ kind: "agent", id: "a_busy", name: "Busy agent" }, { kind: "run", id: "pr_other", name: "Other run" }] } } } });
      }
      return route.fulfill({ status: 201, json: { run: runDetail("pr_discussion"), replay: false, workspace_conflicts: [] } });
    }
    const runMatch = path.match(/^\/api\/pipeline-runs\/([^/]+)$/);
    if (runMatch && RUNS[runMatch[1]]) return route.fulfill({ json: runDetail(runMatch[1]) });
    if (path === "/api/tasks") return route.fulfill({ json: { tasks: TASKS } });
    if (path === "/api/think-tanks/tt_stage") return route.fulfill({ json: room });
    if (path === "/api/think-tanks/tt_stage/entries") return route.fulfill({ json: { version: 1, entries: ENTRIES, complete: true } });
    if (path === "/api/think-tanks/tt_stage/activity") return route.fulfill({ json: { version: 1, activity: [], complete: true } });
    if (path === "/api/think-tanks") return route.fulfill({ json: { version: 1, rooms: [], clipped: false } });
    if (path === "/api/config-sources") return route.fulfill({ json: [] });
    return route.fulfill({ json: {} });
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

// WCAG contrast of an element's text colour against its nearest painted
// background, computed in the page.
async function contrast(locator) {
  return locator.evaluate((el) => {
    // Computed colours arrive as rgb()/rgba() or, from color-mix(), as
    // color(srgb r g b / a) with 0–1 channels.
    const parse = (value) => {
      const rgb = value.match(/rgba?\(([^)]+)\)/);
      if (rgb) { const p = rgb[1].split(",").map((x) => parseFloat(x)); return { r: p[0], g: p[1], b: p[2], a: p[3] ?? 1 }; }
      const srgb = value.match(/color\(srgb\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)(?:\s*\/\s*([\d.]+))?\)/);
      if (srgb) return { r: +srgb[1] * 255, g: +srgb[2] * 255, b: +srgb[3] * 255, a: srgb[4] === undefined ? 1 : +srgb[4] };
      return null;
    };
    const lum = ({ r, g, b }) => { const c = [r, g, b].map((v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; }); return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]; };
    let node = el; let bg = null;
    while (node && node.nodeType === 1) { const c = parse(getComputedStyle(node).backgroundColor); if (c && c.a > 0.5) { bg = c; break; } node = node.parentElement; }
    const fg = parse(getComputedStyle(el).color);
    if (!bg || !fg) return 0;
    const [a, b] = [lum(fg), lum(bg)].sort((x, y) => y - x);
    return Math.round(((a + 0.05) / (b + 0.05)) * 100) / 100;
  });
}

// ---- Journeys ----------------------------------------------------------------

async function startJourney(browser, skin, width) {
  const label = `${skinLabel(skin)}@${width}`;
  starts = [];
  const { context, page, errors } = await newPage(browser, skin, width);
  await page.goto(`${BASE}/pipelines/runs`);
  await page.getByRole("button", { name: "Start run" }).first().click();
  await page.getByLabel("Template").selectOption("decide");
  await page.getByLabel("Run goal").fill("Pick a queue and build it");
  const review = page.getByRole("button", { name: "Review" });
  await review.waitFor();
  await page.waitForFunction(() => [...document.querySelectorAll("button")].some((b) => b.textContent === "Review" && !b.disabled), null, { timeout: 5000 }).catch(() => {});
  const summary = await page.locator(".pipeline-runtime-summary").innerText();
  record(`${label}: start lists standing, room participants and judge`, /Standing owner/.test(summary) && /Debate Think Tank · pro/.test(summary) && /Debate Think Tank · judge/.test(summary), summary.replace(/\s+/g, " ").slice(0, 160));
  await review.click();
  await page.getByRole("button", { name: "Start run" }).last().click();
  const warning = page.locator(".pipeline-warning");
  await warning.waitFor({ timeout: 5000 });
  for (const [name, locator] of [["heading", warning.locator("strong").first()], ["explanation", warning.locator("p").first()], ["conflict", warning.locator("li").first()]]) {
    const ratio = await contrast(locator);
    record(`${label}: warning ${name} contrast ≥ 4.5`, ratio >= 4.5, ratio);
  }
  const confirm = warning.getByRole("button", { name: "Confirm shared workspace and start" });
  const buttonRatio = await contrast(confirm);
  record(`${label}: confirm button contrast ≥ 4.5`, buttonRatio >= 4.5, buttonRatio);
  await confirm.focus();
  const focusVisible = await confirm.evaluate((el) => el.matches(":focus-visible") || document.activeElement === el);
  record(`${label}: confirm button takes keyboard focus`, focusVisible);
  record(`${label}: nothing started before acknowledgement`, starts.length === 1 && !starts[0].acknowledge_shared_workspace, JSON.stringify(starts.map((s) => s.acknowledge_shared_workspace)));
  await page.screenshot({ path: join(OUT_DIR, `start-warning-${label}.png`), fullPage: true });
  await page.keyboard.press("Enter");
  await page.waitForURL(/\/pipelines\/runs\/pr_discussion/, { timeout: 5000 }).catch(() => {});
  const acknowledged = starts.filter((s) => s.acknowledge_shared_workspace === true);
  record(`${label}: exactly one acknowledged start with room slots`, acknowledged.length === 1 && Boolean(acknowledged[0].think_tank_assignments?.debate?.judge?.backend), JSON.stringify(acknowledged.map((s) => Object.keys(s.think_tank_assignments ?? {}))));
  record(`${label}: start journey without page errors`, errors.length === 0, errors.join(" | "));
  await context.close();
}

async function runStates(browser, skin, width) {
  const label = `${skinLabel(skin)}@${width}`;
  const expectations = {
    pr_discussion: { phase: "Discussion", button: null },
    pr_accepting: { phase: "Accepting judge output", button: null },
    pr_held: { phase: "Needs room recovery", button: "Retry output acceptance" },
    pr_room_failed: { phase: "Needs room recovery", button: null },
    pr_gate: { phase: "Judge synthesis accepted", button: "Approve and continue" },
  };
  for (const [id, want] of Object.entries(expectations)) {
    const { context, page, errors } = await newPage(browser, skin, width);
    await page.goto(`${BASE}/pipelines/runs/${id}`);
    const strip = page.locator(".pipeline-owner-strip").first();
    const shown = await strip.waitFor({ timeout: 8000 }).then(() => true, () => false);
    const text = shown ? await strip.innerText() : "";
    record(`${label} ${id}: phase reads "${want.phase}"`, text.includes("Think Tank stage") && text.includes(want.phase), text.replace(/\s+/g, " "));
    // Let the refetch fade and entrance motion settle before measuring/shooting.
    await page.locator(".pipeline-run-page:not(.pipeline-updating)").waitFor({ timeout: 5000 }).catch(() => {});
    await page.waitForTimeout(800);
    if (shown) {
      const phaseRatio = await contrast(strip.locator("strong").first());
      record(`${label} ${id}: phase text contrast ≥ 4.5`, phaseRatio >= 4.5, phaseRatio);
    }
    record(`${label} ${id}: Open Think Tank link`, await page.getByRole("link", { name: "Open Think Tank", exact: true }).count() === 1);
    record(`${label} ${id}: no standing-agent link or generic stage retry`, await page.getByRole("link", { name: "Open agent" }).count() === 0 && await page.getByRole("button", { name: "Retry stage" }).count() === 0);
    if (want.button) {
      record(`${label} ${id}: offers ${want.button}`, await page.getByRole("button", { name: want.button }).count() === 1);
      const hint = page.locator(".pipeline-action-choice small").first();
      if (await hint.count()) {
        const ratio = await contrast(hint);
        record(`${label} ${id}: action hint contrast ≥ 4.5`, ratio >= 4.5, ratio);
      }
    }
    if (id === "pr_gate") {
      const body = await page.locator(".pipeline-timeline-current").innerText();
      record(`${label} ${id}: names synthesis source and completion meaning`, /judge's synthesis \(room entry 7\)/.test(body) && /not that participants agreed/.test(body), body.replace(/\s+/g, " ").slice(0, 200));
    }
    if (id === "pr_held" || id === "pr_room_failed") {
      const ratio = await contrast(page.locator(".pipeline-warning p").first());
      record(`${label} ${id}: attention text contrast ≥ 4.5`, ratio >= 4.5, ratio);
    }
    await page.screenshot({ path: join(OUT_DIR, `run-${id}-${label}.png`), fullPage: true });
    record(`${label} ${id}: no page errors`, errors.length === 0, errors.join(" | "));
    await context.close();
  }
}

async function tasksCollapse(browser, skin, width) {
  const label = `${skinLabel(skin)}@${width}`;
  const { context, page, errors } = await newPage(browser, skin, width);
  await page.goto(`${BASE}/tasks`);
  const worker = page.getByText("Implement queue").first();
  const shown = await worker.waitFor({ timeout: 8000 }).then(() => true, () => false);
  record(`${label}: tasks rendered`, shown);
  if (!shown) { await context.close(); return; }
  record(`${label}: room task labelled Think Tank with room/run links`, await page.getByText(/runs as Think Tank/).count() === 1);
  const toggle = page.getByRole("button", { name: /Collapse tasks/ }).first();
  record(`${label}: parent disclosure is expanded initially`, (await toggle.getAttribute("aria-expanded")) === "true");
  await toggle.focus();
  await page.keyboard.press("Enter");
  await page.waitForTimeout(100);
  record(`${label}: collapse hides descendants`, !(await page.getByText("Implement queue").first().isVisible()) && !(await page.getByText("Review queue").first().isVisible()));
  record(`${label}: parent stays visible with hidden count`, await page.getByText("Coordinate the build").first().isVisible() && /2/.test(await page.getByRole("button", { name: /Expand tasks/ }).first().innerText()));
  await page.screenshot({ path: join(OUT_DIR, `tasks-collapsed-${label}.png`), fullPage: true });
  await page.reload();
  await page.getByText("Coordinate the build").first().waitFor({ timeout: 8000 });
  record(`${label}: collapse survives refresh`, !(await page.getByText("Implement queue").first().isVisible()));
  await page.getByRole("button", { name: /Expand tasks/ }).first().click();
  record(`${label}: expand restores descendants`, await page.getByText("Review queue").first().isVisible());
  record(`${label}: tasks without page errors`, errors.length === 0, errors.join(" | "));
  await context.close();
}

async function roomPage(browser, skin, width) {
  const label = `${skinLabel(skin)}@${width}`;
  const { context, page, errors } = await newPage(browser, skin, width);
  await page.goto(`${BASE}/think-tank/tt_stage`);
  const shown = await page.getByRole("list", { name: "Discussion" }).waitFor({ timeout: 8000 }).then(() => true, () => false);
  record(`${label}: pipeline room renders`, shown);
  record(`${label}: room links back to its run`, await page.getByRole("link", { name: /Pipeline run · stage debate/ }).count() === 1);
  record(`${label}: stage context entry first`, (await page.locator('.think-tank-entry').first().getAttribute("data-variant")) === "stage_context");
  record(`${label}: pinned room offers no Delete`, await page.getByRole("button", { name: "Delete" }).count() === 0);
  await page.screenshot({ path: join(OUT_DIR, `room-${label}.png`), fullPage: true });
  record(`${label}: room without page errors`, errors.length === 0, errors.join(" | "));
  await context.close();
}

async function main() {
  const server = await createServer({ server: { port: PORT, strictPort: true }, logLevel: "error" });
  await server.listen();
  const browser = await chromium.launch().catch(() => chromium.launch({ executablePath: "/opt/pw-browsers/chromium" }));
  try {
    for (const skin of SKINS) {
      for (const width of WIDTHS) {
        await startJourney(browser, skin, width);
        await runStates(browser, skin, width);
        await tasksCollapse(browser, skin, width);
        await roomPage(browser, skin, width);
      }
    }
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
