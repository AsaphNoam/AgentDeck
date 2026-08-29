import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import {
  PipelineAPIError,
  useDeletePipelineRun,
  usePipelineControl,
  usePipelineRun,
  usePipelineRuns,
} from "../../api/pipelines";
import type { PipelineAttemptAgents, PipelineRunDetail } from "../../schemas/pipeline";
import { useAgentStore } from "../../store/agentStore";

export function attemptTranscriptPath(agentID: string, live: boolean) {
  return `/${live ? "agent" : "archive"}/${agentID}`;
}

export function RunsLedger() {
  const query = usePipelineRuns();
  const pages = query.data?.pages ?? [];
  const runs = useMemo(() => {
    const byID = new Map(pages.flatMap((page) => page.runs).map((run) => [run.run_id, run]));
    return [...byID.values()].sort((a, b) => b.updated_at.localeCompare(a.updated_at));
  }, [pages]);
  const total = pages[0]?.total ?? runs.length;

  if (query.isLoading) return <RunsSkeleton />;
  if (query.error) return <p className="form-error">{query.error.message}</p>;
  if (runs.length === 0) return <section className="pipeline-empty pipeline-empty-ledger"><strong>No runs yet</strong><p>Start a run to turn a reusable template into supervised work.</p></section>;

  return (
    <section className={query.isFetching ? "pipeline-ledger pipeline-updating" : "pipeline-ledger"} data-ui="pipeline-run-list" data-slot="list">
      <div className="pipeline-ledger-head" aria-hidden="true"><span>Run</span><span>Stage</span><span>Project</span><span>State</span><span>Updated</span><span /></div>
      {runs.map((run) => (
        <Link className="pipeline-ledger-row" key={run.run_id} to={`/pipelines/runs/${encodeURIComponent(run.run_id)}`} data-slot="item">
          <span className="pipeline-ledger-identity"><strong>{run.display_name || run.template_id}</strong><small><code>{run.run_id}</code></small></span>
          <span><small>Current stage</small>{run.current_stage_title || run.current_stage_id || "Complete"}</span>
          <span><small>Project</small>{run.project}</span>
          <span className={`pipeline-state pipeline-state-${run.state}`}>{run.final_outcome || run.state}</span>
          <time dateTime={run.updated_at}>{formatRelative(run.updated_at)}</time>
          <span className="pipeline-row-arrow" aria-hidden="true">→</span>
          {run.diagnostics.map((diagnostic) => <small className="form-error pipeline-ledger-diagnostic" key={diagnostic.code}>{diagnostic.message}</small>)}
        </Link>
      ))}
      <footer className="pipeline-ledger-footer">
        <span>{runs.length} of {total} retained runs</span>
        {query.hasNextPage
          ? <button type="button" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>{query.isFetchingNextPage ? "Loading…" : "More runs"}</button>
          : <span className="pipeline-history-complete">Complete history loaded</span>}
      </footer>
    </section>
  );
}

// FS-14.R45: only an attempt that arrives after this page's first render is
// newly appended; the initial list rides the page entrance instead of each row
// replaying it, and a background refetch of unchanged attempts appends nothing.
function useAppendedAttempts(ids: string[], loaded: boolean) {
  const key = ids.join("\n");
  const seen = useRef<Set<string> | null>(null);
  const [appended, setAppended] = useState<Set<string>>(() => new Set());

  useEffect(() => {
    if (!loaded) return;
    const current = key ? key.split("\n") : [];
    if (!seen.current) {
      seen.current = new Set(current);
      return;
    }
    const known = seen.current;
    const fresh = current.filter((id) => !known.has(id));
    for (const id of current) known.add(id);
    if (fresh.length > 0) setAppended(new Set(fresh));
  }, [key, loaded]);

  return appended;
}

