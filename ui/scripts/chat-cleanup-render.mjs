// Rendered evidence for the chat-cleanup change set: Commands tab removal, web
// links opening in a new tab with a right-click "Open in new tab"/"Copy link"
// menu (composing with "Annotate whole event" where annotations are enabled),
// and readable GFM tables (`div.markdown-table`). Drives Vite + the real UI
// source in Chromium; every /api call and the event stream are stubbed, so no
// Chuck server is needed (same pattern as scripts/phone-render.mjs and
// scripts/room-render.mjs).
//
//   node scripts/chat-cleanup-render.mjs [outDir]
//
// Writes one screenshot per check group, plus report.json, to outDir (default
// os.tmpdir()/chat-cleanup-render-<ts>). Exits 1 if any check fails.
import { mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createServer } from "vite";
import { chromium, devices } from "playwright";

const OUT_DIR = process.argv[2] || join(tmpdir(), `chat-cleanup-render-${Date.now()}`);
mkdirSync(OUT_DIR, { recursive: true });

const PORT = 5198;
const BASE = `http://localhost:${PORT}`;
const AT = "2026-10-08T09:00:00Z";
const SKINS = ["", "sky-grove", "studio"];
const skinLabel = (skin) => skin || "core";

const checks = [];
function record(name, pass, detail) {
  checks.push({ name, pass: Boolean(pass), detail: detail === undefined ? undefined : String(detail) });
  console.log(`${pass ? "PASS" : "FAIL"} ${name}${detail !== undefined ? ` — ${detail}` : ""}`);
}

// ---- Fixture content ----------------------------------------------------

// A deterministic 180-char URL with no hyphen/slash break opportunities after
// the scheme, so it is one unbreakable run: a browser's default `td` wrapping
// cannot shrink it to fit, and the column (and so the table) genuinely
// overflows its box — proving the table scrolls locally rather than widening
// the page (FS-03.R79, TS-08.R108).
const LONG_URL = `https://example.com/${"x".repeat(170)}`;

const TABLE_MD = [
  "| Name | Status | Link |",
  "|:---|:---:|---:|",
  `| Alpha | \`ok\` | [${LONG_URL}](${LONG_URL}) |`,
  "| Beta | `warn` | [short](https://example.com/b) |",
  "| Gamma | `ok` | [short](https://example.com/g) |",
  "| Delta | `fail` | [short](https://example.com/d) |",
].join("\n");

const ASSISTANT_TEXT = [
  "Check [Example docs](https://example.com/docs) for details, open [notes](docs/notes.md), or email [support](mailto:support@example.com).",
  "",
  TABLE_MD,
].join("\n");

const NOTES_MD = TABLE_MD;

const event = (seq, type, data) => ({ agent_id: "a1", seq, type, ts: AT, data });
const TRANSCRIPT = [event(1, "assistant_text", { text: ASSISTANT_TEXT })];

function makeAgent(id, overrides = {}) {
  return {
    agent_id: id, name: id, role: "implementer", project: "demo", backend: "claude", model: "m",
    fast: false, fast_available: false, steering_available: true, interface: "chat", created_at: AT,
    running: false, state: "idle", detail: "", context_pct: 0, updated_at: 0, archived: false,
    ...overrides,
  };
}
const AGENT_A1 = makeAgent("a1", { name: "Chat cleanup demo" });
const AGENT_A2 = makeAgent("a2", { name: "Archived demo", archived: true });

function fileContentFor(id, url) {
  const path = url.searchParams.get("path") || "";
  return {
    agent_id: id, path, size: NOTES_MD.length, mod_time: AT,
    line_count: NOTES_MD.split("\n").length, content: NOTES_MD, truncated: false, language: "markdown",
  };
}

// ---- Shared Playwright plumbing -----------------------------------------

async function hydrate(context, agents) {
  await context.addInitScript((hydrated) => {
    // The desktop SSE client prefers a SharedWorker transport and only falls
    // back to a plain EventSource when none is available (src/api/sse.ts).
    // Removing it keeps the override below on the one path the app actually
    // takes, instead of racing a real SharedWorker this script cannot stub.
    delete window.SharedWorker;
    window.EventSource = class extends EventTarget {
      constructor() {
        super();
        setTimeout(() => {
          this.onopen?.();
          for (const data of hydrated) this.dispatchEvent(new MessageEvent("state_update", { data: JSON.stringify(data) }));
        });
      }
      close() {}
    };
  }, [...agents.map((data) => ({ agent_id: data.agent_id, data })), { agent_id: "__hydrated__", data: { hydrated: true } }]);
}

