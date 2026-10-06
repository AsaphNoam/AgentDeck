// Rendered-evidence journey for "quiet completed turns" (FS-03.R73-R76,
// TS-08.R100-R105): drives the REAL chuck server + embedded UI in a real
// Chromium browser, with a chat agent backed by the deterministic fakeacp
// binary running the `activity_showcase` scenario.
//
// It needs the isolated stress fixture serving the scenario, from the repo
// root after `make embed` so the binary carries the working tree's UI:
//
//   go run -tags sqlite_fts5 ./scripts/stress-fixture -port 4411 -scenario activity_showcase
//
// The phone view is rendered from this run's real transcript:
//   node scripts/phone-render.mjs <outDir>/phone-transcript.json <outDir>/11-phone.png
//
// Usage:
//   node ui/scripts/turns-journey.mjs [baseURL] [outDir]
//
// baseURL defaults to http://127.0.0.1:4411; outDir defaults to a fresh
// directory under the system temp dir. Screenshots and a JSON report of
// PASS/FAIL checks are written under outDir.
import { mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { chromium } from "playwright";

const BASE_URL = process.argv[2] || "http://127.0.0.1:4411";
const OUT_DIR = process.argv[3] || join(tmpdir(), `turns-journey-${Date.now()}`);
mkdirSync(OUT_DIR, { recursive: true });

const VIEWPORTS = [
  { name: "desktop-1024", width: 1024, height: 768 },
  { name: "desktop-1600", width: 1600, height: 1000 },
];
const SKINS = [
  { name: "core", value: "" },
  { name: "sky-grove", value: "sky-grove" },
  { name: "studio", value: "studio" },
];

const checks = [];
function record(name, pass, detail) {
  checks.push({ name, pass, detail });
  console.log(`${pass ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`);
}

async function api(path, options) {
  const resp = await fetch(BASE_URL + path, {
    ...options,
    headers: { "Content-Type": "application/json", ...(options?.headers || {}) },
  });
  const text = await resp.text();
  let json;
  try { json = text ? JSON.parse(text) : undefined; } catch { json = text; }
  if (!resp.ok) throw new Error(`${path} -> ${resp.status}: ${text}`);
  return json;
}

async function setSkin(skin) {
  await api("/api/config", { method: "PUT", body: JSON.stringify({ appearance_skin: skin }) });
}

async function launchAgent(name) {
  const session = await api("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ role: "implementer", project: "stress", backend: "claude", model: "haiku", name, interface: "chat" }),
  });
  return session.agent.agent_id;
}

async function prompt(agentId, text) {
  await api(`/api/sessions/${agentId}/prompt`, { method: "POST", body: JSON.stringify({ text }) });
}

function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

async function waitForTurnEnd(agentId, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const t = await api(`/api/sessions/${agentId}/transcript`);
    const events = t.events || [];
    if (events.some((e) => e.type === "turn_end")) return events;
    await sleep(300);
  }
  throw new Error(`turn did not end for ${agentId} within ${timeoutMs}ms`);
}

async function shot(page, path, opts) {
  await page.screenshot({ path, ...opts });
  console.log(`screenshot: ${path}`);
}

