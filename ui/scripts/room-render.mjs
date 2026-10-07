// Renders the Think Tank room page with representative content in every
// appearance at the desktop floor and a wider viewport, writing screenshots for
// rendered review (TS-08.R87, INV §13). Every /api call and the event stream are
// stubbed; no Chuck server is needed.
//
//   node scripts/room-render.mjs [outDir] [state]
//
// state is "live" (default: a speaker mid-turn with a pending approval and a
// queued message), "ended" (operator ended, judge synthesis complete), "project"
// (room discovery cards), or "matrix" (the development presentation matrix).
import { mkdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createServer } from "vite";
import { chromium } from "playwright";

const [outDir = join(tmpdir(), "room-render"), state = "live"] = process.argv.slice(2);
mkdirSync(outDir, { recursive: true });

const at = "2026-10-06T09:00:00Z";
const member = (agent_id, name, project, order, limit, completed, extra = {}) => ({
  agent_id, name, project, role: "participant", order, limit, completed, may_leave: true, state: "active",
  setup_state: "ready", exists: true, running: true, agent_status: "idle", ...extra,
});
const live = state === "live";
const room = {
  version: 1, room_id: "tt_demo", title: "Session cache invalidation", goal: "Choose the cache invalidation strategy for the session store, including how stale reads are bounded under failover.",
  origin_project: "alpha", phase: live ? "discussion" : "ended", control: "running", hold: "",
  end_reason: live ? "" : "operator", judge_status: live ? "waiting" : "completed",
  participants: ["Ari", "Bea", "Cyd"], revision: 9, created_at: at, updated_at: at, openings: true,
  members: [
    member("a_ari", "Ari", "alpha", 0, 3, 2, live ? { agent_status: "waiting_input" } : {}),
    member("a_bea", "Bea", "beta", 1, 3, 1),
    member("a_cyd", "Cyd", "gamma", 2, 2, 2, { state: "exhausted", exists: false, running: false }),
    ...(live ? [] : [{ ...member("a_judge", "Judge", "alpha", 3, 1, 0), role: "judge" }]),
  ],
  active: live ? { attempt_id: "tta_3", agent_id: "a_ari", turn: "discussion", state: "running", failure: "", started_at: at } : undefined,
  pending: live ? [{ input_id: "tti_9", kind: "user", body: "Also consider read-your-writes.", created_at: at }] : [],
  failed: [],
  judge: { enabled: true, status: live ? "waiting" : "completed", agent_id: live ? "" : "a_judge", error: "" },
  deletable: !live,
};
room.active_attempts = room.active ? [room.active] : [];
const entry = (seq, kind, agent_id, agent_name, project, body, extra = {}) => ({ seq, kind, agent_id, agent_name, project, body, attempt_id: `tta_${seq}`, created_at: at, ...extra });
const entries = [
  entry(1, "opening", "a_ari", "Ari", "alpha", "**Write-through with TTL.** Every write updates Redis synchronously; reads tolerate at most `ttl=30s` staleness.\n\n```go\ncache.Set(ctx, key, value, 30*time.Second)\n```"),
  entry(2, "opening", "a_bea", "Bea", "beta", "Prefer **versioned keys**: invalidation becomes a version bump, so failover cannot resurrect stale values.\n\n| Option | Stale bound |\n|---|---|\n| TTL | 30s |\n| Versioned | 0s |"),
  entry(3, "missing_opening", "a_cyd", "Cyd", "gamma", "", { attempt_id: "" }),
  entry(4, "user", "", "", "", "Focus on failover between regions.", { attempt_id: "", input_id: "tti_1" }),
  entry(5, "reply", "a_bea", "Bea", "beta", "Material objection: TTL alone does not bound staleness after a regional failover, because the replica can serve values written before the partition."),
  ...(live ? [] : [entry(6, "synthesis", "a_judge", "Judge", "alpha", "## Synthesis\n\nVersioned keys are preferred. **Unresolved:** Ari still argues TTL is simpler to operate.")]),
];
const ev = (seq, attempt, agent_id, agent_name, type, data) => ({
  version: 1, room_id: "tt_demo", seq, attempt_id: attempt, agent_id, agent_name, project: "beta", source_seq: seq,
  event: { type, seq, ts: at, activity_id: "", parent_activity_id: "", data }, created_at: at,
});
const activity = [
  ev(1, "tta_5", "a_bea", "Bea", "tool_call", { tool_call_id: "c1", name: "mcp__chuck-messaging__read_think_tank", title: "Read the room", args: {}, status: "completed" }),
  ev(2, "tta_5", "a_bea", "Bea", "tool_call", { tool_call_id: "c2", name: "Bash", title: "Bash", args: { command: "go test ./internal/cache/..." }, status: "completed" }),
  ev(3, "tta_5", "a_bea", "Bea", "tool_result", { tool_call_id: "c2", status: "completed", content: "ok  internal/cache 0.41s" }),
  ev(4, "tta_5", "a_bea", "Bea", "diff", { tool_call_id: "c3", path: "internal/cache/keys.go", old_text: "key := id\n", new_text: "key := fmt.Sprintf(\"%s:v%d\", id, version)\n", patch: "" }),
  ...(live ? [
    ev(5, "tta_3", "a_ari", "Ari", "tool_call", { tool_call_id: "c9", name: "Edit", title: "Edit cache/ttl.go", args: { path: "cache/ttl.go" }, status: "pending" }),
    ev(6, "tta_3", "a_ari", "Ari", "permission_request", { tool_call_id: "c9", name: "Edit", reason: "Edit cache/ttl.go", args: {}, options: [{ option_id: "allow", label: "Allow", kind: "allow_once" }, { option_id: "deny", label: "Deny", kind: "reject_once" }], auto_approved: false, expires_at: "2026-10-06T10:00:00Z" }),
  ] : []),
];
// "project" renders the originating project's room cards before its agents
// (FS-02.R71): concurrent openings, a held room and an ended room with judge.
const roster = (agent_id, name, project, limit, completed, extra = {}) => ({
  agent_id, name, project, role: "participant", state: "active", limit, completed, remaining: limit - completed, exists: true, ...extra,
});
const summary = (room_id, title, extra) => ({
  version: 1, room_id, title, goal: room.goal, origin_project: "alpha", phase: "discussion", control: "running", hold: "",
  end_reason: "", judge_status: "", participants: [], revision: 3, created_at: at, updated_at: at, active_attempts: [],
  roster: [], total_remaining: 0, judge_enabled: false, judge_name: "", ...extra,
});
const attempt = (agent_id, turn) => ({ attempt_id: `tta_${agent_id}`, agent_id, turn, state: "running", failure: "", started_at: at });
const cards = [
  summary("tt_a", "Session cache invalidation", {
    phase: "openings", judge_enabled: true, judge_status: "waiting", total_remaining: 14,
    active_attempts: [attempt("a_ari", "opening"), attempt("a_bea", "opening")],
    roster: [roster("a_ari", "Ari", "alpha", 4, 0), roster("a_bea", "Bea the long-named reviewer of storage engines", "beta", 4, 0),
      roster("a_cyd", "Cyd", "gamma", 3, 0), roster("a_dov", "Dov", "alpha", 3, 0)],
  }),
  summary("tt_b", "Queue retry policy", {
    hold: "Bea's turn failed: provider error. Retry it, or end the discussion.", total_remaining: 3,
    roster: [roster("a_ari", "Ari", "alpha", 3, 2), roster("a_bea", "Bea", "beta", 3, 1), roster("a_cyd", "Cyd", "gamma", 2, 2, { state: "exhausted", remaining: 0, exists: false })],
  }),
  summary("tt_c", "Pick a release cadence", {
    phase: "ended", end_reason: "allowance_exhausted", judge_enabled: true, judge_status: "completed", judge_name: "Judge",
    roster: [roster("a_ari", "Ari", "alpha", 2, 2, { state: "exhausted", remaining: 0 }), roster("a_bea", "Bea", "beta", 2, 1, { state: "departed", remaining: 0 })],
  }),
];
const agents = room.members.filter((m) => m.exists).map((m) => ({
  agent_id: m.agent_id, name: m.name, role: "impl", project: m.project, backend: "claude", model: "m", fast: false,
  interface: "chat", created_at: at, running: true, state: m.agent_status || "idle", detail: "", context_pct: 0, updated_at: 0, archived: false,
}));