function apiHandler(skin, { perRow = 2 } = {}) {
  return (route) => {
    const req = route.request();
    const url = new URL(req.url());
    const path = url.pathname;
    if (path === "/api/config") return route.fulfill({ json: { appearance_skin: skin, onboarding_complete: true } });
    if (path === "/api/backends") return route.fulfill({ json: { backends: {} } });
    if (path === "/api/projects") return route.fulfill({ json: { demo: { title: "Demo", archived: false, color: [90, 130, 210] } } });
    if (path === "/api/layout") {
      if (req.method() === "PUT") return route.fulfill({ json: JSON.parse(req.postData() || "{}") });
      return route.fulfill({ json: { order: [], density: { perRow, gap: 16 }, expanded: ["a1"] } });
    }
    if (path === "/api/tasks") return route.fulfill({ json: { tasks: [] } });
    if (path === "/api/think-tanks") return route.fulfill({ json: { rooms: [], clipped: false } });
    if (path === "/api/sessions/a1/transcript") return route.fulfill({ json: { agent_id: "a1", has_more: false, events: TRANSCRIPT } });
    if (path === "/api/sessions/a2/transcript") return route.fulfill({ json: { agent_id: "a2", has_more: false, events: TRANSCRIPT } });
    if (path === "/api/sessions/a1/file") return route.fulfill({ json: fileContentFor("a1", url) });
    if (path === "/api/sessions/a2/file") return route.fulfill({ json: fileContentFor("a2", url) });
    if (path === "/api/sessions/a1/prompt") return route.fulfill({ status: 404, json: { error: { code: "not_found", message: "none" } } });
    if (path === "/api/sessions/a1/available-commands") return route.fulfill({ json: { commands: [] } });
    if (path === "/api/remote/home") return route.fulfill({ json: { needs_you: [], active_runs: [] } });
    return route.fulfill({ json: {} });
  };
}

async function exampleRoute(route) {
  return route.fulfill({ contentType: "text/html", body: "<!doctype html><title>Example</title><body>Offline example.com stand-in.</body>" });
}

// tableMetrics reads the style facts the checks care about from the first
// .markdown-table under `scope` (a Page or Locator-scoped root).
async function tableMetrics(scope) {
  const wrap = scope.locator(".markdown-table").first();
  await wrap.waitFor();
  return wrap.evaluate((el) => {
    const table = el.querySelector("table");
    const th = table.querySelector("th");
    const bodyRows = [...table.querySelectorAll("tbody tr")];
    const headerStyle = th ? getComputedStyle(th) : null;
    const rowStyle = bodyRows[1] ? getComputedStyle(bodyRows[1]) : null;
    const firstRowCells = bodyRows[0] ? [...bodyRows[0].querySelectorAll("td")] : [];
    const cellStyle = firstRowCells[0] ? getComputedStyle(firstRowCells[0]) : null;
    return {
      scrollWidth: el.scrollWidth,
      clientWidth: el.clientWidth,
      headerBorderWidth: headerStyle ? parseFloat(headerStyle.borderBottomWidth) : 0,
      headerBorderColor: headerStyle ? headerStyle.borderBottomColor : "",
      rowBorderWidth: rowStyle ? parseFloat(rowStyle.borderTopWidth) : 0,
      tdPaddingLeft: cellStyle ? parseFloat(cellStyle.paddingLeft) : 0,
      tdPaddingRight: cellStyle ? parseFloat(cellStyle.paddingRight) : 0,
      userSelect: cellStyle ? cellStyle.userSelect : "",
      aligns: firstRowCells.map((cell) => getComputedStyle(cell).textAlign),
    };
  });
}

function isTransparent(color) {
  return !color || color === "transparent" || /rgba\([^)]*,\s*0\s*\)/.test(color);
}

async function noPageOverflow(page) {
  return page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);
}

async function menuItems(page) {
  const menu = page.locator('[role="menu"]');
  await menu.waitFor();
  const count = await page.locator('[role="menu"]').count();
  const items = await menu.locator('[data-slot="item"]').allInnerTexts();
  return { menu, count, items };
}