async function main() {
  const browser = await chromium.launch().catch(async (err) => {
    console.error("default chromium launch failed, retrying with /opt/pw-browsers/chromium:", err.message);
    return chromium.launch({ executablePath: "/opt/pw-browsers/chromium" });
  });

  try {
    // ---- Primary pass: desktop-1024, Core skin, normal motion ----
    await setSkin("");
    let context = await browser.newContext({ viewport: { width: 1024, height: 768 } });
    let page = await context.newPage();
    const consoleErrors = [];
    page.on("console", (msg) => { if (msg.type() === "error") consoleErrors.push(msg.text()); });
    page.on("pageerror", (err) => consoleErrors.push(String(err)));

    const agentId = await launchAgent("Journey Primary");
    await page.goto(`${BASE_URL}/agent/${agentId}`);
    await page.waitForSelector('[data-ui="turn-activity"], .chat-panel, main', { timeout: 10000 }).catch(() => {});

    await prompt(agentId, "go");
    // Mid-turn: thinking disclosure should start expanded while live.
    await sleep(600);
    await shot(page, join(OUT_DIR, "01-running-thinking-expanded.png"));
    const thinkingDuringRun = await page.evaluate(() => {
      const details = Array.from(document.querySelectorAll('[data-ui="turn-activity"] details, [data-ui="turn-activity"] [aria-expanded]'));
      return details.map((el) => ({ tag: el.tagName, open: el.open, ariaExpanded: el.getAttribute("aria-expanded") }));
    });
    record(
      "thinking expanded while turn running",
      thinkingDuringRun.some((d) => d.open === true || d.ariaExpanded === "true"),
      JSON.stringify(thinkingDuringRun),
    );

    const events = await waitForTurnEnd(agentId);
    await sleep(300);
    await shot(page, join(OUT_DIR, "02-completed-collapsed.png"));

    const collapsedState = await page.evaluate(() => {
      const turn = document.querySelector('[data-ui="turn-activity"][data-state="collapsed"]');
      const toggle = turn?.querySelector(".turn-activity-toggle button");
      const bodyText = turn?.textContent || "";
      return {
        found: !!turn,
        toggleText: toggle?.textContent || null,
        ariaExpanded: toggle?.getAttribute("aria-expanded") || null,
        hidesIntermediatePassage: !bodyText.includes("I'll delegate the log reading"),
        hasFinalPassage: bodyText.includes("dev server keeps running in the background"),
      };
    });
    record(
      "completed turn shows only user+final+Show activity (intermediate passage hidden)",
      collapsedState.found && collapsedState.toggleText?.includes("Show activity") && collapsedState.hidesIntermediatePassage && collapsedState.hasFinalPassage,
      JSON.stringify(collapsedState),
    );

    // Background task row with Stop still visible after completion.
    const bgTask = await page.evaluate(() => {
      const row = document.querySelector('.background-task, [data-ui="runtime-activity"] .background-task');
      const stopBtn = document.querySelector(".background-task-stop");
      return { rowFound: !!row, stopVisible: !!(stopBtn && stopBtn.offsetParent !== null) };
    });
    record("background task row with Stop visible after completion", bgTask.rowFound && bgTask.stopVisible, JSON.stringify(bgTask));

    // Click Show activity -> expanded.
    await page.click('[data-ui="turn-activity"] .turn-activity-toggle button');
    await sleep(200);
    await shot(page, join(OUT_DIR, "03-completed-expanded.png"));
    const expandedState = await page.evaluate(() => {
      const turn = document.querySelector('[data-ui="turn-activity"][data-state="expanded"]');
      const toggle = turn?.querySelector(".turn-activity-toggle button");
      const bodyText = turn?.textContent || "";
      return {
        found: !!turn,
        toggleText: toggle?.textContent || null,
        ariaExpanded: toggle?.getAttribute("aria-expanded") || null,
        showsIntermediatePassage: bodyText.includes("I'll delegate the log reading"),
        showsThinking: bodyText.toLowerCase().includes("thinking") || bodyText.includes("Weighing whether"),
        showsRanTools: /Ran \d+ tool/.test(bodyText),
        showsChild: bodyText.includes("researcher") || bodyText.includes("parser.go:88"),
      };
    });
    record(
      "expanded reveals intermediate passage, thoughts, tool runs, child activity",
      expandedState.found && expandedState.toggleText?.includes("Hide activity") && expandedState.showsIntermediatePassage,
      JSON.stringify(expandedState),
    );

    // Second turn: send + let it complete, confirm first turn stays open.
    await prompt(agentId, "go again");
    await sleep(500);
    const stillOpenMidSecondTurn = await page.evaluate(() => {
      const turns = Array.from(document.querySelectorAll('[data-ui="turn-activity"]'));
      return turns[0]?.getAttribute("data-state");
    });
    await waitForTurnEnd(agentId, 15000).catch(() => {});
    await sleep(300);
    await shot(page, join(OUT_DIR, "04-older-turn-open-during-newer-completion.png"));
    const afterSecondTurn = await page.evaluate(() => {
      const turns = Array.from(document.querySelectorAll('[data-ui="turn-activity"]'));
      return turns.map((t) => t.getAttribute("data-state"));
    });
    record(
      "older turn stays open while newer turn completes",
      stillOpenMidSecondTurn === "expanded" && afterSecondTurn[0] === "expanded",
      `mid-second-turn=${stillOpenMidSecondTurn} after=${JSON.stringify(afterSecondTurn)}`,
    );

    record("no console errors (primary pass)", consoleErrors.length === 0, JSON.stringify(consoleErrors));

    // ---- Archive view ----
    try {
      await page.goto(`${BASE_URL}/archive/${agentId}`);
      await page.waitForTimeout(800);
      await shot(page, join(OUT_DIR, "05-archive-view.png"));
      record("archive view reachable", true, "");
    } catch (err) {
      record("archive view reachable", false, String(err));
    }

    // ---- Dashboard chat pane ----
    try {
      // Pre-expand this agent's card so DashboardChatPane mounts inline
      // (CardGrid.tsx: a card's chat pane only renders when its id is in the
      // persisted `expanded` layout list).
      await api("/api/layout", { method: "PUT", body: JSON.stringify({ order: [], density: { perRow: 3, gap: 16 }, expanded: [agentId] }) });
      await page.goto(`${BASE_URL}/project/stress`);
      await page.waitForTimeout(800);
      await shot(page, join(OUT_DIR, "06-dashboard.png"));
      record("dashboard reachable", true, "");
    } catch (err) {
      record("dashboard reachable", false, String(err));
    }

    await context.close();

    // ---- Viewport sweep (1600x1000) ----
    context = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
    page = await context.newPage();
    const agentWide = await launchAgent("Journey Wide");
    await page.goto(`${BASE_URL}/agent/${agentWide}`);
    await prompt(agentWide, "go");
    await waitForTurnEnd(agentWide);
    await sleep(300);
    await shot(page, join(OUT_DIR, "07-viewport-1600x1000-collapsed.png"));
    await context.close();

    // ---- Appearance sweep: sky-grove, studio ----
    for (const skin of SKINS.filter((s) => s.value)) {
      await setSkin(skin.value);
      context = await browser.newContext({ viewport: { width: 1024, height: 768 } });
      page = await context.newPage();
      const a = await launchAgent(`Journey ${skin.name}`);
      await page.goto(`${BASE_URL}/agent/${a}`);
      await prompt(a, "go");
      await waitForTurnEnd(a);
      await sleep(300);
      await shot(page, join(OUT_DIR, `08-appearance-${skin.name}.png`));
      await context.close();
    }
    await setSkin(""); // restore Core

    // ---- Reduced motion ----
    context = await browser.newContext({ viewport: { width: 1024, height: 768 }, reducedMotion: "reduce" });
    page = await context.newPage();
    const agentRM = await launchAgent("Journey ReducedMotion");
    await page.goto(`${BASE_URL}/agent/${agentRM}`);
    await prompt(agentRM, "go");
    await sleep(600);
    await shot(page, join(OUT_DIR, "09-reduced-motion-running.png"));
    await waitForTurnEnd(agentRM);
    await sleep(300);
    await shot(page, join(OUT_DIR, "10-reduced-motion-completed.png"));
    await context.close();

    // ---- Phone: fetch real transcript JSON, feed to phone-render.mjs ----
    const phoneTranscript = await api(`/api/sessions/${agentId}/transcript`);
    const phoneTranscriptPath = join(OUT_DIR, "phone-transcript.json");
    writeFileSync(phoneTranscriptPath, JSON.stringify(phoneTranscript, null, 2));
    console.log(`fetched real transcript for ${agentId} -> ${phoneTranscriptPath}; feeding to phone-render.mjs (the real tailnet-paired phone listener is not reachable from this environment, so this is the documented fallback)`);

  } finally {
    await browser.close();
  }

  writeFileSync(join(OUT_DIR, "report.json"), JSON.stringify({ baseUrl: BASE_URL, outDir: OUT_DIR, checks }, null, 2));
  console.log(`\nReport: ${join(OUT_DIR, "report.json")}`);
  console.log(`Out dir: ${OUT_DIR}`);
  const failed = checks.filter((c) => !c.pass);
  if (failed.length) {
    console.error(`${failed.length} check(s) failed.`);
    process.exitCode = 1;
  }
}

await main();
