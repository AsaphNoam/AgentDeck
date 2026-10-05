// Renders the phone app's agent conversation at iPhone size and writes a
// screenshot, so phone UI work can be checked against the real view (jsdom
// tests cannot see layout, INV §13). Every /api call and the event stream are
// stubbed; no Chuck server or paired device is needed.
//
//   node scripts/phone-render.mjs [transcript.json] [out.png]
//
// transcript.json is a transcript window ({events, has_more}) for agent "a1";
// without one (or with ""), a streamed reply plus a tool call is shown. The
// screenshot defaults to the system temp directory.
import { readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createServer } from "vite";
import { chromium, devices } from "playwright";

const [fixturePath, out = join(tmpdir(), "phone-render.png")] = process.argv.slice(2);

const agent = {
  agent_id: "a1", name: "phone-render", role: "implementer", project: "my-app", backend: "claude", model: "m",
  fast: false, fast_available: false, steering_available: true, interface: "chat", created_at: "", running: true,
  state: "idle", detail: "", context_pct: 0, updated_at: 0, archived: false,
};
const event = (seq, type, data) => ({ agent_id: "a1", seq, type, ts: "", data });
const words = "Done. **All tests** pass now, and the build is `green` again after the fix.".split(/(?<= )/);
const transcript = fixturePath
  ? JSON.parse(readFileSync(fixturePath, "utf8"))
  : {
      agent_id: "a1",
      has_more: false,
      events: [
        event(1, "user_text", { text: "Run the tests and tell me the status." }),
        event(2, "tool_call", { tool_call_id: "t1", name: "Bash", args: { command: "make test" } }),
        event(3, "tool_result", { tool_call_id: "t1", output: "ok  github.com/AsaphNoam/Chuck/internal/server" }),
        ...words.map((text, i) => event(4 + i, "assistant_text", { delta: text })),
      ],
    };

const server = await createServer({ server: { port: 5199, strictPort: true }, logLevel: "error" });
await server.listen();
const browser = await chromium.launch();
try {
  const context = await browser.newContext({ ...devices["iPhone 13"] });
  // Replace the event stream with one that hydrates the single agent and stays open.
  await context.addInitScript((hydrated) => {
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
  }, [{ agent_id: "a1", data: agent }, { agent_id: "__hydrated__", data: { hydrated: true } }]);
  // A URL predicate, not "**/api/**", which would also catch Vite's /src/api modules.
  await context.route((url) => url.pathname.startsWith("/api/"), (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/sessions/a1/transcript") return route.fulfill({ json: transcript });
    if (path === "/api/remote/home") return route.fulfill({ json: { needs_you: [], active_runs: [] } });
    if (path === "/api/sessions/a1/prompt") return route.fulfill({ status: 404, json: { error: { code: "not_found", message: "none" } } });
    return route.fulfill({ json: {} });
  });
  const page = await context.newPage();
  page.on("pageerror", (error) => console.error(`page error: ${error.message}`));
  await page.goto("http://localhost:5199/remote.html");
  // Vite serves the desktop entry for deep links, so route inside the phone app.
  await page.evaluate(() => {
    history.pushState(null, "", "/agent/a1");
    window.dispatchEvent(new PopStateEvent("popstate"));
  });
  // Screenshot even when the conversation never appears; that view is the evidence.
  const shown = await page.getByRole("list", { name: "Conversation" }).waitFor({ timeout: 10_000 }).then(() => true, () => false);
  await page.screenshot({ path: out, fullPage: true });
  console.log(`wrote ${out}`);
  if (!shown) {
    console.error("the conversation did not render; see the screenshot");
    process.exitCode = 1;
  }
} finally {
  await browser.close();
  await server.close();
}