// ---- Desktop full chat ----------------------------------------------------

async function runDesktopChat(browser, skin, width, height) {
  const label = `${skinLabel(skin)}@${width}x${height}`;
  const context = await browser.newContext({ viewport: { width, height } });
  await context.grantPermissions(["clipboard-read", "clipboard-write"], { origin: BASE });
  await hydrate(context, [AGENT_A1, AGENT_A2]);
  await context.route((u) => u.pathname.startsWith("/api/"), apiHandler(skin));
  await context.route("https://example.com/**", exampleRoute);
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (err) => errors.push(err.message));
  await page.goto(`${BASE}/agent/a1?tab=commands`);

  const shown = await page.locator('[role="tab"]').first().waitFor({ timeout: 10_000 }).then(() => true, () => false);
  if (!shown) {
    record(`${label}: chat view rendered`, false, "no tabs appeared");
    await context.close();
    return;
  }

  const tabs = await page.locator('[role="tab"]').allInnerTexts();
  record(`${label}: tabs exactly Transcript, Files`, JSON.stringify(tabs) === JSON.stringify(["Transcript", "Files"]), JSON.stringify(tabs));
  const selected = await page.locator('[role="tab"][aria-selected="true"]').innerText();
  record(`${label}: ?tab=commands selects Transcript`, selected === "Transcript", selected);
  const transcriptText = (await page.locator(".transcript-view").innerText()).trim();
  record(`${label}: transcript panel non-empty`, transcriptText.length > 0, `len=${transcriptText.length}`);

  await page.locator(".markdown-table").first().waitFor({ timeout: 10_000 });
  const metrics = await tableMetrics(page);
  record(`${label}: thead border-bottom >=1px non-transparent`, metrics.headerBorderWidth >= 1 && !isTransparent(metrics.headerBorderColor), `${metrics.headerBorderWidth}px ${metrics.headerBorderColor}`);
  record(`${label}: tbody tr+tr border-top >=1px`, metrics.rowBorderWidth >= 1, `${metrics.rowBorderWidth}px`);
  record(`${label}: td padding-left/right >=12px`, metrics.tdPaddingLeft >= 12 && metrics.tdPaddingRight >= 12, `${metrics.tdPaddingLeft}/${metrics.tdPaddingRight}`);
  const aligns = metrics.aligns;
  const alignOk = (aligns[0] === "left" || aligns[0] === "start") && aligns[1] === "center" && aligns[2] === "right";
  record(`${label}: column text-align left/center/right`, alignOk, JSON.stringify(aligns));
  record(`${label}: td user-select not none`, metrics.userSelect !== "none", metrics.userSelect);

  const overflowOk = await noPageOverflow(page);
  record(`${label}: no page overflow`, overflowOk, `doc scrollWidth vs innerWidth`);
  record(`${label}: markdown-table scrollWidth vs clientWidth`, true, `${metrics.scrollWidth} vs ${metrics.clientWidth}`);

  await page.screenshot({ path: join(OUT_DIR, `chat-${skinLabel(skin)}-${width}.png`), fullPage: true });

  // Left-click the paragraph web link opens a new tab; original page unchanged.
  const webLink = page.locator('.transcript-view a[href="https://example.com/docs"]').first();
  await webLink.scrollIntoViewIfNeeded();
  const [popup] = await Promise.all([context.waitForEvent("page"), webLink.click()]);
  await popup.waitForLoadState().catch(() => {});
  record(`${label}: left-click web link opens new tab`, popup.url() === "https://example.com/docs", popup.url());
  record(`${label}: original page URL unchanged`, page.url().startsWith(`${BASE}/agent/a1`), page.url());
  await popup.close();

  // Right-click the web link: one menu with link + annotation actions.
  await webLink.click({ button: "right" });
  let { menu, count, items } = await menuItems(page);
  record(`${label}: right-click web link shows exactly one menu`, count === 1, `count=${count}`);
  record(`${label}: menu has Open in new tab, Copy link, Annotate whole event`, JSON.stringify(items) === JSON.stringify(["Open in new tab", "Copy link", "Annotate whole event"]), JSON.stringify(items));
  await menu.getByText("Copy link", { exact: true }).click();
  const clip = await page.evaluate(() => navigator.clipboard.readText());
  record(`${label}: Copy link writes the href to the clipboard`, clip === "https://example.com/docs", clip);

  await webLink.click({ button: "right" });
  ({ menu } = await menuItems(page));
  const [popup2] = await Promise.all([context.waitForEvent("page"), menu.getByText("Open in new tab", { exact: true }).click()]);
  await popup2.waitForLoadState().catch(() => {});
  record(`${label}: right-click menu's Open in new tab opens a new page`, popup2.url() === "https://example.com/docs", popup2.url());
  await popup2.close();

  // Right-click the link inside the table shows link actions too.
  const tableLink = page.locator(`.markdown-table a[href="${LONG_URL}"]`).first();
  await tableLink.scrollIntoViewIfNeeded();
  await tableLink.click({ button: "right" });
  ({ items } = await menuItems(page));
  record(`${label}: right-click link inside table shows link actions`, items.includes("Open in new tab") && items.includes("Copy link"), JSON.stringify(items));
  await page.keyboard.press("Escape");

  // Click the local file link: file viewer opens, no new page, styled table.
  const pagesBefore = context.pages().length;
  const fileLink = page.locator(".transcript-view button.file-link", { hasText: "notes" }).first();
  await fileLink.scrollIntoViewIfNeeded();
  await fileLink.click();
  const viewer = page.locator('[data-ui="file-viewer"]');
  const viewerShown = await viewer.waitFor({ timeout: 10_000 }).then(() => true, () => false);
  record(`${label}: local file link opens the file viewer`, viewerShown);
  record(`${label}: opening a local file link does not open a new page`, context.pages().length === pagesBefore, `${context.pages().length} vs ${pagesBefore}`);
  if (viewerShown) {
    const viewerMetrics = await tableMetrics(viewer);
    record(`${label}: file viewer rendered table matches transcript table styling`, viewerMetrics.headerBorderWidth >= 1 && viewerMetrics.rowBorderWidth >= 1 && viewerMetrics.tdPaddingLeft >= 12, JSON.stringify(viewerMetrics));
    record(`${label}: no page overflow with file viewer open`, await noPageOverflow(page));
    await page.screenshot({ path: join(OUT_DIR, `chat-file-${skinLabel(skin)}-${width}.png`), fullPage: true });
  }

  record(`${label}: no uncaught page errors`, errors.length === 0, errors.join(" | "));
  await context.close();
}

