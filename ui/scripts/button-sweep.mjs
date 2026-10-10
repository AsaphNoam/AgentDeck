// Rendered sweep for the shared button language (FS-12.R68/A38): real embedded desktop and
// authenticated phone routes on the isolated fake-provider fixture, in every appearance.
// After make embed, start from repo root:
// CHUCK_GROUP_BROWSER_READY=/tmp/buttons.json go test -tags sqlite_fts5 ./internal/server -run '^TestGroupBrowserFixture$' -timeout 20m -v
// Then: node ui/scripts/button-sweep.mjs /tmp/buttons.json /tmp/chuck-button-sweep
// Finish the fixture by creating /tmp/buttons.json.done after the browser exits.
// Writes screenshots plus report.json listing action buttons that still carry a raised shadow,
// lift, highlight fill or solid fill, and phone actions below a 44px target.
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { chromium } from "playwright";

const fixture = JSON.parse(readFileSync(process.argv[2], "utf8"));
const out = process.argv[3] || "/tmp/chuck-button-sweep";
mkdirSync(out, { recursive: true });
const browser = await chromium.launch();
const findings = [];
const errors = [];
async function api(path, body, method = body === undefined ? "GET" : "POST") {
  const r = await fetch(fixture.desktop + path, { method, headers: { "Content-Type": "application/json" }, body: body === undefined ? undefined : JSON.stringify(body) });
  const data = await r.json();
  if (!r.ok) throw new Error(`${path}: ${JSON.stringify(data)}`);
  return data;
}
const launch = async (name, project = "my-app") => (await api("/api/sessions", { name, group: "", project, role: "implementer", backend: "claude", model: "sonnet", interface: "chat" })).agent.agent_id;

// Action buttons are everything except rows, menu items, tabs and nav items, which keep their own construction.
async function audit(page, where, phone) {
  const rows = await page.evaluate((phone) => {
    const skip = (b) => b.closest('[role="menu"], [role="tablist"], [role="listbox"], .context-menu, .phone-row, nav') || b.matches('[role="tab"], [role="radio"], [data-slot="item"], .drag-handle');
    return [...document.querySelectorAll("button")].filter((b) => b.offsetParent && !skip(b)).map((b) => {
      const s = getComputedStyle(b);
      const r = b.getBoundingClientRect();
      const label = (b.getAttribute("aria-label") || b.textContent || "").trim().slice(0, 40);
      const issues = [];
      if (s.boxShadow !== "none") issues.push(`shadow ${s.boxShadow}`);
      if (s.transform !== "none") issues.push(`transform ${s.transform}`);
      if (s.backgroundColor !== "rgba(0, 0, 0, 0)" && !b.matches(".jump-to-latest, :hover")) issues.push(`fill ${s.backgroundColor}`);
      if (parseFloat(s.borderTopWidth) > 0) issues.push(`border ${s.borderTopWidth} ${s.borderTopColor}`);
      if (phone && r.height < 43.5) issues.push(`height ${r.height}`);
      return { label, cls: b.className, issues };
    }).filter((x) => x.issues.length);
  }, phone);
  for (const row of rows) findings.push({ where, ...row });
}
async function shot(page, name, phone = false) {
  await page.waitForTimeout(250);
  await audit(page, name, phone);
  await page.screenshot({ path: join(out, `${name}.png`), fullPage: true });
}
async function step(name, fn) {
  try { await fn(); console.log(`ok ${name}`); } catch (e) { errors.push(`${name}: ${e.message.split("\n")[0]}`); console.log(`FAIL ${name}`); }
}

const desktopContext = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const phoneContext = await browser.newContext({ viewport: { width: 390, height: 844 }, ignoreHTTPSErrors: true, hasTouch: true, isMobile: true });
await phoneContext.addCookies([{ name: fixture.cookie, value: fixture.token, url: fixture.phone, secure: true, httpOnly: true, sameSite: "Strict" }]);
const desktop = await desktopContext.newPage();
const phone = await phoneContext.newPage();
for (const page of [desktop, phone]) page.on("pageerror", (e) => errors.push(e.message));