export function RunDetail({ runID, onDeleted }: { runID: string; onDeleted: () => void }) {
  const detail = usePipelineRun(runID);
  const continueRun = usePipelineControl("continue");
  const retryRun = usePipelineControl("retry");
  const stopRun = usePipelineControl("stop");
  const deleteRun = useDeletePipelineRun();
  const [continuation, setContinuation] = useState("");
  const [error, setError] = useState<string | null>(null);
  const appended = useAppendedAttempts(detail.data?.attempts.map((item) => item.attempt_id) ?? [], detail.isSuccess);

  if (detail.isLoading) return <RunDetailSkeleton />;
  if (!detail.data) {
    // FS-14.R43: a deleted run explains itself in product language; any other
    // read failure keeps its transport detail visible rather than claiming the
    // record is gone.
    const deleted = detail.error instanceof PipelineAPIError && detail.error.status === 404;
    return <section className="pipeline-missing">
      <p className="pipeline-eyebrow">Run unavailable</p>
      <h2>{deleted ? "This run is gone." : "This run could not be loaded."}</h2>
      <p>{deleted ? "It may have been deleted in another tab. Other runs still retain their frozen setup." : "The run record could not be read just now. Return to Runs and open it again."}</p>
      {!deleted && detail.error && <p className="form-error">{detail.error.message}</p>}
      <Link to="/pipelines/runs">Return to Runs</Link>
    </section>;
  }

  const data = detail.data;
  const run = data.run;
  const stage = data.template.stages.find((item) => item.id === run.current_stage_id);
  const attempt = data.attempts.find((item) => item.attempt_id === run.current_attempt_id);
  const paused = run.state === "paused";
  const blocked = paused && run.attention_reason === "blocked" && attempt?.report_outcome === "blocked";
  const approval = paused && run.pending_action === "await_approval";
  const canContinue = blocked || approval;
  const canRetry = paused && !approval && run.attention_reason !== "loop_limit_reached";
  // A restart pause stopped the stage agent, Continue rejects the state, and an
  // ordinary chat resume of that agent mints an unrelated generation whose report
  // is refused forever — so Open agent here is a dead end. Withhold it and name
  // the route that works (FS-14.R48).
  const recovered = paused && run.attention_reason.startsWith("restart_");
  const terminal = run.state === "completed" || run.state === "stopped";
  const busy = continueRun.isPending || retryRun.isPending || stopRun.isPending || deleteRun.isPending;

  const control = (action: "continue" | "retry" | "stop") => {
    setError(null);
    const mutation = action === "continue" ? continueRun : action === "retry" ? retryRun : stopRun;
    mutation.mutate(
      { id: run.run_id, revision: run.revision, input: action === "continue" ? continuation : "" },
      { onSuccess: () => setContinuation(""), onError: (reason) => setError(messageOf(reason)) },
    );
  };

  return (
    <article className={detail.isFetching ? "pipeline-run-page pipeline-updating" : "pipeline-run-page"} data-ui="pipeline-run">
      <Link className="pipeline-back-link" to="/pipelines/runs">← All runs</Link>
      <section className="pipeline-run-hero" data-slot="live">
        <div className="pipeline-run-kicker"><code>{run.run_id}</code><span className={`pipeline-state pipeline-state-${run.state}`}>{run.final_outcome || run.state}</span></div>
        <div className="pipeline-run-title"><div><h2>{run.display_name || data.template.title}</h2><p>{run.goal}</p></div><div className="pipeline-live-stage"><small>{terminal ? "Final position" : "Current stage"}</small><strong>{stage?.title ?? (run.current_stage_id || "Complete")}</strong>{attempt && <span>Visit {attempt.visit_no} · attempt {attempt.attempt_no}</span>}</div></div>
        {run.attention_reason && <div className="pipeline-warning"><strong>Needs attention</strong><p>{humanize(run.attention_reason)}</p>{recovered && <p>The stage agent was stopped when AgentDeck restarted, so its chat can no longer report against this run. Retry the stage to run it again with a fresh agent.</p>}</div>}
        <div className="pipeline-run-actions" data-slot="actions">
          {run.current_agent_id && !recovered && <Link className="pipeline-link-button" to={`/agent/${run.current_agent_id}`}>Open agent</Link>}
          {canContinue && <button type="button" disabled={busy || (blocked && !continuation.trim())} onClick={() => control("continue")}>{approval ? "Approve and continue" : "Continue"}</button>}
          {canRetry && <button type="button" disabled={busy} onClick={() => control("retry")}>Retry stage</button>}
          {!terminal && <button type="button" className="btn-danger" disabled={busy} onClick={() => control("stop")}>Stop run</button>}
          {terminal && <button type="button" className="btn-danger" disabled={busy} onClick={() => deleteRun.mutate(run.run_id, { onSuccess: onDeleted, onError: (reason) => setError(messageOf(reason)) })}>Delete run record</button>}
        </div>
        {blocked && <label className="form-field pipeline-continuation"><span>New input for the blocked stage</span><textarea rows={3} value={continuation} onChange={(event) => setContinuation(event.target.value)} /></label>}
        {error && <p className="form-error">{error}</p>}
      </section>

      <div className="pipeline-run-workspace">
        <section className="pipeline-timeline" data-slot="timeline">
          <div className="pipeline-timeline-heading"><div><p className="pipeline-eyebrow">Execution timeline</p><h3>{data.attempts.length} attempt{data.attempts.length === 1 ? "" : "s"}</h3></div><span>Oldest → newest</span></div>
          <ol>
            {data.attempts.map((item) => <TimelineAttempt key={item.attempt_id} data={data} attemptID={item.attempt_id} appended={appended.has(item.attempt_id)} current={item.attempt_id === run.current_attempt_id} attention={item.attempt_id === run.current_attempt_id && Boolean(run.attention_reason)} />)}
          </ol>
        </section>
        <aside className="pipeline-run-rail">
          <details className="pipeline-disclosure" open data-slot="setup"><summary>Frozen setup <span>{data.template.stages.length} stages</span></summary><div className="pipeline-disclosure-body">
            <dl className="pipeline-rail-facts"><div><dt>Project</dt><dd>{run.project}</dd></div><div><dt>Template</dt><dd>{data.template.title}</dd></div><div><dt>Started</dt><dd>{formatDate(run.created_at)}</dd></div><div><dt>Revision</dt><dd>{run.revision}</dd></div></dl>
            <ol className="pipeline-setup-list">{data.template.stages.map((item, index) => { const runtime = data.assignments[item.id]; return <li key={item.id}><span>{index + 1}</span><div><strong>{item.title}</strong><small>{[runtime?.backend, runtime?.model, runtime?.effort].filter(Boolean).join(" · ") || "No runtime"}</small></div></li>; })}</ol>
          </div></details>
          <details className="pipeline-disclosure" data-slot="values"><summary>Named values <span>{data.values.length}</span></summary><div className="pipeline-disclosure-body">{data.values.length === 0 ? <p className="pipeline-empty">No values recorded.</p> : <dl className="pipeline-value-list">{data.values.map((value) => <div key={value.name}><dt>{value.name}<small>{value.source_kind}{value.source_attempt_id ? ` · ${value.source_attempt_id}` : ""}</small></dt><dd>{value.value}</dd></div>)}</dl>}</div></details>
        </aside>
      </div>
    </article>
  );
}

