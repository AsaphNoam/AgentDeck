import { mkdirSync, readFileSync, unlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { chromium } from "playwright";
import { tmpdir } from "node:os";

// Complete the room journey's retained annotation/mail and deletion branches.
// Usage: node room-followup-check.mjs BASE_URL ROOM_ID OUT_DIR FIXTURE_HOME HOLD_FILE

const BASE_URL = process.argv[2] || "http://127.0.0.1:4414";
const ROOM_ID = process.argv[3];
const OUT_DIR = process.argv[4] || join(tmpdir(), `chuck-room-followup-${Date.now()}`);
const HOLD_FILE = process.argv[6];
// Restrict mutations to the disposable synthetic fixture created by this task.
const FIXTURE_HOME = process.argv[5];
if (!ROOM_ID || !HOLD_FILE || !FIXTURE_HOME || !/\/chuck-stress-\d+$/.test(FIXTURE_HOME)) throw new Error("supply the disposable stress fixture home, room and hold path");
const fixtureConfig = JSON.parse(readFileSync(join(FIXTURE_HOME, "config.json"), "utf8"));
if (BASE_URL !== `http://127.0.0.1:${fixtureConfig.port}`) throw new Error("base URL must match the isolated fixture port");
const backends = JSON.parse(readFileSync(join(FIXTURE_HOME, "backends.json"), "utf8"));
if (backends.backends.claude.env.FAKEACP_SCENARIO !== "hold_turn") throw new Error("fixture must use fake ACP");
mkdirSync(OUT_DIR, { recursive: true });
const checks = [];
const INSTRUCTION = `Please revisit the stale-read tradeoff in a private follow-up. Test ${Date.now()}.`;
const record = (name, pass, detail = "") => { checks.push({ name, pass, detail }); console.log(`${pass ? "PASS" : "FAIL"} ${name}${detail ? ` — ${detail}` : ""}`); };
const api = async (path, options = {}) => {
  const response = await fetch(BASE_URL + path, { ...options, headers: { "Content-Type": "application/json", ...(options.headers || {}) } });
  const text = await response.text();
  let body; try { body = text ? JSON.parse(text) : undefined; } catch { body = text; }
  if (!response.ok) throw new Error(`${path} -> ${response.status}: ${text}`);
  return body;
};
const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
async function waitFor(fn, timeout = 15000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) { const value = await fn(); if (value) return value; await wait(250); }
  throw new Error("timed out");
}
const screenshot = async (page, name) => page.screenshot({ path: join(OUT_DIR, name), fullPage: true });
const removeHold = () => { try { unlinkSync(HOLD_FILE); } catch (error) { if (error.code !== "ENOENT") throw error; } };

const config = await api("/api/config");
const roomBefore = await api(`/api/think-tanks/${ROOM_ID}`);
if (roomBefore.title !== "Release cache council" || roomBefore.origin_project !== "stress" || roomBefore.phase !== "ended") throw new Error("expected the synthetic ended test room");
const entriesBefore = (await api(`/api/think-tanks/${ROOM_ID}/entries`)).entries;
const recipient = roomBefore.members.find((member) => member.role === "participant" && member.exists);
const judgeID = roomBefore.judge.agent_id;
const resultsBefore = (await api(`/api/sessions/${encodeURIComponent(judgeID)}/think-tank-results?after=0`)).results;
const resultBefore = resultsBefore.find((result) => result.room_id === ROOM_ID);
if (!recipient || !resultBefore) throw new Error("fixture lacks existing recipient or retained judge result");

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 1024, height: 768 } });
const openRoom = async (skin) => {
  await api("/api/config", { method: "PUT", body: JSON.stringify({ appearance_skin: skin === "core" ? "" : skin }) });
  await page.goto(`${BASE_URL}/think-tank/${ROOM_ID}`, { waitUntil: "domcontentloaded" });
  await page.getByRole("heading", { name: roomBefore.title }).waitFor();
};
const makeDraft = async () => {
  await page.locator(".think-tank-entry").first().click({ button: "right" });
  await page.getByText("Annotate whole entry", { exact: true }).click();
  await page.getByLabel("Instruction", { exact: true }).fill(INSTRUCTION);
  await page.getByText("An agent", { exact: true }).click();
  const select = page.getByLabel("Annotation recipient");
  await select.selectOption(recipient.agent_id);
};