const server = await createServer({ server: { port: 5198, strictPort: true }, logLevel: "error" });
await server.listen();
const browser = await chromium.launch();
let failed = false;
try {
  for (const skin of ["", "sky-grove", "studio"]) {
    for (const width of [1024, 1440]) {
      const context = await browser.newContext({ viewport: { width, height: 1100 } });
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
      }, [...agents.map((data) => ({ agent_id: data.agent_id, data })), { agent_id: "__hydrated__", data: { hydrated: true } }]);
      await context.route((url) => url.pathname.startsWith("/api/"), (route) => {
        const path = new URL(route.request().url()).pathname;
        if (path === "/api/think-tanks") return route.fulfill({ json: { version: 1, rooms: cards, clipped: false } });
        if (path === "/api/think-tanks/tt_demo") return route.fulfill({ json: room });
        if (path === "/api/think-tanks/tt_demo/entries") return route.fulfill({ json: { version: 1, entries, complete: true } });
        if (path === "/api/think-tanks/tt_demo/activity") return route.fulfill({ json: { version: 1, activity, complete: true } });
        if (path === "/api/projects") return route.fulfill({ json: { alpha: { title: "Alpha", cwd: "/tmp", color: [80, 120, 200] }, beta: { title: "Beta", cwd: "/tmp", color: [60, 160, 120] } } });
        if (path === "/api/config") return route.fulfill({ json: { appearance_skin: skin, onboarded: true } });
        if (path === "/api/layout") return route.fulfill({ json: { order: [], density: { perRow: 3, gap: 16 } } });
        if (path === "/api/backends") return route.fulfill({ json: { version: 2, backends: {} } });
        if (path === "/api/config-sources") return route.fulfill({ json: [] });
        return route.fulfill({ json: {} });
      });
      const page = await context.newPage();
      page.on("pageerror", (error) => { failed = true; console.error(`page error: ${error.stack}`); });
      const project = state === "project";
      const matrix = state === "matrix";
      await page.goto(matrix ? "http://localhost:5198/__visual-matrix" : project ? "http://localhost:5198/project/alpha" : "http://localhost:5198/think-tank/tt_demo");
      const ready = matrix ? page.getByRole("heading", { name: "Presentation matrix" }) : project ? page.getByRole("heading", { name: "Think Tanks" }) : page.getByRole("list", { name: "Discussion" });
      const shown = await ready.waitFor({ timeout: 15_000 }).then(() => true, () => false);
      if (matrix && shown) await page.getByLabel("Fixture appearance").selectOption(skin || "core");
      await page.waitForTimeout(600);
      const out = join(outDir, `room-${state}-${skin || "core"}-${width}.png`);
      await page.screenshot({ path: out, fullPage: true });
      console.log(`wrote ${out}`);
      if (!shown) {
        failed = true;
        console.error(`room did not render (${skin || "core"} ${width})`);
      }
      await context.close();
    }
  }
} finally {
  await browser.close();
  await server.close();
}
if (failed) process.exitCode = 1;
