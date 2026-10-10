// Rendered-evidence journey for "quiet completed turns" (FS-03.R73-R76,
// TS-08.R100-R106): drives the REAL chuck server + embedded UI in a real
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

// archiveAgent exercises the real archive API (POST /api/sessions/{id}/archive,
// internal/server/archive_actions.go), not navigation alone: it stops the
// agent and marks it archived so the archive surface has something real to
// read back.
async function archiveAgent(agentId) {
  await api(`/api/sessions/${agentId}/archive`, { method: "POST" });
}

function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

// rootTurnEndCount counts only root turn_end events — a nested child/subagent
// scope never proves the root turn finished (ui/src/components/chat/turnActivity.ts
// isRootTurnEnd: kind === "turn_end" && !activity_id).
function rootTurnEndCount(events) {
  return events.filter((e) => (e.kind ?? e.type) === "turn_end" && !e.activity_id).length;
}

// waitForTurnEnd waits for a NEW root turn_end — one whose count exceeds
// previousCount, so it can't be satisfied by a turn that already ended before
// this call. A timeout is reported in the return value (timedOut: true)
// rather than thrown, so callers always get a result to assert against.
async function waitForTurnEnd(agentId, previousCount = 0, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  let events = [];
  while (Date.now() < deadline) {
    const t = await api(`/api/sessions/${agentId}/transcript`);
    events = t.events || [];
    const count = rootTurnEndCount(events);
    if (count > previousCount) return { events, count, timedOut: false };
    await sleep(300);
  }
  return { events, count: rootTurnEndCount(events), timedOut: true };
}