function TimelineAttempt({ data, attemptID, appended, current, attention }: { data: PipelineRunDetail; attemptID: string; appended: boolean; current: boolean; attention: boolean }) {
  const item = data.attempts.find((attempt) => attempt.attempt_id === attemptID)!;
  const stage = data.template.stages.find((candidate) => candidate.id === item.stage_id);
  const effort = data.assignments[item.stage_id]?.effort;
  const agents = data.agents_by_attempt[item.attempt_id];
  const outcome = item.report_outcome || (item.state === "completed" ? "unreported" : item.state);
  const [expanded, setExpanded] = useState(current || attention);

  useEffect(() => {
    if (current || attention) setExpanded(true);
  }, [attention, current]);

  return <li className={`${current ? "pipeline-timeline-item pipeline-timeline-current" : "pipeline-timeline-item"}${appended ? " pipeline-timeline-appended" : ""}`} data-slot="attempt">
    <span className="pipeline-timeline-line" aria-hidden="true" />
    <details open={expanded} onToggle={(event) => setExpanded(event.currentTarget.open)}>
      <summary>
        <span className="pipeline-stage-number">{item.attempt_no}</span>
        <span className="pipeline-attempt-identity"><strong>{stage?.title ?? item.stage_id}</strong><small>Visit {item.visit_no} · {[item.backend, item.model, effort].filter(Boolean).join(" · ")}</small></span>
        <span className={`pipeline-state pipeline-state-${outcome}`}>{humanize(outcome)}</span>
        <span className="pipeline-disclosure-chevron" aria-hidden="true">⌄</span>
      </summary>
      <div className="pipeline-attempt-body">
        {item.report_summary ? <p className="pipeline-result-summary">{item.report_summary}</p> : <p className="pipeline-unreported">No stage result was reported for this attempt.</p>}
        {item.report_details && <section><h4>Result details</h4><pre>{item.report_details}</pre></section>}
        {item.report_checks && <section><h4>Checks</h4><pre>{item.report_checks}</pre></section>}
        {agents && <AttemptAgents agents={agents} />}
      </div>
    </details>
  </li>;
}

