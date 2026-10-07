// Real-binary Think Tank rendered journey (FS-21.A33-A37, TS-08.R96-R99).
//
// Start the isolated fixture after `make embed`:
//   go run -tags sqlite_fts5 ./scripts/stress-fixture -port 4411 -scenario hold_turn -hold-file /tmp/chuck-room-hold
// Then run:
//   node ui/scripts/room-journey.mjs http://127.0.0.1:4411 <outDir> /tmp/chuck-room-hold <fixtureHome>
//
// The harness stages explicit room contributions through each isolated agent's
// authenticated MCP session while fake ACP holds the provider turn. This proves
// the host's tool, REST and UI boundaries; real-provider tool use remains a
// separate credentialed smoke. Supply only the fixture-owned home and hold path.
import { mkdirSync, readFileSync, unlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { chromium } from "playwright";

const BASE_URL = process.argv[2] || "http://127.0.0.1:4411";
const OUT_DIR = process.argv[3] || join(tmpdir(), `room-journey-${Date.now()}`);
const HOLD_FILE = process.argv[4] || "";
const FIXTURE_HOME = process.argv[5] || "";
mkdirSync(OUT_DIR, { recursive: true });

const checks = [];
function record(name, pass, detail = "") {
  checks.push({ name, pass, detail });
  console.log(`${pass ? "PASS" : "FAIL"} ${name}${detail ? ` — ${detail}` : ""}`);
}
function blocked(name, detail) {
  checks.push({ name, pass: null, blocked: true, detail });
  console.log(`BLOCKED ${name} — ${detail}`);
}
async function api(path, options = {}) {
  const response = await fetch(BASE_URL + path, {
    ...options,
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
  });
  const text = await response.text();
  let body;
  try { body = text ? JSON.parse(text) : undefined; } catch { body = text; }
  if (!response.ok) throw new Error(`${path} -> ${response.status}: ${text}`);
  return body;
}
const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
async function waitFor(fn, timeout = 12000, interval = 250) {
  const deadline = Date.now() + timeout;
  let last;
  while (Date.now() < deadline) {
    last = await fn();
    if (last) return last;
    await wait(interval);
  }
  throw new Error(`timed out${last ? ` (last=${JSON.stringify(last)})` : ""}`);
}
async function launch(name, project = "stress") {
  const result = await api("/api/sessions", { method: "POST", body: JSON.stringify({
    role: "implementer", project, backend: "claude", model: "haiku", name, interface: "chat",
  }) });
  return result.agent;
}
async function screenshot(page, name) {
  const path = join(OUT_DIR, name);
  await page.screenshot({ path, fullPage: true });
  console.log(`screenshot: ${path}`);
}
async function setSkin(value) {
  await api("/api/config", { method: "PUT", body: JSON.stringify({ appearance_skin: value }) });
}
function commandID() { return `room-journey-${Date.now()}-${Math.random().toString(36).slice(2)}`; }
function removeFile(path) {
  try { unlinkSync(path); } catch (error) { if (error.code !== "ENOENT") throw error; }
}

let mcpID = 0;
function mcpToken(agentID) {
  if (!FIXTURE_HOME) throw new Error("fixture home is required for MCP staging");
  const config = JSON.parse(readFileSync(join(FIXTURE_HOME, "mcp", `${agentID}.mcp.json`), "utf8"));
  const entry = config.mcpServers?.["chuck-messaging"];
  const token = entry?.headers?.["X-Chuck-Token"];
  if (!entry?.url || !token) throw new Error(`missing isolated MCP config for ${agentID}`);
  return { url: entry.url, token };
}
function mcpPayload(text) {
  const line = text.split(/\r?\n/).filter((item) => item.startsWith("data:")).at(-1)?.slice(5).trim();
  try { return JSON.parse(line || text); } catch { throw new Error(`invalid MCP response: ${text.slice(0, 500)}`); }
}
async function mcpRequest(agentID, method, params, sessionID) {
  const auth = mcpToken(agentID);
  const headers = { "Content-Type": "application/json", Accept: "application/json, text/event-stream", "MCP-Protocol-Version": "2025-06-18", "X-Chuck-Token": auth.token };
  if (sessionID) headers["Mcp-Session-Id"] = sessionID;
  const request = { jsonrpc: "2.0", method, params };
  if (!method.startsWith("notifications/")) request.id = ++mcpID;
  const response = await fetch(auth.url, { method: "POST", headers, body: JSON.stringify(request) });
  const text = await response.text();
  if (!response.ok && response.status !== 202) throw new Error(`MCP ${method} -> ${response.status}: ${text}`);
  return { payload: text ? mcpPayload(text) : null, sessionID: response.headers.get("mcp-session-id") || sessionID };
}
async function mcpCall(agentID, name, args, sessionID) {
  const result = await mcpRequest(agentID, "tools/call", { name, arguments: args }, sessionID);
  const item = result.payload?.result?.content?.find((content) => content.type === "text");
  if (result.payload?.result?.isError) throw new Error(`MCP ${name} returned an error: ${item?.text || "unknown"}`);
  let value = item?.text;
  try { value = JSON.parse(value); } catch { /* text result */ }
  return { value, sessionID: result.sessionID };
}
async function mcpReadAndSubmit(agentID, roomID, message) {
  let sessionID;
  const initialized = await mcpRequest(agentID, "initialize", { protocolVersion: "2025-06-18", capabilities: {}, clientInfo: { name: "room-journey", version: "1" } });
  sessionID = initialized.sessionID;
  await mcpRequest(agentID, "notifications/initialized", {}, sessionID);
  let cursor = "";
  let read;
  do {
    const call = await mcpCall(agentID, "read_think_tank", cursor ? { cursor } : {}, sessionID);
    read = call.value;
    sessionID = call.sessionID;
    cursor = read?.next_cursor || "";
  } while (read && !read.complete);
  if (!read?.turn_token || !read?.read_receipt) throw new Error(`MCP read did not produce a turn token for ${agentID}`);
  const submitted = await mcpCall(agentID, "submit_think_tank_turn", { turn_token: read.turn_token, disposition: "reply", message, read_receipt: read.read_receipt }, sessionID);
  return submitted.value;
}

let browser;
try {
  await api("/api/health");
  const existing = await launch("Existing Room Analyst");
  const second = await launch("Existing Room Reviewer");
  const room = await api("/api/think-tanks", {
    method: "POST",
    body: JSON.stringify({
      command_id: commandID(),
      title: "Release cache council",
      goal: "Choose a cache invalidation strategy for regional failover, including a long explicit stale-read bound and recovery plan.",
      origin_project: "stress",
      openings: true,
      participants: [
        { agent_id: existing.agent_id, limit: 2, may_leave: true },
        { agent_id: second.agent_id, limit: 2, may_leave: true },
        { new: { role: "implementer", project: "stress", backend: "claude", model: "haiku", name: "New Room Architect" }, limit: 2, may_leave: true },
      ],
      judge: { role: "implementer", project: "stress", backend: "claude", model: "haiku", name: "Room Judge" },
    }),
  });
  const roomID = room.room_id;
  const detail = await waitFor(async () => {
    const current = await api(`/api/think-tanks/${encodeURIComponent(roomID)}`);
    return current.members?.length === 3 ? current : null;
  });
  record("titled room created with mixed existing/new roster", detail.title === "Release cache council" && detail.members.length === 3 && detail.members.some((m) => m.setup_state === "pending" || m.name === "New Room Architect"), JSON.stringify({ roomID, title: detail.title, members: detail.members.map((m) => ({ name: m.name, role: m.role, project: m.project, setup: m.setup_state })) }));
  const opening = await waitFor(async () => {
    const current = await api(`/api/think-tanks/${encodeURIComponent(roomID)}`);
    return current.phase === "openings" && current.active_attempts?.length === 3 ? current : null;
  }, 20000);
  record("opening is visibly held by fake provider", opening.phase === "openings" && opening.active_attempts?.length > 0, `phase=${opening.phase} active=${opening.active_attempts?.length ?? 0}`);

  const newMember = opening.members.find((m) => m.name === "New Room Architect");
  const targetMember = opening.members.find((m) => m.agent_id === existing.agent_id);
  if (!newMember || !targetMember) throw new Error("created room did not expose expected members");

  // Exercise durable live controls and shared addressed input through the real API.
  const raised = await api(`/api/think-tanks/${encodeURIComponent(roomID)}/participants/${encodeURIComponent(targetMember.agent_id)}/turn-limit`, {
    method: "POST", body: JSON.stringify({ command_id: commandID(), expected_limit: targetMember.limit, limit: targetMember.limit + 2 }),
  });
  const raisedMember = raised.members.find((m) => m.agent_id === targetMember.agent_id);
  record("live ceiling increase persists while opening is held", raisedMember?.limit === targetMember.limit + 2 && raisedMember?.completed === targetMember.completed, JSON.stringify({ before: targetMember.limit, after: summarizeMember(raisedMember) }));
  const mentionedText = `@${targetMember.name} please check stale reads before the next room turn.`;
  const end = new TextEncoder().encode(mentionedText.slice(0, targetMember.name.length + 1)).length;
  const msg = await api(`/api/think-tanks/${encodeURIComponent(roomID)}/messages`, {
    method: "POST", body: JSON.stringify({ command_id: commandID(), body: mentionedText, mentions: [{ agent_id: targetMember.agent_id, start: 0, end }] }),
  });
  const afterMessage = await api(`/api/think-tanks/${encodeURIComponent(roomID)}`);
  const entry = (await api(`/api/think-tanks/${encodeURIComponent(roomID)}/entries`)).entries.find((e) => e.input_id === msg.input_id || e.seq === msg.published_seq);
  const pendingInput = afterMessage.pending?.find((input) => input.input_id === msg.input_id);
  const contextText = JSON.stringify(entry?.context ?? pendingInput?.context ?? "");
  record("shared @ mention is persisted with addressee context", contextText.includes(targetMember.agent_id) && contextText.includes(targetMember.name), JSON.stringify({ input_id: msg.input_id, published_seq: msg.published_seq, context: entry?.context ?? pendingInput?.context }));
  record("room detail remains readable after message", afterMessage.pending?.length > 0 || afterMessage.active_attempts?.length > 0, `pending=${afterMessage.pending?.length ?? 0} active=${afterMessage.active_attempts?.length ?? 0}`);

  let openingStaged = false;
  if (FIXTURE_HOME && HOLD_FILE) {
    try {
      const held = await waitFor(async () => {
        const current = await api(`/api/think-tanks/${encodeURIComponent(roomID)}`);
        return current.active_attempts?.length === 3 ? current : null;
      }, 20000);
      const hiddenBeforeRelease = (await api(`/api/think-tanks/${encodeURIComponent(roomID)}/entries`)).entries || [];
      record("opening bodies stay hidden before the barrier releases", hiddenBeforeRelease.length === 0, `entries=${hiddenBeforeRelease.length}`);
      for (const attempt of held.active_attempts) {
        await mcpReadAndSubmit(attempt.agent_id, roomID, `Opening contribution from ${attempt.agent_id}: compare bounded stale-read recovery and operational cost.\n\n## Technical notes\n\nUse a versioned key and a bounded TTL fallback. The read path must preserve **read-your-writes** after regional failover.\n\n| strategy | stale bound | recovery |\n|---|---:|---|\n| versioned key | 0s | bump generation |\n| TTL fallback | 30s | invalidate replica |\n\ncode: resolveCache(ctx, key) -> cache.ReadThrough(ctx, key, Bound{Seconds: 30})`);
      }
      const paused = await api(`/api/think-tanks/${encodeURIComponent(roomID)}/pause`, { method: "POST", body: "{}" });
      record("pause request is retained while openings are held", paused.control === "pause_requested" || paused.control === "paused", `control=${paused.control}`);
      openingStaged = true;
      record("MCP stages every held opening through read and submit", true, `attempts=${held.active_attempts.length}`);
    } catch (error) {
      record("MCP stages every held opening through read and submit", false, String(error));
    }
  } else {
    blocked("MCP stages every held opening through read and submit", "pass fixture home and hold-file paths to use the isolated room-tool tokens");
  }

  let publishedEntries = [];
  if (openingStaged && HOLD_FILE) {
    writeFileSync(HOLD_FILE, "release openings\n");
    const released = await waitFor(async () => {
      const current = await api(`/api/think-tanks/${encodeURIComponent(roomID)}`);
      return current.phase === "discussion" && current.control === "paused" && current.active_attempts?.length === 0 ? current : null;
    }, 20000);
    publishedEntries = (await api(`/api/think-tanks/${encodeURIComponent(roomID)}/entries`)).entries || [];
    const expectedOrder = opening.members.filter((m) => m.role === "participant").map((m) => m.agent_id);
    record("opening publication waits for all peers and preserves configured order", publishedEntries.length >= 3 && expectedOrder.every((id, index) => publishedEntries[index]?.agent_id === id), JSON.stringify({ phase: released.phase, control: released.control, order: publishedEntries.slice(0, 3).map((entry) => entry.agent_id) }));
    unlinkSync(HOLD_FILE);
  }

  browser = await chromium.launch().catch(() => chromium.launch({ executablePath: "/opt/pw-browsers/chromium" }));
  const contexts = [];
  const roomComposerStyles = {};
  for (const skin of [{ name: "core", value: "" }, { name: "sky-grove", value: "sky-grove" }, { name: "studio", value: "studio" }]) {
    await setSkin(skin.value);
    for (const viewport of [{ name: "1024", width: 1024, height: 768 }, { name: "1600", width: 1600, height: 1000 }]) {
      const context = await browser.newContext({ viewport: { width: viewport.width, height: viewport.height } });
      contexts.push(context);
      const page = await context.newPage();
      const consoleErrors = [];
      page.on("console", (m) => { if (m.type() === "error") consoleErrors.push(m.text()); });
      page.on("pageerror", (e) => consoleErrors.push(String(e)));
      await page.goto(`${BASE_URL}/project/stress`);
      await page.getByRole("heading", { name: "Think Tanks" }).waitFor({ timeout: 15000 }).catch(() => {});
      const roomLink = page.getByRole("link", { name: "Release cache council" }).first();
      await roomLink.waitFor({ timeout: 10000 }).catch(() => {});
      record(`origin project shows titled room card (${skin.name}/${viewport.name})`, await roomLink.count() > 0, `url=${page.url()}`);
      if (await roomLink.count()) await roomLink.click();
      await page.waitForURL(new RegExp(`/think-tank/${roomID}`), { timeout: 10000 }).catch(() => {});
      const roomTitle = page.getByText("Release cache council", { exact: true }).first();
      await roomTitle.waitFor({ timeout: 10000 }).catch(() => {});
      record(`room navigation and title (${skin.name}/${viewport.name})`, page.url().includes(`/think-tank/${roomID}`) && await roomTitle.count() > 0, page.url());
      await screenshot(page, `room-${skin.name}-${viewport.name}.png`);
      const composer = page.locator('[data-ui="composer"][data-variant="room"]');
      const geometry = await page.evaluate(() => {
        const form = document.querySelector('[data-ui="composer"][data-variant="room"]');
        const entries = document.querySelector('.think-tank-entries');
        if (!form || !entries) return null;
        const before = form.getBoundingClientRect();
        const beforeTop = entries.scrollTop;
        entries.scrollTop = entries.scrollHeight;
        const after = form.getBoundingClientRect();
        return { anchored: after.bottom <= window.innerHeight + 4 && Math.abs(after.bottom - before.bottom) < 2, scrollable: entries.scrollHeight > entries.clientHeight, moved: entries.scrollTop !== beforeTop, bottom: after.bottom, viewport: window.innerHeight, scrollHeight: entries.scrollHeight, clientHeight: entries.clientHeight };
      }).catch(() => null);
      record(`room composer present and anchored through long discussion (${skin.name}/${viewport.name})`, await composer.count() > 0 && Boolean(geometry?.anchored) && Boolean(geometry?.scrollable) && Boolean(geometry?.moved), JSON.stringify(geometry));
      if (viewport.width === 1024) {
        const slots = await page.evaluate(() => Array.from(document.querySelectorAll('.think-tank-entry[data-speaker-slot]')).map((entry) => ({ slot: entry.getAttribute('data-speaker-slot'), name: entry.querySelector('.think-tank-author')?.textContent || '' })));
        record(`speaker tint slots remain attributed (${skin.name})`, slots.length >= 3 && new Set(slots.slice(0, 3).map((item) => item.slot)).size === 3, JSON.stringify(slots.slice(0, 3)));
      }
      if (viewport.width === 1024) {
        roomComposerStyles[skin.name] = await page.evaluate(() => {
          const form = document.querySelector('[data-ui="composer"][data-variant="room"]');
          const textarea = form?.querySelector("textarea");
          if (!form || !textarea) return null;
          const fs = getComputedStyle(form);
          const ts = getComputedStyle(textarea);
          return { borderRadius: fs.borderRadius, padding: fs.padding, textareaMinHeight: ts.minHeight, textareaPadding: ts.padding };
        });
      }
      if (await composer.count()) {
        const textarea = composer.locator("textarea");
        await textarea.fill("keyboard check");
        await textarea.press("Shift+Enter");
        await textarea.press("Escape");
        record(`room composer keyboard surface (${skin.name}/${viewport.name})`, await textarea.inputValue() === "keyboard check\n", await textarea.inputValue());
        if (skin.value === "" && viewport.width === 1024) {
          await textarea.fill("@");
          const option = page.getByRole("option").first();
          await option.waitFor({ timeout: 4000 }).catch(() => {});
          const optionVisible = await option.count() > 0;
          if (optionVisible) {
            await textarea.press("Enter");
            const picked = await textarea.inputValue();
            record("keyboard @ picker selects the first participant option", picked.startsWith(`@${targetMember.name} `), picked);
            await textarea.press("Enter");
            const persisted = await waitFor(async () => {
              const current = await api(`/api/think-tanks/${encodeURIComponent(roomID)}`);
              const entries = (await api(`/api/think-tanks/${encodeURIComponent(roomID)}/entries`)).entries || [];
              const shared = entries.find((input) => input.body === picked && JSON.stringify(input.context || "").includes(targetMember.agent_id)) || current.pending?.find((input) => input.body === picked && JSON.stringify(input.context || "").includes(targetMember.agent_id));
              return shared ? { current, shared } : null;
            }, 5000);
            record("keyboard-selected mention sends shared input", Boolean(persisted), JSON.stringify({ picked, pending: persisted?.current?.pending?.length ?? 0, published: persisted?.shared?.seq ?? null }));
          } else {
            blocked("keyboard @ picker selects a participant", "the real binary did not expose a participant option");
          }
        }
      }
      record(`no browser errors (${skin.name}/${viewport.name})`, consoleErrors.length === 0, JSON.stringify(consoleErrors));
      await context.close();
    }
  }

  // Participant chat and Think Tank tab are checked against the same real room.
  await setSkin("");
  const participantContext = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const participantPage = await participantContext.newPage();
  await participantPage.goto(`${BASE_URL}/agent/${existing.agent_id}`);
  const tab = participantPage.getByRole("tab", { name: "Think Tank" });
  await tab.waitFor({ timeout: 10000 }).catch(() => {});
  record("participant chat exposes Think Tank tab", await tab.count() > 0, participantPage.url());
  if (await tab.count()) {
    await tab.click();
    record("Think Tank tab exposes title and member identity", await participantPage.getByText("Release cache council", { exact: true }).count() > 0 && await participantPage.getByText("Existing Room Analyst", { exact: false }).count() > 0, "tab selected");
  }
  const ordinaryStyle = await participantPage.evaluate(() => {
    const form = document.querySelector('[data-ui="composer"]:not([data-variant="room"])');
    const textarea = form?.querySelector("textarea");
    if (!form || !textarea) return null;
    const fs = getComputedStyle(form);
    const ts = getComputedStyle(textarea);
    return { borderRadius: fs.borderRadius, padding: fs.padding, textareaMinHeight: ts.minHeight, textareaPadding: ts.padding };
  });
  record("room and ordinary composer geometry share the same surface (core)", Boolean(roomComposerStyles.core && ordinaryStyle) && JSON.stringify(roomComposerStyles.core) === JSON.stringify(ordinaryStyle), JSON.stringify({ room: roomComposerStyles.core, ordinary: ordinaryStyle }));
  await participantContext.close();
  for (const skin of [{ name: "sky-grove", value: "sky-grove" }, { name: "studio", value: "studio" }]) {
    await setSkin(skin.value);
    const skinContext = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const skinPage = await skinContext.newPage();
    await skinPage.goto(`${BASE_URL}/agent/${existing.agent_id}`);
    const ordinaryForm = skinPage.locator('[data-ui="composer"]:not([data-variant="room"])').first();
    await ordinaryForm.locator("textarea").waitFor({ timeout: 10000 }).catch(() => {});
    const ordinary = await skinPage.evaluate(() => {
        const form = document.querySelector('[data-ui="composer"]:not([data-variant="room"])');
        const textarea = form?.querySelector("textarea");
        if (!form || !textarea) return null;
        const fs = getComputedStyle(form);
        const ts = getComputedStyle(textarea);
        return { borderRadius: fs.borderRadius, padding: fs.padding, textareaMinHeight: ts.minHeight, textareaPadding: ts.padding };
      });
    record(`room and ordinary composer geometry share the same surface (${skin.name})`, Boolean(roomComposerStyles[skin.name] && ordinary) && JSON.stringify(roomComposerStyles[skin.name]) === JSON.stringify(ordinary), JSON.stringify({ room: roomComposerStyles[skin.name], ordinary }));
    await skinContext.close();
  }
  await setSkin("");

  if (openingStaged && HOLD_FILE && FIXTURE_HOME) {
    try {
      removeFile(HOLD_FILE);
      const ended = await api(`/api/think-tanks/${encodeURIComponent(roomID)}/end`, { method: "POST", body: "{}" });
      const judge = await waitFor(async () => {
        const current = await api(`/api/think-tanks/${encodeURIComponent(roomID)}`);
        return current.judge?.agent_id && current.active_attempts?.some((a) => a.agent_id === current.judge.agent_id) ? current : null;
      }, 20000);
      const synthesis = "Synthesis: versioned keys bound stale reads most clearly; retain TTL only as an operational fallback.";
      await mcpReadAndSubmit(judge.judge.agent_id, roomID, synthesis);
      writeFileSync(HOLD_FILE, "release judge\n");
      const completed = await waitFor(async () => {
        const current = await api(`/api/think-tanks/${encodeURIComponent(roomID)}`);
        return current.phase === "ended" && current.judge?.status === "completed" ? current : null;
      }, 20000);
      unlinkSync(HOLD_FILE);
      const resultPage = await api(`/api/sessions/${encodeURIComponent(judge.judge.agent_id)}/think-tank-results`);
      const exactResult = (resultPage.results || []).find((result) => result.room_id === roomID && result.body === synthesis);
      record("End runs judge and retains exact synthesis", completed.judge?.status === "completed" && Boolean(exactResult), JSON.stringify({ end: ended.phase, judge: completed.judge, exact_result: exactResult ? { result_id: exactResult.result_id, body: exactResult.body } : null }));
      if (completed.judge?.status === "completed") {
        const judgeID = judge.judge.agent_id;
        for (const skin of [{ name: "core", value: "" }, { name: "sky-grove", value: "sky-grove" }, { name: "studio", value: "studio" }]) {
          await setSkin(skin.value);
          const resultContext = await browser.newContext({ viewport: { width: 1280, height: 900 } });
          const resultPage = await resultContext.newPage();
          await resultPage.goto(`${BASE_URL}/agent/${judgeID}`);
          const resultText = resultPage.getByText(synthesis, { exact: true }).first();
          await resultText.waitFor({ timeout: 15000 }).catch(() => {});
          record(`live judge chat shows exact synthesis (${skin.name})`, await resultText.count() > 0, resultPage.url());
          await resultPage.reload();
          const reloadedText = resultPage.getByText(synthesis, { exact: true }).first();
          await reloadedText.waitFor({ timeout: 15000 }).catch(() => {});
          record(`reloaded judge chat retains exact synthesis (${skin.name})`, await reloadedText.count() > 0, resultPage.url());
          await resultPage.goto(`${BASE_URL}/archive/${judgeID}`);
          const archivedText = resultPage.getByText(synthesis, { exact: true }).first();
          await archivedText.waitFor({ timeout: 15000 }).catch(() => {});
          record(`archive judge view retains exact synthesis (${skin.name})`, await archivedText.count() > 0, resultPage.url());
          await screenshot(resultPage, `archive-judge-${skin.name}.png`);
          await resultContext.close();
        }
        await setSkin("");
        const archiveContext = await browser.newContext({ viewport: { width: 1280, height: 900 } });
        const archivePage = await archiveContext.newPage();
        await archivePage.goto(`${BASE_URL}/archive`);
        await archivePage.getByRole("heading", { name: "Think Tanks" }).waitFor({ timeout: 10000 }).catch(() => {});
        const archivedRoomLink = archivePage.getByRole("link", { name: "Release cache council" }).first();
        await archivedRoomLink.waitFor({ timeout: 10000 }).catch(() => {});
        record("ended room remains discoverable in Archive", await archivedRoomLink.count() > 0, archivePage.url());
        await screenshot(archivePage, "archive-ended.png");
        await archiveContext.close();
      }
    } catch (error) {
      record("End runs judge and retains exact synthesis", false, String(error));
    }
  } else {
    blocked("opening overlap/isolation, End/judge, retained Archive", "MCP staging or hold release was unavailable");
  }
  writeFileSync(join(OUT_DIR, "report.json"), JSON.stringify({ baseURL: BASE_URL, roomID, checks }, null, 2));
  if (checks.some((check) => check.pass !== true)) process.exitCode = 1;
  console.log(`Report: ${join(OUT_DIR, "report.json")}`);
} catch (error) {
  record("journey completed", false, String(error));
  writeFileSync(join(OUT_DIR, "report.json"), JSON.stringify({ baseURL: BASE_URL, checks }, null, 2));
  process.exitCode = 1;
} finally {
  if (browser) await browser.close();
}

function summarizeMember(m) {
  return m ? { limit: m.limit, completed: m.completed, remaining: m.remaining } : null;
}
