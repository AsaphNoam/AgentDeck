# codex-acp 1.12.0 wire notes (temporary)

Non-authoritative extraction from `npm pack @agentclientprotocol/codex-acp@1.12.0` `dist/index.js`,
supporting the in-progress `adopt-modern-codex-acp-capabilities` change. Line numbers refer to that
file. Correction: `tool_call` sites also carry `name` (dynamic tools, `view_image`,
`exec_command`/`write_stdin`). Delete with the change.


All citations are `codex-acp-1.12.0/dist/index.js:LINE` unless explicitly marked 1.10.0. File is
plain (non-minified, one-statement-per-line) esbuild output, so line numbers are stable and directly
readable — no prettified copy was needed for this pass (`codex-acp-1.12.0/dist/index.pretty.js` also
exists from an earlier pass, unchanged).

---

## Correction: the `name` field on tool_call / tool_call_update

The earlier blanket claim ("`tool_call_update` never carries `name`") is **wrong**. Precise picture:

### `name` on `tool_call` (initial announcement) — common, not what the coordinator flagged

- `createDynamicToolCallUpdate` (`index.js:23046-23051`) — despite the name, this builds a
  **`sessionUpdate: "tool_call"`** (not `_update`) object, because it spreads
  `createExecuteToolCallUpdate` (`index.js:23133-23142`), which hardcodes `sessionUpdate: "tool_call"`:
  ```js
  async function createDynamicToolCallUpdate(item) {
    return {
      ...await createExecuteToolCallUpdate(item, item.tool, { arguments: item.arguments }),
      name: functionToolName(item.tool, item.namespace)
    };
  }
  ```
  `functionToolName` (`index.js:22933-22935`): `` `${namespace ?? ""}${name}` ``.
- `createImageViewUpdate` (`index.js:23052-23069`) — the `view_image` case — is also
  `sessionUpdate: "tool_call"`, with `name: "view_image"` hardcoded (`index.js:23058`).

So **both of the coordinator's cited examples are `tool_call`, not `tool_call_update`**. `tool_call`
already carries `name` in several places even in 1.10.0-style code (it's the initial-announcement
event and less interesting for the "never" claim).

### `name` on `tool_call_update` (the actually-corrected claim) — two real, confirmed cases

1. **`dynamicToolCall` completion**, inside `completeItemEvent`'s switch (`index.js:24108-24124`):
   ```js
   case "dynamicToolCall":
     return {
       sessionUpdate: "tool_call_update",
       toolCallId: event.item.id,
       name: functionToolName(event.item.tool, event.item.namespace),   // <-- always present
       status: event.item.status === "completed" ? "completed" : "failed"
     };
   ```
   `name` is unconditional here — every dynamic (custom/function) tool call's completion update
   carries it.

2. **Command-execution completion**, in `completeCommandExecutionEvent` (`index.js:25334-25341`):
   ```js
   completeCommandExecutionEvent(item) {
     const name = commandToolName(item.source);
     const update = {
       sessionUpdate: "tool_call_update",
       toolCallId: item.id,
       ...name === void 0 ? {} : { name },   // <-- conditional
       status: item.status === "completed" ? "completed" : "failed",
       ...
     };
   ```
   `commandToolName` (`index.js:22936-22945`) only returns a name for
   `unifiedExecStartup` → `"exec_command"` and `unifiedExecInteraction` → `"write_stdin"`; for
   `agent`/`userShell` sources it returns `undefined`, so **`name` is omitted** for ordinary shell
   commands and present only for the unified-exec tool family.

### Every other `tool_call_update` site — checked, no `name`

Verified every `sessionUpdate: "tool_call_update"` construction site in the file (grep, 21 sites):
`index.js:23007` (command-execution partial), `23086` (image-gen complete), `23116` (context
compaction complete), `23176`/`23206`/`23215`/`23232` (guardian review / fuzzy-file-search /
web-search), `23260` (collab-agent tool call complete), `25112` (`fileChange` case, sibling of the
`dynamicToolCall` case above), `25125` (`mcpToolCall` case — **no `name`**, only
`rawInput`/`rawOutput`), `25268`/`25292` (terminal/mcp progress deltas — `_meta` only), `26622`/
`26630`/`26900` (permission-completion notifications, `title`/`content`/`rawOutput`, no `name`),
`30982`/`30999` (terminal exec completion — `rawOutput` only), `32117` (async-task backgrounded
marker — `_meta` only), `34813` (plan-mode approval completion — `rawOutput` only) — none of these
set `name`.

**Corrected summary:** `tool_call_update.name` appears in exactly two cases — dynamic/custom tool
call completions (always) and unified-exec command completions (`exec_command`/`write_stdin` only).
`mcpToolCall` completions do **not** get `name` (they only ever had `title` on the initial `tool_call`,
built as `` `mcp.${server}.${tool}` `` — see §G.4). The AgentDeck-facing rule of thumb: a client that
wants a stable tool identifier on updates should track it from the initial `tool_call`'s `name`/`title`
and correlate by `toolCallId`; only rely on `tool_call_update.name` being present for
dynamic-tool-call and unified-exec-command completions.