function AttemptAgents({ agents }: { agents: PipelineAttemptAgents }) {
  const summaries = agents.stage_agent ? [agents.stage_agent, ...agents.delegated_agents] : agents.delegated_agents;
  if (summaries.length === 0) return null;
  return <section className="pipeline-attempt-agents" data-slot="agents"><div className="pipeline-agent-heading"><h4>Agents</h4><span>{agents.delegated_running_count > 0 && `${agents.delegated_running_count} delegated still running`}{agents.delegated_running_count > 0 && agents.delegated_total > agents.delegated_agents.length && " · "}{agents.delegated_total > agents.delegated_agents.length && `Showing ${agents.delegated_agents.length} of ${agents.delegated_total} delegated`}</span></div><div className="pipeline-agent-grid">{summaries.map((summary, index) => <AttemptAgentCard key={`${summary.agent_id}-${index}`} summary={summary} stage={index === 0 && Boolean(agents.stage_agent)} />)}</div></section>;
}

function AttemptAgentCard({ summary, stage }: { summary: PipelineAttemptAgents["delegated_agents"][number] | NonNullable<PipelineAttemptAgents["stage_agent"]>; stage: boolean }) {
  const live = useAgentStore((state) => state.agents[summary.agent_id]);
  const available = live ? true : summary.available;
  const running = live?.running ?? summary.running;
  const state = live?.state ?? summary.state;
  const name = live?.name || summary.name;
  const preview = live?.detail || summary.preview;
  const href = available ? attemptTranscriptPath(summary.agent_id, running) : null;
  const content = <><span className={`pipeline-agent-dot pipeline-agent-dot-${state}`} /><span><strong>{name}</strong><small>{stage ? "Stage agent" : "Delegated work"} · {humanize(state)}</small><em>{preview || "No recent activity"}</em></span><span aria-hidden="true">{href ? "↗" : "—"}</span></>;
  return href ? <Link className="pipeline-agent-card" to={href}>{content}</Link> : <div className="pipeline-agent-card pipeline-agent-unavailable">{content}</div>;
}

function RunsSkeleton() { return <div className="pipeline-ledger pipeline-skeleton" aria-label="Loading runs"><span /><span /><span /><span /></div>; }
function RunDetailSkeleton() { return <div className="pipeline-run-page pipeline-skeleton" aria-label="Loading run"><span /><span /><span /><span /></div>; }
function messageOf(reason: unknown) { return reason instanceof Error ? reason.message : String(reason); }
function humanize(value: string) { return value.replace(/_/g, " "); }
function formatDate(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(date); }
function formatRelative(value: string) { const date = new Date(value); if (Number.isNaN(date.getTime())) return value; const delta = Date.now() - date.getTime(); if (delta < 60_000) return "just now"; if (delta < 3_600_000) return `${Math.floor(delta / 60_000)}m ago`; if (delta < 86_400_000) return `${Math.floor(delta / 3_600_000)}h ago`; return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric" }).format(date); }

// Compatibility wrapper for older focused tests and embedders.
export function RunBrowser({ selectedID }: { selectedID: string | null; onSelect?: (id: string | null) => void }) {
  return selectedID ? <RunDetail runID={selectedID} onDeleted={() => undefined} /> : <RunsLedger />;
}