// expectTurnEnd wraps waitForTurnEnd with a recorded PASS/FAIL check so a
// timeout always surfaces as a failed check instead of being swallowed.
async function expectTurnEnd(name, agentId, previousCount = 0, timeoutMs = 15000) {
  const result = await waitForTurnEnd(agentId, previousCount, timeoutMs);
  record(name, !result.timedOut && result.count > previousCount, JSON.stringify({ previousCount, count: result.count, timedOut: result.timedOut }));
  return result;
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
    // Thoughts are admitted only once the open transcript has loaded
    // (TS-08.R103), so prompt after it renders.
    await page.waitForSelector('[data-ui="transcript"]', { timeout: 10000 });
    await sleep(300);

    await prompt(agentId, "go");
    // Mid-turn: thinking disclosure should start expanded while live.
    await page.waitForSelector('[data-ui="turn-activity"] [aria-expanded="true"]', { timeout: 5000 }).catch(() => {});
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

    const turn1 = await expectTurnEnd("first turn completed (new root turn_end observed)", agentId, 0);
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
      const stopBtn = document.querySelector(".background-task button");
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
      expandedState.found
        && expandedState.toggleText?.includes("Hide activity")
        && expandedState.showsIntermediatePassage
        && expandedState.showsThinking
        && expandedState.showsRanTools
        && expandedState.showsChild,
      JSON.stringify(expandedState),
    );

    // Second turn: send + let it complete, confirm first turn stays open.
    await prompt(agentId, "go again");
    await sleep(500);
    const stillOpenMidSecondTurn = await page.evaluate(() => {
      const turns = Array.from(document.querySelectorAll('[data-ui="turn-activity"]'));
      return turns[0]?.getAttribute("data-state");
    });
    const turn2 = await expectTurnEnd("second turn completed (new root turn_end observed)", agentId, turn1.count);
    await sleep(300);
    await shot(page, join(OUT_DIR, "04-older-turn-open-during-newer-completion.png"));
    const afterSecondTurn = await page.evaluate(() => {
      const turns = Array.from(document.querySelectorAll('[data-ui="turn-activity"]'));
      return turns.map((t) => t.getAttribute("data-state"));
    });
    record(
      "older turn stays open while newer turn completes",
      stillOpenMidSecondTurn === "expanded" && afterSecondTurn[0] === "expanded" && !turn2.timedOut && turn2.count > turn1.count,
      `mid-second-turn=${stillOpenMidSecondTurn} after=${JSON.stringify(afterSecondTurn)} turn2=${JSON.stringify(turn2.count)}`,
    );

    record("no console errors (primary pass)", consoleErrors.length === 0, JSON.stringify(consoleErrors));

    // ---- Archive: real archive action, then read the archive surface ----
    // A dedicated agent so archiving it cannot affect the primary agent's
    // later checks (dashboard pane, phone transcript).
    try {
      const archiveAgentId = await launchAgent("Journey Archive");
      await page.goto(`${BASE_URL}/agent/${archiveAgentId}`);
      await prompt(archiveAgentId, "go");
      const archiveTurn = await expectTurnEnd("archive-candidate turn completed", archiveAgentId, 0);
      await archiveAgent(archiveAgentId); // POST /api/sessions/{id}/archive — the real API, not navigation.
      await page.goto(`${BASE_URL}/archive/${archiveAgentId}`);
      await page.waitForSelector('[data-ui="transcript"]', { timeout: 10000 });
      await sleep(500);
      await shot(page, join(OUT_DIR, "05-archive-view.png"));
      const archiveView = await page.evaluate(() => {
        const turn = document.querySelector('[data-ui="turn-activity"][data-state="collapsed"]');
        const toggle = turn?.querySelector(".turn-activity-toggle button");
        const bodyText = document.querySelector('[data-ui="transcript"]')?.textContent || "";
        return {
          readOnlyLabel: bodyText.includes("Archived") || !!document.querySelector(".archive-readonly-label"),
          hasFinalPassage: bodyText.includes("dev server keeps running in the background"),
          disclosureFound: !!turn,
          toggleText: toggle?.textContent || null,
          ariaExpanded: toggle?.getAttribute("aria-expanded") || null,
        };
      });
      record(
        "archive view (real archive action) renders transcript content and a collapsed Show activity disclosure",
        !archiveTurn.timedOut
          && archiveView.readOnlyLabel
          && archiveView.hasFinalPassage
          && archiveView.disclosureFound
          && archiveView.toggleText?.includes("Show activity")
          && archiveView.ariaExpanded === "false",
        JSON.stringify({ archiveTurn: { count: archiveTurn.count, timedOut: archiveTurn.timedOut }, archiveView }),
      );
    } catch (err) {
      record("archive view (real archive action) renders transcript content and a collapsed Show activity disclosure", false, String(err));
    }

    // ---- Dashboard chat pane ----
    try {
      // Pre-expand this agent's card so DashboardChatPane mounts inline
      // (CardGrid.tsx: a card's chat pane only renders when its id is in the
      // persisted `expanded` layout list).
      await api("/api/layout", { method: "PUT", body: JSON.stringify({ order: [], density: { perRow: 3, gap: 16 }, expanded: [agentId] }) });
      await page.goto(`${BASE_URL}/project/stress`);
      await page.waitForSelector(`[data-agent-pane="${agentId}"] [data-ui="transcript"]`, { timeout: 10000 });
      await sleep(500);
      await shot(page, join(OUT_DIR, "06-dashboard.png"));
      const paneState = await page.evaluate((id) => {
        const pane = document.querySelector(`[data-agent-pane="${id}"]`);
        const toggle = pane?.querySelector(".turn-activity-toggle button");
        const bodyText = pane?.textContent || "";
        return {
          found: !!pane,
          hasFinalPassage: bodyText.includes("dev server keeps running in the background"),
          disclosureFound: !!toggle,
          toggleText: toggle?.textContent || null,
          ariaExpanded: toggle?.getAttribute("aria-expanded") || null,
        };
      }, agentId);
      record(
        "dashboard pane renders transcript content and a Show activity disclosure",
        paneState.found && paneState.hasFinalPassage && paneState.disclosureFound && paneState.toggleText?.includes("Show activity") && paneState.ariaExpanded === "false",
        JSON.stringify(paneState),
      );
    } catch (err) {
      record("dashboard pane renders transcript content and a Show activity disclosure", false, String(err));
    }

    await context.close();

    // ---- Keyboard operation: Tab/focus + Enter opens Show activity ----
    try {
      context = await browser.newContext({ viewport: { width: 1024, height: 768 } });
      page = await context.newPage();
      const agentKb = await launchAgent("Journey Keyboard");
      await page.goto(`${BASE_URL}/agent/${agentKb}`);
      await prompt(agentKb, "go");
      const kbTurn = await expectTurnEnd("keyboard-scenario turn completed", agentKb, 0);
      await sleep(300);
      const toggleSelector = '[data-ui="turn-activity"][data-state="collapsed"] .turn-activity-toggle button';
      await page.waitForSelector(toggleSelector, { timeout: 10000 });
      const before = await page.evaluate((sel) => {
        const el = document.querySelector(sel);
        el?.focus();
        return { isActive: document.activeElement === el, ariaExpanded: el?.getAttribute("aria-expanded") || null };
      }, toggleSelector);
      await page.keyboard.press("Enter");
      await sleep(200);
      const after = await page.evaluate(() => {
        const turn = document.querySelector('[data-ui="turn-activity"][data-state="expanded"]');
        const toggle = turn?.querySelector(".turn-activity-toggle button");
        return {
          ariaExpanded: toggle?.getAttribute("aria-expanded") || null,
          toggleText: toggle?.textContent || null,
          contentVisible: !!turn?.querySelector(".turn-activity-content"),
          toggleIsActive: document.activeElement === toggle,
        };
      });
      await shot(page, join(OUT_DIR, "11-keyboard-enter-opens.png"));
      record(
        "keyboard focus + Enter opens Show activity (aria-expanded flips, content shows, label reads Hide activity)",
        !kbTurn.timedOut && before.isActive && before.ariaExpanded === "false" && after.ariaExpanded === "true" && after.contentVisible && after.toggleText?.includes("Hide activity"),
        JSON.stringify({ before, after }),
      );
      await context.close();
    } catch (err) {
      record("keyboard focus + Enter opens Show activity (aria-expanded flips, content shows, label reads Hide activity)", false, String(err));
    }

    // ---- Focus inside live activity returns to its controlling disclosure
    // on automatic collapse (TS-08.R105), rather than falling to body. ----
    try {
      context = await browser.newContext({ viewport: { width: 1024, height: 768 } });
      page = await context.newPage();
      const agentFocus = await launchAgent("Journey FocusReturn");
      await page.goto(`${BASE_URL}/agent/${agentFocus}`);
      await prompt(agentFocus, "go");
      const thinkingSelector = '[data-ui="runtime-activity"][data-slot="thinking"] button.tool-toggle';
      await page.waitForSelector(thinkingSelector, { timeout: 10000 });
      const liveFocus = await page.evaluate((sel) => {
        const el = document.querySelector(sel);
        el?.focus();
        return { isActive: document.activeElement === el, insideThinking: !!document.activeElement?.closest('[data-slot="thinking"]') };
      }, thinkingSelector);
      const focusTurn = await expectTurnEnd("focus-return scenario turn completed", agentFocus, 0);
      await sleep(400); // let the completion re-render (and the focus-return effect) settle
      const afterFocus = await page.evaluate(() => {
        const el = document.activeElement;
        const turn = el?.closest('[data-ui="turn-activity"]');
        return {
          onBody: el === document.body,
          isTurnToggle: !!(el && el.matches && el.matches(".turn-activity-toggle button")),
          turnState: turn?.getAttribute("data-state") || null,
        };
      });
      await shot(page, join(OUT_DIR, "12-focus-return-after-collapse.png"));
      record(
        "focus inside live activity returns to the turn's own disclosure on automatic collapse (not body)",
        !focusTurn.timedOut && liveFocus.isActive && liveFocus.insideThinking && !afterFocus.onBody && afterFocus.isTurnToggle,
        JSON.stringify({ liveFocus, afterFocus }),
      );
      await context.close();
    } catch (err) {
      record("focus inside live activity returns to the turn's own disclosure on automatic collapse (not body)", false, String(err));
    }

    // ---- Reading above the tail: scrolling up during a live turn must not
    // snap back to the bottom when the turn completes. ----
    try {
      // A short viewport so the live activity genuinely overflows the
      // transcript panel (at 1024x768 the showcase's live content — a couple
      // of thought lines plus one collapsed child summary row — fits without
      // scrolling, which would make this check trivially true).
      context = await browser.newContext({ viewport: { width: 1024, height: 400 } });
      page = await context.newPage();
      const agentScroll = await launchAgent("Journey ScrollAnchor");
      await page.goto(`${BASE_URL}/agent/${agentScroll}`);
      await prompt(agentScroll, "go");
      await page.waitForSelector(".transcript-view", { timeout: 10000 });
      await sleep(1000); // let live activity accumulate enough height to overflow
      const before = await page.evaluate(() => {
        const el = document.querySelector(".transcript-view");
        const anchor = document.querySelector(".user-message");
        if (!el || !anchor) return null;
        el.scrollTop = 0;
        const containerRect = el.getBoundingClientRect();
        const anchorRect = anchor.getBoundingClientRect();
        return {
          scrollTop: el.scrollTop,
          overflowing: el.scrollHeight > el.clientHeight + 10,
          anchorVisible: anchorRect.top >= containerRect.top - 2 && anchorRect.top <= containerRect.bottom,
        };
      });
      const scrollTurn = await expectTurnEnd("reading-above-tail scenario turn completed", agentScroll, 0);
      await sleep(400);
      const after = await page.evaluate(() => {
        const el = document.querySelector(".transcript-view");
        const anchor = document.querySelector(".user-message");
        if (!el || !anchor) return null;
        const containerRect = el.getBoundingClientRect();
        const anchorRect = anchor.getBoundingClientRect();
        return {
          scrollTop: el.scrollTop,
          anchorVisible: anchorRect.top >= containerRect.top - 2 && anchorRect.top <= containerRect.bottom,
        };
      });
      await shot(page, join(OUT_DIR, "13-reading-above-tail.png"));
      record(
        "scrolling up during a live turn keeps its position and anchor in view after completion (does not jump to the tail)",
        !scrollTurn.timedOut
          && !!before?.overflowing
          && before?.scrollTop === 0
          && before?.anchorVisible === true
          && after?.scrollTop === 0
          && after?.anchorVisible === true,
        JSON.stringify({ before, after }),
      );
      await context.close();
    } catch (err) {
      record("scrolling up during a live turn keeps its position and anchor in view after completion (does not jump to the tail)", false, String(err));
    }

    // ---- Following at bottom still sticks to bottom after completion. ----
    try {
      context = await browser.newContext({ viewport: { width: 1024, height: 768 } });
      page = await context.newPage();
      const agentBottom = await launchAgent("Journey FollowBottom");
      await page.goto(`${BASE_URL}/agent/${agentBottom}`);
      await prompt(agentBottom, "go");
      await page.waitForSelector(".transcript-view", { timeout: 10000 });
      const bottomTurn = await expectTurnEnd("follow-at-bottom scenario turn completed", agentBottom, 0);
      await sleep(400);
      const gapFromBottom = await page.evaluate(() => {
        const el = document.querySelector(".transcript-view");
        if (!el) return null;
        return el.scrollHeight - el.scrollTop - el.clientHeight;
      });
      await shot(page, join(OUT_DIR, "14-follow-bottom-after-completion.png"));
      record(
        "following at bottom still sticks to bottom after completion",
        !bottomTurn.timedOut && gapFromBottom !== null && gapFromBottom < 24,
        JSON.stringify({ gapFromBottom }),
      );
      await context.close();
    } catch (err) {
      record("following at bottom still sticks to bottom after completion", false, String(err));
    }

    // ---- Viewport sweep (1600x1000) ----
    context = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
    page = await context.newPage();
    const agentWide = await launchAgent("Journey Wide");
    await page.goto(`${BASE_URL}/agent/${agentWide}`);
    await prompt(agentWide, "go");
    await expectTurnEnd("wide-viewport (1600x1000) turn completed", agentWide, 0);
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
      await expectTurnEnd(`${skin.name} skin turn completed`, a, 0);
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
    const rmTurn = await expectTurnEnd("reduced-motion turn completed", agentRM, 0);
    await sleep(300);
    await shot(page, join(OUT_DIR, "10-reduced-motion-completed.png"));
    const rmCollapsed = await page.evaluate(() => {
      const turn = document.querySelector('[data-ui="turn-activity"][data-state="collapsed"]');
      const toggle = turn?.querySelector(".turn-activity-toggle button");
      return { found: !!turn, toggleText: toggle?.textContent || null };
    });
    record(
      "reduced-motion pass renders collapsed Show activity disclosure",
      !rmTurn.timedOut && rmCollapsed.found && rmCollapsed.toggleText?.includes("Show activity"),
      JSON.stringify(rmCollapsed),
    );
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