---

## A. Subagents: session/update routing, request_permission, agent_thought_chunk

### `subagent_spawned` / `subagent_state_update` full JSON

Built at `materialize()` (`index.js:24019-24046`) and `finish()` (`index.js:24055-24069`):

```json
// subagent_spawned
{
  "sessionUpdate": "subagent_spawned",
  "subagentSessionId": "th_child_abc123",
  "name": "researcher",
  "task": "Delegated task for researcher",
  "capabilities": {}
}
```
```json
// subagent_state_update
{
  "sessionUpdate": "subagent_state_update",
  "subagentSessionId": "th_child_abc123",
  "state": "completed"
}
```
`state` comes from `terminalStateFromTurn`/explicit `"cancelled"`/`"disconnected"` — observed values:
`"completed"`, `"cancelled"`, `"disconnected"`, `"interrupted"`→mapped to `"cancelled"`.

`capabilities: {}` is a hardcoded empty object at every call site (`24030`, `24089`, `24109`,
`33784`, `33827`) — 1.12.0 does not populate per-subagent capabilities.

### Which sessionId does `subagent_spawned`/`subagent_state_update` go out under?

**Always the parent's sessionId**, passed explicitly as the second arg to
`ACPSessionConnection.update(update, sessionId)`:

```js
// index.js:22147-22152
async update(update, sessionId = this.sessionId) {
  await this.connection.notify(methods.client.session.update, asSdkSessionNotification({
    sessionId,
    update
  }));
}
```

`materialize()` calls `this.session.update({...subagent_spawned}, parentSessionId)`
(`index.js:24027-24032`); `finish()` calls `this.session.update({...subagent_state_update},
child.parentSessionId)` (`index.js:24060-24065`). Since `session.update`'s second parameter becomes
the **top-level `sessionId`** of the JSON-RPC notification params verbatim (no wrapping), these two
update kinds are unconditionally emitted with the immediate parent's own sessionId at top level —
this part of the earlier pass's claim was correct.

### Child's *ordinary* updates (agent_message_chunk, tool_call, agent_thought_chunk) — capability-gated, NOT always parent

This is where the earlier "always parent, child only in payload" claim needs correction. The live
(non-replay) forwarding path is:

1. `CodexSubagentSubscriptions.discover()` (`index.js:27991-28009`) subscribes to the child thread's
   own app-server notifications. When a child event arrives:
   ```js
   // index.js:28004-28009
   this.client.onServerNotification(childSessionId, (childEvent) => {
     const eventThreadId = childEvent.params.threadId;
     if (eventThreadId !== childSessionId) return;
     this.discover(session, childEvent);
     if (session.current.supportsSubagents) session.current.dispatch(childEvent);
     else session.current.enqueueInteraction(this.rootAttributed(childEvent, session.current.rootSessionId));
   });
   ```
   `rootAttributed` (`index.js:28074-28079`) rewrites `childEvent.params.threadId` to the ROOT
   sessionId when the client does **not** advertise subagent support — that's the "always parent"
   fallback path, but it's conditional.

2. When supported, `dispatch(childEvent)` reaches `CodexEventHandler.handleNotification`
   (`index.js:24734-24761`), which ends with:
   ```js
   // index.js:24759-24761
   if (updateEvent === void 0) updateEvent = await this.createUpdateEvent(notification);
   if (updateEvent) {
     await this.session.update(updateEvent, this.subagents.notificationSessionId(notification));
   }
   ```
   and `notificationSessionId` (`index.js:23917-23920`):
   ```js
   notificationSessionId(notification) {
     const threadId = notification.params.threadId;
     return typeof threadId === "string" && this.children.has(threadId)
       ? this.children.get(threadId).sessionId
       : this.rootSessionId;
   }
   ```

**Conclusion (corrects the earlier pass):** when the client's capabilities indicate subagent support
(`clientSupportsSubagents`, `index.js:31815-31821` — either a `capabilities.subagents` object or the
JetBrains AIR `nativeSubagentSessions` capability key), codex-acp emits the child's ordinary
`session/update` notifications (agent_message_chunk, tool_call, tool_call_update, agent_thought_chunk,
etc.) with the **child's own sessionId as the top-level `sessionId` param** — not the parent's, and
not merely referenced via `subagentSessionId`/`_meta` in the payload. Only when the client does *not*
support subagents does codex-acp flatten everything under the root/parent sessionId
(`rootAttributed`). `session.update`'s call signature (`update(update, sessionId = this.sessionId)`)
makes this unambiguous — whatever sessionId is passed becomes the notification's top-level field
verbatim.

`clientSupportsSubagents` (`index.js:31815-31821`):
```js
function clientSupportsSubagents(capabilities) {
  const subagents = capabilities?.subagents;
  if (typeof subagents === "object" && subagents !== null && !Array.isArray(subagents)) {
    return true;
  }
  return clientSupportsAirCapability(capabilities, AIR_NATIVE_SUBAGENT_SESSIONS_KEY);
}
```