const projects = await api("/api/projects");
await api("/api/projects/my-app", { ...projects["my-app"], cwd: "/tmp" }, "PUT");
for (const session of await api("/api/sessions")) await api(`/api/sessions/${session.agent_id}/archive`, {}, "POST");
for (const skin of ["", "sky-grove", "studio"]) {
  const name = skin || "core";
  await api("/api/config", { appearance_skin: skin }, "PUT");
  const agent = await launch(`Builder ${name}`);
  const stopped = await launch(`Stopped ${name}`);
  await api(`/api/sessions/${stopped}/stop`, {}, "POST");
  for (const width of [1024, 1440]) {
    await desktop.setViewportSize({ width, height: 900 });
    await step(`${name} dashboard ${width}`, async () => {
      await desktop.goto(fixture.desktop + "/project/my-app");
      await desktop.getByText(`Builder ${name}`, { exact: true }).first().waitFor();
      await shot(desktop, `${name}-dashboard-${width}`);
    });
  }
  await desktop.setViewportSize({ width: 1440, height: 900 });
  await step(`${name} primary hover`, async () => {
    await desktop.getByRole("button", { name: "New agent", exact: true }).hover();
    await desktop.screenshot({ path: join(out, `${name}-hover-new-agent.png`), clip: { x: 0, y: 0, width: 1440, height: 260 } });
  });
  await step(`${name} new agent dialog`, async () => {
    await desktop.getByRole("button", { name: "New agent", exact: true }).click();
    await desktop.getByRole("dialog").waitFor();
    await shot(desktop, `${name}-new-agent-dialog`);
    await desktop.keyboard.press("Escape");
  });
  await step(`${name} agent page`, async () => {
    await desktop.goto(fixture.desktop + `/agent/${agent}`);
    await desktop.locator("textarea").first().waitFor();
    await desktop.locator("textarea").first().fill("Draft so Send is enabled");
    await shot(desktop, `${name}-agent-page`);
    await desktop.keyboard.press("Tab");
    await desktop.screenshot({ path: join(out, `${name}-agent-focus.png`) });
  });
  for (const [route, label] of [["/settings", "settings"], ["/tasks", "tasks"], ["/pipelines", "pipelines"], ["/archive", "archive"]]) {
    await step(`${name} ${label}`, async () => {
      await desktop.goto(fixture.desktop + route);
      await desktop.locator("main h1").first().waitFor();
      await shot(desktop, `${name}-${label}`);
    });
  }
  for (const tab of ["Projects", "Backends", "Notifications", "Remote"]) {
    await step(`${name} settings ${tab}`, async () => {
      await desktop.goto(fixture.desktop + "/settings");
      await desktop.getByRole("tab", { name: tab }).click();
      await shot(desktop, `${name}-settings-${tab.toLowerCase()}`);
    });
  }
  await step(`${name} phone project`, async () => {
    await phone.goto(fixture.phone + "/project/my-app");
    await phone.getByText(`Builder ${name}`, { exact: true }).first().waitFor();
    await shot(phone, `${name}-phone-project`, true);
  });
  await step(`${name} phone agent`, async () => {
    await phone.getByText(`Builder ${name}`, { exact: true }).first().click();
    await phone.locator("textarea").first().waitFor();
    await phone.locator("textarea").first().fill("Draft");
    await shot(phone, `${name}-phone-agent`, true);
    await phone.getByRole("tab", { name: "Manage", exact: true }).click();
    await shot(phone, `${name}-phone-manage`, true);
  });
  await step(`${name} phone home`, async () => {
    await phone.goto(fixture.phone + "/");
    await phone.locator("h1").first().waitFor();
    await shot(phone, `${name}-phone-home`, true);
  });
  await api(`/api/sessions/${agent}/archive`, {}, "POST");
  await api(`/api/sessions/${stopped}/archive`, {}, "POST");
}
await api("/api/config", { appearance_skin: "" }, "PUT");
await browser.close();
writeFileSync(join(out, "report.json"), JSON.stringify({ errors, findings }, null, 2));
console.log(`${findings.length} findings, ${errors.length} errors → ${out}/report.json`);