try {
  for (const skin of ["core", "sky-grove", "studio"]) {
    await openRoom(skin);
    await makeDraft();
    const geometry = await page.locator(".annotation-tray").evaluate((node) => {
      const rect = node.getBoundingClientRect();
      const controls = [...node.querySelectorAll("button, select, textarea, input")].map((control) => {
        const r = control.getBoundingClientRect();
        return { left: r.left, right: r.right, top: r.top, bottom: r.bottom };
      });
      return { tray: { left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom }, controls };
    });
    const fits = geometry.tray.left >= 0 && geometry.tray.right <= 1024 && geometry.controls.every((r) => r.left >= 0 && r.right <= 1024 && r.top >= 0 && r.bottom <= 768);
    record(`annotation tray fits viewport ( ${skin} / 1024 )`, fits, JSON.stringify(geometry));
    await screenshot(page, `annotation-tray-${skin}-1024.png`);
    if (skin === "core") {
      writeFileSync(HOLD_FILE, "release private follow-up\n");
      await page.getByRole("button", { name: "Send annotations" }).click();
      await page.getByRole("button", { name: "Send annotations" }).waitFor({ state: "detached", timeout: 15000 }).catch(() => {});
      await waitFor(async () => {
        const mail = await api(`/api/sessions/${recipient.agent_id}/messages`);
        return JSON.stringify(mail).includes(INSTRUCTION);
      });
      removeHold();
    } else {
      await page.getByRole("button", { name: "Discard all" }).click();
    }
  }

  const entriesAfter = (await api(`/api/think-tanks/${ROOM_ID}/entries`)).entries;
  record("agent-target annotation retains its room audit entry", entriesAfter.length === entriesBefore.length + 1 && entriesAfter.at(-1).kind === "annotation" && entriesAfter.at(-1).context?.recipient === recipient.name, `before=${entriesBefore.length} after=${entriesAfter.length}`);
  record("room remains ended after private follow-up", (await api(`/api/think-tanks/${ROOM_ID}`)).phase === "ended");

  await page.goto(`${BASE_URL}/agent/${recipient.agent_id}`, { waitUntil: "domcontentloaded" });
  await page.locator('[data-ui="composer"] textarea').waitFor();
  record("annotation recipient remains an ordinary chat", true, `${BASE_URL}/agent/${recipient.agent_id}`);

  const deleted = await api(`/api/think-tanks/${ROOM_ID}`, { method: "DELETE" });
  record("ended room deletion succeeds", Boolean(deleted === undefined || deleted === ""));
  const resultsAfter = (await api(`/api/sessions/${encodeURIComponent(judgeID)}/think-tank-results?after=0`)).results;
  const resultAfter = resultsAfter.find((result) => result.result_id === resultBefore.result_id);
  record("retained judge result survives room deletion", resultAfter?.body === resultBefore.body && resultAfter?.result_id === resultBefore.result_id && resultAfter.room_available === false, `result_id=${resultBefore.result_id}`);
  await page.goto(`${BASE_URL}/think-tank/${ROOM_ID}`, { waitUntil: "domcontentloaded" });
  await page.getByRole("heading", { name: "Think Tank unavailable" }).waitFor();
  record("deleted room UI reports unavailable", true);
} finally {
  removeHold();
  await api("/api/config", { method: "PUT", body: JSON.stringify({ appearance_skin: config.appearance_skin }) }).catch(() => {});
  await browser.close();
}

writeFileSync(join(OUT_DIR, "report.json"), JSON.stringify({ baseURL: BASE_URL, roomID: ROOM_ID, recipient: recipient.agent_id, checks }, null, 2));
if (checks.some((check) => !check.pass)) process.exitCode = 1;