### History-replay path (`session/load`, `session/fork`) — also uses the child's own sessionId

`streamNativeThreadHistory(sessionId, thread, ...)` (`index.js:33767-33858`) builds a **fresh**
`ACPSessionConnection` bound to whatever `sessionId` it's called with:
```js
async streamNativeThreadHistory(sessionId, thread, sessionState, ancestry, threadCache) {
  const session = new ACPSessionConnection(this.connection, sessionId);
  ...
  await this.streamNativeThreadHistory(childSessionId, { ...child, turns: [childTurn] }, ...);
  // (index.js:33800-33807)
```
So replayed child turns are streamed under `childSessionId` directly (recursive call rebinds the
session object), consistent with the live path.

### `session/request_permission` for a child tool call — child's own sessionId, capability-gated

`registerInteractiveHandlers` (`index.js:28011-28053`) resolves the sessionId to use for a
permission/elicitation request via `interactionSessionId`:
```js
// index.js:28065-28069
async interactionSessionId(subscription, targetSessionId) {
  if (targetSessionId === subscription.rootSessionId) return targetSessionId;
  if (!subscription.supportsSubagents) return subscription.rootSessionId;
  return await subscription.waitForChildSession(targetSessionId);
}
```
and each handler does `{ ...params, threadId: sessionId }` before forwarding to
`current.approvalHandler.handleCommandExecution` / `handleFileChange` /
`handlePermissionsRequest` (`index.js:28014-28033`). So: **child's own sessionId when the client
supports subagents; the root/parent's sessionId when it doesn't.**

### Child-emitted `agent_thought_chunk` — same routing, no special case

`agent_thought_chunk` updates are produced by `createAgentThoughtChunk`/`createAgentReasoningEventUpdates`/
`createReasoningUpdates` (`index.js:23709-23723`, `23863-23870`, `30905-30912`) exactly like any
other item update, and go through the same `createUpdateEvent` → `this.session.update(updateEvent,
this.subagents.notificationSessionId(notification))` path as agent_message_chunk/tool_call above.
There is no separate/different routing rule for thought chunks — they follow the same
capability-gated child-vs-root sessionId logic as §A's main finding. (See §E for the exact
content shape.)

---

## B. Async tasks

### `async_task_spawned` / `async_task_state_update` full field list

`publishSpawn` (`index.js:32114-32133`):
```json
// preceding tool_call_update (marks the originating tool call as backgrounded)
{
  "sessionUpdate": "tool_call_update",
  "toolCallId": "item_42",
  "_meta": {
    "jetbrains": { "air": { "asyncTasks": { "backgrounded": true } } }
  }
}
```
```json
// async_task_spawned
{
  "sessionUpdate": "async_task_spawned",
  "asyncTaskId": "wire-task-id-string",
  "name": "npm run dev",
  "taskType": "shell",
  "showInTranscript": false,
  "canStop": true,
  "toolCallId": "item_42"
}
```
`publishTerminalState` (`index.js:32163-32172`):
```json
// async_task_state_update
{
  "sessionUpdate": "async_task_state_update",
  "asyncTaskId": "wire-task-id-string",
  "state": "stopped",
  "toolCallId": "item_42"
}
```
`state` values come from the task's terminal-state machine (`"stopped"`, plus whatever
`isTerminalState`/`task.state` allows — running/stopping transitions are internal, not emitted as
`async_task_state_update` until terminal).

Both `publishSpawn` and `publishTerminalState` call `this.session.update(update, task.sessionId)`
(`index.js:32118-32133`, `32177-32180`) — `task.sessionId` is set when the task is remembered
(`remember(threadId, sessionId, ...)`, `index.js:32138-32157`), and that `sessionId` is whatever
sessionId `CodexBackgroundTerminalTasks` was constructed with per-session
(`createAsyncTasks(sessionId)`, `index.js:32702-32709` — one instance per `ACPSessionConnection`,
itself keyed by the owning session, i.e. the child's own sessionId when the terminal/task belongs to
a subagent thread). **A child subagent's async task therefore arrives under the child's own
sessionId**, not the root's — `CodexBackgroundTerminalTasks` is instantiated per-session in
`installSessionState`/session bookkeeping and threaded through with that session's own
`ACPSessionConnection`.

### `_session/async_task/stop` — request/response/error shapes

Method constant: `ASYNC_TASK_STOP_METHOD = "_session/async_task/stop"` (`index.js:31608`).

Request schema (`index.js:35126-35129`):
```json
{ "sessionId": "th_abc123", "asyncTaskId": "wire-task-id-string" }
```
(zod: `sessionId: string().trim().min(1)`, `asyncTaskId: string().trim().min(1)`, `.passthrough()`.)