// ---- Dashboard expanded (narrow) pane -------------------------------------

async function runDashboard(browser, skin) {
  const label = `${skinLabel(skin)} dashboard`;
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  await hydrate(context, [AGENT_A1]);
  await context.route((u) => u.pathname.startsWith("/api/"), apiHandler(skin, { perRow: 4 }));
  await context.route("https://example.com/**", exampleRoute);
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (err) => errors.push(err.message));
  await page.goto(`${BASE}/project/demo`);

  const pane = page.locator('[data-slot="chat-pane"]').first();
  const shown = await pane.waitFor({ timeout: 10_000 }).then(() => true, () => false);
  if (!shown) {
    record(`${label}: expanded chat pane rendered`, false);
    await context.close();
    return;
  }
  const tableLoc = pane.locator(".markdown-table").first();
  await tableLoc.waitFor({ timeout: 10_000 });
  const metrics = await tableLoc.evaluate((el) => ({ scrollWidth: el.scrollWidth, clientWidth: el.clientWidth }));
  record(`${label}: table wrapper scrolls locally`, metrics.scrollWidth > metrics.clientWidth, JSON.stringify(metrics));
  record(`${label}: no page overflow`, await noPageOverflow(page));
  const paneTabs = await pane.locator('[role="tab"]').allInnerTexts();
  record(`${label}: pane has no Commands tab`, !paneTabs.includes("Commands"), JSON.stringify(paneTabs));
  await pane.screenshot({ path: join(OUT_DIR, `dashboard-${skinLabel(skin)}.png`) });
  record(`${label}: no uncaught page errors`, errors.length === 0, errors.join(" | "));
  await context.close();
}

// ---- Archived agent page ---------------------------------------------------