Handler (`index.js:32395-32406`):
```js
case ASYNC_TASK_STOP_METHOD: {
  if (this.providerUpdate !== null) await this.providerUpdate;
  const sessionState = this.sessions.get(methodRequest.params.sessionId);
  if (!sessionState) return { stopped: false };
  return {
    stopped: await this.runWithProcessCheck(
      () => sessionState.asyncTasks.stop(methodRequest.params.asyncTaskId)
    )
  };
}
```
Response shape: `{ "stopped": boolean }` — always this one field, both success and "unknown
session"/"unknown task" cases return `{ stopped: false }` (no distinct error shape at the app level;
`runWithProcessCheck` can still surface a JSON-RPC-level transport error if the underlying process
call fails, but the extension method itself has no dedicated error variant).

`asyncTasks.stop(taskId)` (`index.js:32025-32046`) returns `false` for: task not found, task not yet
published, or the underlying `threadBackgroundTerminalsTerminate` call reporting
`terminated: false`; returns `true` only after a successful terminate + `finish(task, "stopped")`.

### Async-task capability gate — exact conditional

`createAsyncTasks(sessionId)` (`index.js:32702-32709`):
```js
createAsyncTasks(sessionId) {
  return new CodexBackgroundTerminalTasks(
    clientSupportsAirCapability(this.clientCapabilities, AIR_ASYNC_TASKS_KEY),
    sessionId,
    this.codexAcpClient.appServerClient,
    new ACPSessionConnection(this.connection, sessionId)
  );
}
```
`AIR_ASYNC_TASKS_KEY = "asyncTasks"` (`index.js:22847`). `clientSupportsAirCapability`
(`index.js:22868-22875`):
```js
function clientSupportsAirCapability(capabilities, capability) {
  const meta3 = asRecord(capabilities?._meta);
  const jetbrains = asRecord(meta3[JETBRAINS_META_KEY]);   // "jetbrains"
  const air = asRecord(jetbrains[AIR_META_KEY]);            // "air"
  const version2 = air[AIR_EXTENSION_VERSION_KEY];          // "version"
  const supported = air[AIR_EXTENSION_CAPABILITIES_KEY];    // "capabilities"
  return typeof version2 === "number" && Number.isInteger(version2)
    && version2 >= AIR_EXTENSION_VERSION            // AIR_EXTENSION_VERSION = 1
    && Array.isArray(supported) && supported.includes(capability);
}
```
So the exact key path read on the client's `initialize` request is:
**`clientCapabilities._meta.jetbrains.air.version` (integer ≥ 1) AND
`clientCapabilities._meta.jetbrains.air.capabilities` (array) containing `"asyncTasks"`.**
This is a single unified JetBrains AIR-extension gate — there is no separate non-AIR async-task
capability key; `subagents` native support (§A) is gated the same way via a *different* array
entry, `"nativeSubagentSessions"` (`AIR_NATIVE_SUBAGENT_SESSIONS_KEY`, `index.js:22841`), or the
plain ACP `capabilities.subagents` object.

`isActive()`-style guard: `CodexBackgroundTerminalTasks.stop` etc. all early-return when the
constructor's `isActive`/first boolean arg (the `clientSupportsAirCapability(...)` result) is false.

---

## C. `session/fork`

### Request params (full, zod `zForkSessionRequest`, `index.js:19576-19582`)
```json
{
  "sessionId": "th_source123",
  "cwd": "/Users/me/project",
  "additionalDirectories": ["/Users/me/project/vendor"],
  "mcpServers": [],
  "_meta": null
}
```
(`additionalDirectories` and `mcpServers` default to `[]` on parse error; `_meta` is an optional
passthrough record.)

### Response (full, zod `zForkSessionResponse`, `index.js:19043-19048`)
```json
{
  "sessionId": "th_forked456",
  "modes": { "currentModeId": "code", "availableModes": [] },
  "configOptions": [],
  "_meta": null
}
```
Runtime builder, `forkSession` (`index.js:32761-32772`):
```js
async forkSession(params) {
  ...
  const [sessionId, , modeState] = await this.tryCreateSession(params, "fork");
  return {
    sessionId,
    modes: modeState,
    ...this.createSessionConfigOptionsResponse(this.getSessionState(sessionId))
  };
}
```

### History replay: notifications before the RPC response, not a separate mechanism

Confirmed by control flow, not just naming. `loadSession` (`index.js:32720-32741`) — and
`forkSession`'s shared session-creation path via `tryCreateSession` → (for `load`, and for `fork`
when the codex thread already has turns to replay) — calls:
```js
await this.streamThreadHistory(sessionId, thread);   // index.js:32731
...
return { models: modeState.availableModels, ... };   // the RPC response, built and returned AFTER
```
`streamThreadHistory` (`index.js:33737-33746`) delegates to `streamNativeThreadHistory`
(`index.js:33767-33858`), which does `await session.update(update)` synchronously, in a loop, for
every turn/item before the outer async function resolves. Because the JSON-RPC layer only sends the
method's response once its handler's promise resolves, and the handler `await`s the full history
stream first, **every `session/update` notification for replayed history is guaranteed to reach the
client strictly before the `session/fork` (or `session/load`) JSON-RPC response**. This is not a
separate mechanism — it is a straight-line `await` sequence inside the request handler.

### Cursor pagination for history replay

`threadReadHistory(threadId, initialCursor = null)` (`index.js:29584-29604`):
```js
async threadReadHistory(threadId, initialCursor = null) {
  const turns = [];
  const seenCursors = new Set();
  if (initialCursor !== null) seenCursors.add(initialCursor);
  let cursor = initialCursor;
  do {
    const page = await this.threadTurnsList({
      threadId, cursor, limit: 50, sortDirection: "desc", itemsView: "full"
    });
    turns.push(...page.data);
    cursor = page.nextCursor;
    if (cursor !== null) {
      if (seenCursors.has(cursor)) throw new Error("Codex returned a repeated thread history cursor");
      seenCursors.add(cursor);
    }
  } while (cursor !== null);
  return turns.reverse();
}
```
This is codex-acp's own pagination against the underlying Codex app-server's `thread/turns/list`
(page size 50, descending, then reversed to chronological order client-side) — it is internal
bookkeeping, not exposed to the ACP client; the ACP client never sees a cursor, only the resulting
`session/update` stream.

### `sessionCapabilities.fork` and `session/delete` advertisement

Both are advertised, in the real `initialize` response builder (`index.js:32316-32323`):
```js
const sessionCapabilities = {
  resume: {},
  list: {},
  close: {},
  delete: {},
  fork: {},
  additionalDirectories: {},
  subagents: {}
};
```
returned inside `agentCapabilities.sessionCapabilities` (`index.js:32333-32336`). So: **yes**,
`sessionCapabilities.fork = {}` is present (an empty object signals "supported, no sub-capability
flags"), at `index.js:32320`. **`session/delete` is advertised the same way**, as
`sessionCapabilities.delete = {}` at `index.js:32319`; the corresponding request/response methods
are wired at `index.js:35210` (`.onRequest(methods.agent.session.delete, ...)`), method name
`"session/delete"` defined at `index.js:3709`.

---

## D. File-change report (`agentFileChangeReportRequest` / `session_info_update`)

### Request object shape — `_meta.jetbrains.air.agentFileChangeReportRequest`

Parsed by `parseAgentFileChangeReportRequest` (`index.js:24217-24229`):
```js
function parseAgentFileChangeReportRequest(meta3) {
  const jetbrains = asRecord2(meta3?.[JETBRAINS_META_KEY]);
  const air = asRecord2(jetbrains?.[AIR_META_KEY]);
  const request = asRecord2(air?.[AIR_AGENT_FILE_CHANGE_REPORT_REQUEST_KEY]); // "agentFileChangeReportRequest"
  if (request === null || !hasOnlyKeys(request, ["version", "requestId"]) || request["version"] !== AGENT_FILE_CHANGE_REPORT_VERSION) {
    return null;
  }
  const requestId = request["requestId"];
  if (typeof requestId !== "string" || !/^[A-Za-z0-9._:-]{1,128}$/.test(requestId)) return null;
  return { version: AGENT_FILE_CHANGE_REPORT_VERSION, requestId };
}
```
`AGENT_FILE_CHANGE_REPORT_VERSION = 1` (`index.js:24206`). The object accepts **exactly two fields**
(`hasOnlyKeys` enforces no extras): `version` (must equal `1`) and `requestId` (string,
`^[A-Za-z0-9._:-]{1,128}$`).

Example prompt-request `_meta`:
```json
{
  "jetbrains": {
    "air": {
      "agentFileChangeReportRequest": {
        "version": 1,
        "requestId": "fcr-2026-09-22-001"
      }
    }
  }
}
```
Gated by `clientSupportsAgentFileChangeReports(this.clientCapabilities)` before even attempting to
parse (`index.js:34379`) — itself an `AIR` capability check for
`AIR_AGENT_FILE_CHANGE_REPORT_KEY = "agentFileChangeReport"` (`index.js:22839`, via
`clientSupportsAgentFileChangeReports`, `index.js:32241-...`, same `clientSupportsAirCapability`
mechanism as §B).

### `session_info_update` response shape

Built at `publishAgentFileChangeReport` (`index.js:33893-33920`):
```js
await session.update({
  sessionUpdate: "session_info_update",
  _meta: {
    jetbrains: {
      air: {
        version: 1,                        // AIR_EXTENSION_VERSION
        agentFileChangeReport: report       // AIR_AGENT_FILE_CHANGE_REPORT_KEY
      }
    }
  }
});
```
`report` is either:
```js
// createReportedAgentFileChangeReport (index.js:24238-24248)
{
  version: 1,
  requestId: "fcr-2026-09-22-001",
  status: "reported",
  paths: ["src/index.ts", "README.md"],   // string[], see normalizeFileChangeReport
  declaredComplete: false,                // report.complete && !truncated — report.complete is
                                           // hardcoded false by parseTurnDiff (index.js:24264,24290),
                                           // so declaredComplete is effectively always false today
  truncated: false,                       // true if paths/bytes were cut to fit limits, or any path
                                           // exceeded AGENT_FILE_CHANGE_REPORT_MAX_PATH_LENGTH (4096)
  uncertainty: "Codex turn diffs may omit same-content renames and changes made outside apply_patch, including shell commands, version-control commands, generators, and child processes."
}
```
or, on any parse/normalize failure or when there's no turn to report on
(`createUnavailableAgentFileChangeReport`, `index.js:24246-24252`):
```json
{
  "version": 1,
  "requestId": "fcr-2026-09-22-001",
  "status": "unavailable",
  "reason": "providerError"
}
```
`reason` values observed: `"providerError"` (default/unexpected exceptions,
`index.js:34399`-area), plus whatever `AgentFileChangeReportError.reason` was thrown as —
`"invalidOutput"` (from `parseTurnDiff`/`fitReportedAgentFileChangeReport`) or `"cancelled"`
(prompt cancellation, `index.js:34407`).

`paths` entries are plain **strings** (workspace-relative or absolute-normalized file paths), not
objects — `normalizeFileChangeReport` (`index.js:24335-24384`) dedupes, caps at
`AGENT_FILE_CHANGE_REPORT_MAX_PATHS = 1024` entries and `AGENT_FILE_CHANGE_REPORT_MAX_TOTAL_BYTES =
256 * 1024` total bytes, and sets `truncated = true` whenever anything is dropped for length/count/byte
reasons. `uncertainty`'s value is a **single fixed constant string** (`TURN_DIFF_UNCERTAINTY`,
`index.js:24211`), not an enum — it's present whenever `parseTurnDiff` produced it (both of
`parseTurnDiff`'s return paths set it, `index.js:24264`, `24290`), and omitted only via the spread
`...report.uncertainty ? { uncertainty: report.uncertainty } : {}` if a report path lacked it.