async function runArchive(browser, skin) {
  const label = `${skinLabel(skin)} archive`;
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  await context.grantPermissions(["clipboard-read", "clipboard-write"], { origin: BASE });
  await hydrate(context, [AGENT_A2]);
  await context.route((u) => u.pathname.startsWith("/api/"), apiHandler(skin));
  await context.route("https://example.com/**", exampleRoute);
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (err) => errors.push(err.message));
  await page.goto(`${BASE}/archive/a2`);

  const shown = await page.locator(".markdown-table").first().waitFor({ timeout: 10_000 }).then(() => true, () => false);
  if (!shown) {
    record(`${label}: archived transcript table rendered`, false);
    await context.close();
    return;
  }
  const metrics = await tableMetrics(page);
  record(`${label}: table styled`, metrics.headerBorderWidth >= 1 && metrics.rowBorderWidth >= 1 && metrics.tdPaddingLeft >= 12, JSON.stringify(metrics));

  const webLink = page.locator('a[href="https://example.com/docs"]').first();
  const target = await webLink.getAttribute("target");
  record(`${label}: web link target _blank`, target === "_blank", target);

  await webLink.click({ button: "right" });
  const { items } = await menuItems(page);
  record(`${label}: right-click menu works`, items.includes("Open in new tab") && items.includes("Copy link"), JSON.stringify(items));
  await page.keyboard.press("Escape");

  await page.screenshot({ path: join(OUT_DIR, `archive-${skinLabel(skin)}.png`), fullPage: true });
  record(`${label}: no uncaught page errors`, errors.length === 0, errors.join(" | "));
  await context.close();
}

// ---- Phone (core only) -----------------------------------------------------

async function runPhone(browser) {
  const context = await browser.newContext({ ...devices["iPhone 13"] });
  await hydrate(context, [AGENT_A1]);
  await context.route((u) => u.pathname.startsWith("/api/"), apiHandler(""));
  await context.route("https://example.com/**", exampleRoute);
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (err) => errors.push(err.message));
  await page.goto(`${BASE}/remote.html`);
  // Vite serves the desktop entry for deep links, so route inside the phone app
  // the same way scripts/phone-render.mjs does.
  await page.evaluate(() => {
    history.pushState(null, "", "/agent/a1");
    window.dispatchEvent(new PopStateEvent("popstate"));
  });

  const shown = await page.getByRole("list", { name: "Conversation" }).waitFor({ timeout: 10_000 }).then(() => true, () => false);
  if (!shown) {
    record("phone: conversation rendered", false);
    await context.close();
    return;
  }
  const tabs = await page.locator('[role="tab"]').allInnerTexts();
  record("phone: tabs exactly Chat, Files, Manage", JSON.stringify(tabs) === JSON.stringify(["Chat", "Files", "Manage"]), JSON.stringify(tabs));

  await page.locator(".markdown-table").first().waitFor({ timeout: 10_000 });
  const metrics = await page.locator(".markdown-table").first().evaluate((el) => ({ scrollWidth: el.scrollWidth, clientWidth: el.clientWidth }));
  record("phone: table wrapper scrolls locally", metrics.scrollWidth > metrics.clientWidth, JSON.stringify(metrics));
  record("phone: no page overflow", await noPageOverflow(page));

  const webLink = page.locator('a[href="https://example.com/docs"]').first();
  const target = await webLink.getAttribute("target");
  record("phone: web link target _blank", target === "_blank", target);

  await page.screenshot({ path: join(OUT_DIR, "phone-core.png"), fullPage: true });
  record("phone: no uncaught page errors", errors.length === 0, errors.join(" | "));
  await context.close();
}

// ---- Main -------------------------------------------------------------------

async function main() {
  const server = await createServer({ server: { port: PORT, strictPort: true }, logLevel: "error" });
  await server.listen();
  const browser = await chromium.launch().catch(() => chromium.launch({ executablePath: "/opt/pw-browsers/chromium" }));
  try {
    for (const skin of SKINS) {
      await runDesktopChat(browser, skin, 1024, 800);
      await runDesktopChat(browser, skin, 1440, 900);
      await runDashboard(browser, skin);
      await runArchive(browser, skin);
      if (skin === "") await runPhone(browser);
    }
  } finally {
    await browser.close();
    await server.close();
  }
  writeFileSync(join(OUT_DIR, "report.json"), JSON.stringify(checks, null, 2));
  console.log(`\nwrote ${checks.length} checks, ${checks.filter((c) => !c.pass).length} failing`);
  console.log(`report: ${join(OUT_DIR, "report.json")}`);
  console.log(`out dir: ${OUT_DIR}`);
  if (checks.some((c) => !c.pass)) process.exitCode = 1;
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