`requestId` threads straight through unmodified from the parsed prompt-request meta into both the
`reported` and `unavailable` report shapes — no rewriting, no truncation (already bounded to 128
chars by the source-side regex).

---

## E. `agent_thought_chunk` content shape

Primary constructor, `createAgentThoughtChunk` (`index.js:23709-23723`):
```js
function createAgentThoughtChunk(content, messageId, meta3) {
  if (messageId) {
    return { sessionUpdate: "agent_thought_chunk", messageId, content, ...(meta3 ? { _meta: meta3 } : {}) };
  }
  return { sessionUpdate: "agent_thought_chunk", content, ...(meta3 ? { _meta: meta3 } : {}) };
}
```
`content` is an ACP `ContentBlock` — in the reasoning-delta path it's always
`{ type: "text", text: "..." }` (`createAgentReasoningEventUpdates`, `index.js:30863-30870`;
`createReasoningUpdates`, `index.js:30905-30912`, which reads `item["summary"]` falling back to
`item["content"]`). Example:
```json
{
  "sessionUpdate": "agent_thought_chunk",
  "content": { "type": "text", "text": "Let me check the test output first." }
}
```
`messageId` is included only when the caller has one (streaming-chunk correlation); most reasoning
paths (`createAgentReasoningEventUpdates`, `createReasoningUpdates`) don't pass one, so in practice
most `agent_thought_chunk` events omit `messageId`.

### Child subagent thought chunks

No special-cased kind or suppression — `agent_thought_chunk` for a subagent's own reasoning is an
ordinary item update on the child's `item/started`/`item/completed`/delta stream, and goes through
the exact same `createUpdateEvent` → `session.update(updateEvent,
this.subagents.notificationSessionId(notification))` path documented in §A. So: when the client
supports subagents, a child's `agent_thought_chunk` arrives as `agent_thought_chunk` **under the
child's own sessionId**; when the client doesn't, it's re-attributed to the root sessionId via
`rootAttributed` (§A) — same routing as agent_message_chunk/tool_call, not a different kind and not
suppressed.

---

## F. MCP-elicitation permission-completion diff, 1.10.0 → 1.12.0

Both versions implement `CodexElicitationHandler.handleElicitation`'s standalone-MCP-permission
fallback path (used when `canUsePermissionFallback` applies — i.e., no native form-elicitation
support, so codex-acp synthesizes a `session/request_permission` call instead). The diff is in what
happens **after** the client responds to that `session/request_permission` call:

**1.10.0** (`codex-acp-1.10.0/dist/index.js:26161-26166`):
```js
if (correlatedCallId !== void 0 && result.action === "accept") {
  await this.connection.notify(methods.client.session.update, {
    sessionId: params.threadId,
    update: { sessionUpdate: "tool_call_update", toolCallId: correlatedCallId, status: "in_progress" }
  });
}
return result;
```
Only ever notifies `tool_call_update` (status `in_progress`) when there **is** a correlated tool
call **and** the user accepted. Any other outcome (decline/cancel, or no correlated call) — no
follow-up notification at all.

**1.12.0** (`codex-acp-1.12.0/dist/index.js:26618-26643`):
```js
if (correlatedCallId !== void 0) {
  if (result.action === "accept") {
    await this.connection.notify(methods.client.session.update, {
      sessionId: params.threadId,
      update: { sessionUpdate: "tool_call_update", toolCallId: correlatedCallId, status: "in_progress" }
    });
  }
} else {
  try {
    await this.connection.notify(methods.client.session.update, {
      sessionId: params.threadId,
      update: {
        sessionUpdate: "tool_call_update",
        toolCallId: request.toolCall.toolCallId,
        status: "completed",
        title: request.toolCall.title,
        content: request.toolCall.content,
        rawOutput: { action: result.action }
      }
    });
  } catch (error51) {
    logger.error("Failed to finalize standalone MCP elicitation tool call", error51);
  }
}
return result;
```

**What changed, and what an ACP client (AgentDeck) would observe:** the `correlatedCallId` branch is
unchanged (same in_progress-on-accept behavior). The new behavior is the `else` branch: when there is
**no** correlated tool call (i.e., the synthetic standalone tool call codex-acp itself created via
`buildMcpPermissionRequest`'s `nextStandaloneToolCallId()` for the permission-request prompt, not a
pre-existing MCP tool call), 1.12.0 now sends a **second, explicit `tool_call_update`** marking that
synthetic tool call `"completed"`, echoing back the same `title`/`content` the permission request
used and recording the user's decision (accept/decline/cancel) in `rawOutput.action`. In 1.10.0 that
standalone synthetic tool call was left dangling at `status: "pending"` forever (from
`buildMcpPermissionRequest`, `title`/`content`/`status: "pending"`) — the client never got a
completion signal for it. In 1.12.0, AgentDeck (or any ACP client rendering the tool-call timeline)
will now see the standalone MCP-permission tool call properly transition to `completed` regardless of
whether the user accepted, declined, or cancelled — closing a UI state that previously stayed stuck
"pending" for that specific fallback case. `publishAcceptedMcpToolApproval` (the sibling function for
the *non*-fallback, direct-tool-approval path) is byte-identical between the two versions
(`1.10.0:26405` / `1.12.0:26894`) — this diff is scoped to the standalone-elicitation fallback only.

---

## G. Re-verified claims

### G.1 — Single active prompt per session, second `session/prompt` supersedes the first — **TRUE**

`trackActivePrompt(sessionId)` (`index.js:34186-34237`) unconditionally does
`this.activePrompts.set(sessionId, activePrompt)` (`index.js:34237`) on every new `prompt()` call —
it does **not** call `requestCancel()` on any prior entry. Supersession instead happens via identity
checks sprinkled through the still-running first prompt's continuation code, e.g.:
```js
// index.js:34255 area (observePromptRequestCancellation) and reused at 34493/34588/34677
if (this.activePrompts.get(sessionState.sessionId) !== activePrompt) { ... }
```
Once a second `prompt()` overwrites the map entry, every subsequent guard in the first prompt's
in-flight continuation sees `this.activePrompts.get(sessionId) !== activePrompt` and bails out /
treats itself as stale — that is the "supersession check." (`getInterruptibleTurnId`,
`index.js:34349-34368`, and `interruptSessionTurn`, `index.js:34316-34347`, additionally provide the
explicit turn-interrupt path used by `Cancel`/`Close` requests, calling
`this.codexAcpClient.turnInterrupt(...)`.) Net effect matches the claim: only one prompt is "live" per
session; a second `session/prompt` effectively interrupts/supersedes the first via this map-identity
mechanism rather than an explicit synchronous cancel call at `trackActivePrompt` time.

### G.2 — `available_commands_update` after session creation, `$`-prefixed skills — **TRUE, unchanged from 1.10.0**

`CodexCommands.publish` (`index.js:30115-30131`) sends:
```js
await session.update({ sessionUpdate: "available_commands_update", availableCommands });
```
`buildAvailableCommands` (`index.js:30143-30160`) merges built-in commands with skills:
```js
for (const skill of entry.skills) {
  const name = `$${skill.name}`;
  ...
  commands.set(name, { name, description, input: null });
}
```
i.e. `$`-prefixed dollar-sign entries for skills, alongside plain-named built-ins (`plan`, etc., from
`getBuiltinCommands`, `index.js:30165+`). Diffed against `codex-acp-1.10.0/dist/index.js` — the
`buildAvailableCommands`/`$${skill.name}` construction is present there too (same shape); no material
change found in this pass.

### G.3 — `session/prompt` is the inference entrypoint; `mcpCapabilities` shows ACP-transport unsupported, HTTP supported — **TRUE**

`session/prompt` wired at `index.js:35210` (`.onRequest(methods.agent.session.prompt, (ctx) =>
getAgent().prompt(ctx.params, ctx.signal))`), implementation `AgentImpl.prompt` at
`index.js:34370+`. Exact `mcpCapabilities` object in the real `initialize` response
(`index.js:32343-32347`):
```json
{ "acp": false, "http": true, "sse": false }
```
Confirms ACP-transport MCP unsupported, HTTP-transport MCP supported, SSE unsupported.

### G.4 — MCP tool-call naming: dot pattern, not double-underscore — **TRUE, identical 1.10.0 and 1.12.0**

`createMcpToolCallUpdate`, byte-identical in both versions
(`1.12.0:23035-23044` / `1.10.0:22907-22915`):
```js
async function createMcpToolCallUpdate(item) {
  return {
    ...await createExecuteToolCallUpdate(
      item,
      `mcp.${item.server}.${item.tool}`,           // <-- dot notation, e.g. "mcp.github.search_issues"
      createMcpRawInput(item.server, item.tool, item.arguments),
      createMcpRawOutput(item.result, item.error)
    ),
    _meta: { is_mcp_tool_call: true }
  };
}
```
This is the `title` on the initial `tool_call` (via `createExecuteToolCallUpdate`'s `title` param,
`index.js:23133-23142`), not a `name` field — MCP tool-call completions never carry `name` (§ name
correction above). The naming pattern is `` mcp.<server>.<tool> `` (dot-separated), **not**
`mcp__server__tool` (double underscore) — no occurrence of double-underscore MCP naming was found
anywhere in either dist bundle (only grep hit for `mcp__` is the unrelated startup-tool-call title
`` `mcp__${serverName}__startup` `` at `index.js:25317`, which is codex-acp's own synthetic startup
notice, not a naming convention for real MCP tool calls).

For the permission-request path, `buildMcpPermissionRequest` (`index.js:26385-26430`) uses
descriptive English titles instead of the dot-name, e.g. `"MCP tool call approval"` / `"Question from
MCP server"` (form/openai-form mode) or `"MCP server requests to open a URL"` (url mode) — it does
**not** reuse the `mcp.server.tool` title for the synthesized standalone permission tool call; that
convention is specific to real `mcpToolCall` items, not the synthetic elicitation-fallback tool call.

### G.5 — `_meta.steering.supported: true` still advertised — **TRUE**

`initialize` response `_meta` (`index.js:32354-32357`):
```js
_meta: {
  steering: { supported: true },
  goal: { version: GOAL_EXTENSION_VERSION, controlMethod: GOAL_CONTROL_METHOD, actions: [...GOAL_CONTROL_ACTIONS] },
  jetbrains: { air: { version: 1, capabilities: [
    "sessionFailure", "agentFileChangeReport", "nativeSubagentSessions", "asyncTasks", "recommendedValue"
  ] } }
}
```
Confirms `steering.supported: true` (`index.js:32355-32356`), and incidentally gives the full,
authoritative list of AIR extension capability keys the agent advertises in 1.12.0.

---

## Summary of file:line index for quick reference

| Topic | Key lines (1.12.0) |
|---|---|
| `name`-correction: `createDynamicToolCallUpdate` | 23046-23051 |
| `name`-correction: `createImageViewUpdate` (`view_image`) | 23052-23069 |
| `name`-correction: `dynamicToolCall` completion (tool_call_update) | 24108-24124 |
| `name`-correction: `completeCommandExecutionEvent` | 25334-25341 |
| `functionToolName` / `commandToolName` | 22933-22945 |
| `ACPSessionConnection.update` | 22147-22152 |
| Subagent `materialize`/`finish` (parent-routed) | 24019-24069 |
| `CodexSubagentSubscriptions.discover` (child live routing) | 27991-28012 |
| `interactionSessionId` (request_permission routing) | 28065-28069 |
| `notificationSessionId` (child-vs-root pick) | 23917-23920 |
| `clientSupportsSubagents` | 31815-31821 |
| `streamNativeThreadHistory` | 33767-33858 |
| async task spawn/state | 32114-32180 |
| `clientSupportsAirCapability` | 22868-22875 |
| `_session/async_task/stop` handler | 32395-32406 |
| `forkSession` / `zForkSessionRequest` / `zForkSessionResponse` | 32761-32772 / 19576-19582 / 19043-19048 |
| `threadReadHistory` cursor loop | 29584-29604 |
| `sessionCapabilities` (fork/delete advertised) | 32316-32323 |
| `parseAgentFileChangeReportRequest` | 24217-24229 |
| `publishAgentFileChangeReport` / report shapes | 33893-33920 / 24238-24252 |
| `createAgentThoughtChunk` | 23709-23723 |
| MCP elicitation completion diff | 1.12.0:26618-26643 vs 1.10.0:26161-26166 |
| `initialize` `_meta` (steering/goal/air) | 32354-32357 |
| `mcpCapabilities` | 32343-32347 |
| `createMcpToolCallUpdate` naming | 23035-23044 |
