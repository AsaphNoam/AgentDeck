import { DndContext, type DragEndEvent, type DragOverEvent } from "@dnd-kit/core";
import { SortableContext, arrayMove, rectSortingStrategy } from "@dnd-kit/sortable";
import { useEffect, useMemo, useRef, useState, type ComponentProps, type KeyboardEvent } from "react";
import { Link } from "react-router-dom";
import { getLayout, putLayout, releaseGroup } from "../../api/client";
import type { AgentState, AgentStatus } from "../../api/types";
import { useAgentStore } from "../../store/agentStore";
import { useTranscriptStore } from "../../store/transcriptStore";
import { useUiStore } from "../../store/uiStore";
import { AgentCard } from "./AgentCard";
import { CardContextMenu } from "./CardContextMenu";
import { DensityControl } from "./DensityControl";
import { DashboardChatPane } from "./DashboardChatPane";
import { EmptyState } from "./EmptyState";
import { NewAgentModal } from "../../features/launch/NewAgentModal";
import { useProjects } from "../../api/config";
import { useTasks } from "../../api/tasks";
import { needsAttention } from "../../features/tasks/TasksPage";
import { Button, ConfirmDialog, PageHeader } from "../ui";

// projectID scopes which agents the grid shows; fixedProject locks New Agent to a
// launch target and hides its project picker. They are separate because a scoped
// route can show a project's live agents while that project is NOT a current
// catalog member (deleted/hand-removed): fixing the launch to it would hide the
// picker and every launch would be rejected as `unknown project` (FS-02.R43/A25,
// INV §10). The caller passes fixedProject only for an active catalog member.
/** TaskAttentionLink is the dashboard's one task-shaped element: how many tasks
 *  in view need a person, opening the Tasks view (FS-02.R44). The card grid
 *  itself gains no task object — an armed task has no agent and so no card. */
function TaskAttentionLink({ projectID }: { projectID?: string }) {
  const { data: tasks, isError } = useTasks(projectID);
  const count = (tasks ?? []).filter((task) => needsAttention(task)).length;
  if (!projectID) return null;
  return (
    <Link className="task-attention-link" to={`/tasks?project=${encodeURIComponent(projectID)}`}>
      {isError ? "Task attention unavailable" : `${count} task${count === 1 ? "" : "s"} ${count === 1 ? "needs" : "need"} attention`}
    </Link>
  );
}

export function CardGrid({ projectID, projectTitle, fixedProject }: { projectID?: string; projectTitle?: string; fixedProject?: string } = {}) {
  const agents = useAgentStore((state) => state.agents);
  const agentsHydrated = useAgentStore((state) => state.hydrated);
  const agentsHydrating = useAgentStore((state) => state.hydrating);
  const order = useAgentStore((state) => state.order);
  const setOrder = useAgentStore((state) => state.setOrder);
  const density = useUiStore((state) => state.density);
  const setDensity = useUiStore((state) => state.setDensity);
  const groupLayout = useUiStore((state) => state.groupLayout);
  const setGroupLayout = useUiStore((state) => state.setGroupLayout);
  const toggleGroupCollapsed = useUiStore((state) => state.toggleGroupCollapsed);
  const pushError = useUiStore((state) => state.pushError);
  const [showNewAgent, setShowNewAgent] = useState(false);
  const [releaseGroupLabel, setReleaseGroupLabel] = useState<string | null>(null);
  const [releaseGroupError, setReleaseGroupError] = useState("");
  const [expanded, setExpanded] = useState<string[]>([]);
  const [refusedDrop, setRefusedDrop] = useState(false);
  const projects = useProjects();

  const loaded = useRef(false);

  // A failed read is reported and still arms saving: leaving `loaded` false swallowed
  // the error and silently disabled layout persistence for the whole session, so every
  // later expand, reorder, density, or collapse was lost with nothing on screen saying
  // so (INV §7/§8). Saving what the person now sees keeps the screen and the stored
  // layout in agreement.
  useEffect(() => {
    void getLayout()
      .then((layout) => {
        setOrder(layout.order ?? []);
        setDensity(layout.density);
        setGroupLayout(layout.groups ?? {});
        setExpanded((layout.expanded ?? []).slice(0, 4));
        loaded.current = true;
      })
      .catch((err: unknown) => {
        loaded.current = true;
        pushError("Loading layout failed", err instanceof Error ? err.message : String(err));
      });
  }, [pushError, setDensity, setGroupLayout, setOrder]);

  useEffect(() => {
    if (!loaded.current) return;
    const handle = window.setTimeout(() => {
      putLayout({ order, density, groups: groupLayout, expanded }).catch((err: unknown) =>
        pushError("Saving layout failed", err instanceof Error ? err.message : String(err)),
      );
    }, 400);
    return () => window.clearTimeout(handle);
  }, [density, expanded, groupLayout, order, pushError]);

  useEffect(() => {
    if (!agentsHydrated) return;
    setExpanded((current) => {
      const next = current.filter((id) => agents[id] && !agents[id].archived);
      return next.length === current.length ? current : next;
    });
  }, [agents, agentsHydrated, expanded]);

  const globalIds = useMemo(() => {
    const safeOrder = order ?? [];
    const known = new Set(Object.keys(agents));
    return [...safeOrder.filter((id) => known.has(id)), ...Object.keys(agents).filter((id) => !safeOrder.includes(id))]
      .filter((id) => !agents[id].archived);
  }, [agents, order]);

  const ids = useMemo(
    () => globalIds.filter((id) => !projectID || agents[id].project === projectID),
    [agents, globalIds, projectID],
  );

  const grouped = useMemo(() => groupAgents(ids.map((id) => agents[id]).filter(Boolean)), [agents, ids]);

  // FS-02.R61 — a chat agent that newly enters `waiting_input` opens its own pane, so a
  // conversation stopped for an approval is in front of the person instead of behind a
  // badge. Eligibility reuses the set the grid already renders: `grouped` is scoped to
  // this project, a collapsed section (R18) renders no cards and so opens none — a pane
  // nobody can see would spend one of R48's four slots and evict a visible one — and a
  // terminal agent has no pane to open (R46). No second copy of what is on screen.
  const openableIDs = useMemo(
    () => new Set(
      grouped
        .filter((group) => !(groupLayout[group.key]?.collapsed ?? false))
        .flatMap((group) => group.agents)
        .filter((agent) => agent.interface === "chat")
        .map((agent) => agent.agent_id),
    ),
    [grouped, groupLayout],
  );

  // The transition is read from the durable `state` every `state_update` carries, not
  // from the `notification` event the bus emits for the same moment: NotificationsEditor's
  // per-type mute list filters that stream, so muting a toast would silently disable an
  // unrelated layout behaviour, and the server emits it once and never replays it, so a
  // reconnecting tab would see nothing (TS-08.R49).
  //
  // The record below is state derived from the connection, so it is reseeded rather than
  // fired on every boundary crossing (INV §1): while the store is hydrating, and until it
  // has hydrated once, the effect only writes what it observed. That is what makes R61's
  // "newly observed" rule true instead of aspirational — a first load, a reload, a
  // reconnect, and an agent first seen already waiting all open nothing. It lives here,
  // beside `expanded`, so the previous state exists in exactly one place and `agentStore`
  // keeps its single-writer role (INV §2); it is created and discarded with the mounted
  // grid, which is the mechanism behind R61's limit that a pane opens only while a grid is
  // on screen to observe the transition.
  const lastObservedState = useRef(new Map<string, AgentStatus>());
  useEffect(() => {
    const observed = lastObservedState.current;
    const settled = agentsHydrated && !agentsHydrating;
    const opening: string[] = [];
    for (const agent of Object.values(agents)) {
      const previous = observed.get(agent.agent_id);
      observed.set(agent.agent_id, agent.state);
      if (!settled || previous === undefined || previous === "waiting_input") continue;
      if (agent.state === "waiting_input" && openableIDs.has(agent.agent_id)) opening.push(agent.agent_id);
    }
    for (const id of [...observed.keys()]) if (!agents[id]) observed.delete(id);
    if (opening.length === 0) return;
    // Transitions React committed together are ordered by the server time they carry, so
    // "the last four" is the four that happened last rather than whichever order the
    // agent record happens to enumerate (R61).
    opening.sort((a, b) => agents[a].updated_at - agents[b].updated_at);
    // One state update for the whole batch, through the same append-and-cap a click
    // takes, so the cap, its least-recently-used eviction, and the persisted list cannot
    // drift from the manual path (INV §2, §10).
    setExpanded((current) => opening.reduce(expandPane, current));
  }, [agents, agentsHydrated, agentsHydrating, openableIDs]);

  // dnd-kit derives sortable indices and measured-rect transforms from the items
  // each SortableContext is handed, so a drag's live preview is only as correct as
  // that list. One list spanning both blocks let a drag near the running/stopped
  // boundary shift cards on the other side of it, which FS-02.R45/A28 forbid — the
  // cross-block drop was refused at drop time, after the preview had already moved
  // them. Each block therefore gets its own context, in the order its cards render.
  // Expanded ids stay in their block's list: they are already `disabled` through
  // useSortable, but an expanded pane still mounts a sortable node and still spans
  // one column, so omitting it would make every neighbour's preview transform
  // compute over a layout that is not on screen (FS-02.R47, INV §1).
  const blockIDs = (agents: AgentState[], running: boolean) =>
    agents.filter((agent) => agent.running === running).map((agent) => agent.agent_id);

  const toggleExpanded = (agentId: string) => {
    setExpanded((current) => current.includes(agentId)
      ? current.filter((id) => id !== agentId)
      : expandPane(current, agentId));
  };

  const hasExpandedOnGrid = expanded.some((id) => ids.includes(id));
  const collapseAll = () => {
    const gridIDs = new Set(ids);
    setExpanded((current) => current.filter((id) => !gridIDs.has(id)));
  };

  const markExpandedUsed = (agentId: string) => {
    setExpanded((current) => current.includes(agentId)
      ? [...current.filter((id) => id !== agentId), agentId]
      : current);
  };

  const cyclePaneFocus = (event: KeyboardEvent<HTMLDivElement>) => {
    if (!event.ctrlKey || !event.altKey || (event.key !== "ArrowDown" && event.key !== "ArrowUp")) return;
    if ((event.target as Element).closest(".composer")?.querySelector(".composer-picker")) return;
    // Bound at the grid container, not at one group section: FS-02.R48's cap of
    // four is a whole-grid cap and R50 orders "the panes as displayed", so one
    // pane in each of two task groups (R18) must still cycle between them.
    const grid = event.currentTarget;
    const panes = [...grid.querySelectorAll<HTMLElement>("[data-agent-pane]")];
    if (panes.length < 2) return;
    const current = panes.findIndex((pane) => pane.contains(document.activeElement));
    const direction = event.key === "ArrowDown" ? 1 : -1;
    const target = panes[(current < 0 ? (direction > 0 ? 0 : panes.length - 1) : current + direction + panes.length) % panes.length];
    const composer = target.querySelector<HTMLTextAreaElement>(".composer textarea");
    if (!composer) return;
    event.preventDefault();
    composer.focus();
    target.scrollIntoView({ block: "nearest" });
  };

  // dnd-kit refuses a cross-block drop only when the pointer is released, so the card
  // followed the pointer into the other block and then snapped home with nothing said.
  // Marking the refusal while the drag is still in flight states it before the release
  // without interrupting the drag (FS-02.R53).
  const onDragOver = ({ active, over }: DragOverEvent) => {
    setRefusedDrop(!!over && agents[String(active.id)]?.running !== agents[String(over.id)]?.running);
  };

  const onDragEnd = (event: DragEndEvent) => {
    setRefusedDrop(false);
    if (!event.over || event.active.id === event.over.id) return;
    const activeID = String(event.active.id);
    const overID = String(event.over.id);
    // Manual drag order cannot override the running-first boundary (FS-02.R45), so a
    // drop onto the other block reorders nothing and writes no layout — returning here
    // keeps both `arrayMove` and the persisted order untouched.
    if (agents[activeID]?.running !== agents[overID]?.running) return;
    const oldIndex = ids.indexOf(activeID);
    const newIndex = ids.indexOf(overID);
    // The flat manual order stays the source for the move, so a same-block drag commits
    // exactly the order it committed before the split existed (FS-02.R12/R14).
    const reordered = arrayMove(ids, oldIndex, newIndex);
    if (!projectID) {
      setOrder(reordered);
      return;
    }
    setOrder(mergeScopedOrder(globalIds, ids, reordered));
  };

  const body =
    ids.length === 0 ? (
		<>
			<TaskAttentionLink projectID={projectID} />
			<EmptyState onNewAgent={() => setShowNewAgent(true)} />
		</>
    ) : (
      <section className="grid-view" data-ui="dashboard">
      <PageHeader
        className="grid-toolbar"
        eyebrow="Live operations"
        title={projectTitle ?? "Agents"}
        actions={<><TaskAttentionLink projectID={projectID} />{hasExpandedOnGrid && <Button type="button" onClick={collapseAll}>Collapse all</Button>}<Button variant="primary" type="button" onClick={() => setShowNewAgent(true)}>New agent</Button><DensityControl /></>}
        data-slot="header"
      />
      <DndContext onDragEnd={onDragEnd} onDragOver={onDragOver} onDragCancel={() => setRefusedDrop(false)}>
          <div className="group-stack" data-slot="groups" data-drop={refusedDrop ? "refused" : undefined} onKeyDown={cyclePaneFocus}>
            {grouped.map((group) => {
              const collapsed = groupLayout[group.key]?.collapsed ?? false;
              return (
                <section className="agent-group" data-ui="agent-group" data-state={collapsed ? "collapsed" : "expanded"} key={group.key}>
                  <header className="agent-group-header" data-slot="header">
                    <button type="button" onClick={() => toggleGroupCollapsed(group.key)} aria-expanded={!collapsed}>
                      {collapsed ? ">" : "v"}
                    </button>
                    <strong>{group.label}</strong>
                    <span data-slot="summary">{group.agents.length} agents</span>
                    <span data-slot="summary">{summary(group.agents)}</span>
                    {group.key !== "_ungrouped" && (
                      <button
                        type="button"
                        className="group-release"
                        onClick={() => { setReleaseGroupError(""); setReleaseGroupLabel(group.key); }}
                      >
                        Release group
                      </button>
                    )}
                  </header>
                  {!collapsed && (
                    <div className="card-grid" data-slot="grid" style={{ gridTemplateColumns: `repeat(${density.perRow}, minmax(0, 1fr))`, gap: density.gap }}>
                      {[true, false].map((running) => (
                        // SortableContext renders no element of its own, so the cards
                        // below stay direct children of the grid track.
                        <SortableContext key={running ? "running" : "stopped"} items={blockIDs(group.agents, running)} strategy={rectSortingStrategy}>
                          {group.agents.filter((agent) => agent.running === running).map((agent) => (
                            <LiveAgentCard
                              key={agent.agent_id}
                              agent={agent}
                              projectColor={projects.data?.[agent.project]?.color}
                              projectTitle={projects.data?.[agent.project]?.title}
                              showProject={!projectID}
                              expanded={expanded.includes(agent.agent_id)}
                              onToggle={() => toggleExpanded(agent.agent_id)}
                              onUse={() => markExpandedUsed(agent.agent_id)}
                            />
                          ))}
                        </SortableContext>
                      ))}
                    </div>
                  )}
                </section>
              );
            })}
          </div>
      </DndContext>
      <CardContextMenu />
    </section>
    );

  // The NewAgentModal is kept at a stable position in the returned tree (always
  // the second child of this fragment) so it is NOT remounted when `body` flips
  // between the empty and populated branches. A remount during the 0→1 launch
  // transition would unmount the open modal mid-mutation, so its
  // onSuccess→onClose would never fire and the overlay would stay stuck.
  return (
    <>
      {body}
      <NewAgentModal open={showNewAgent} onClose={() => setShowNewAgent(false)} fixedProject={fixedProject} />
      {releaseGroupLabel && (
        <ConfirmDialog
          open
          title={`Release group ${releaseGroupLabel}?`}
          confirmLabel="Release group"
          destructive
          onCancel={() => { setReleaseGroupError(""); setReleaseGroupLabel(null); }}
          onConfirm={() => releaseGroup(releaseGroupLabel).then(() => setReleaseGroupLabel(null)).catch((err: unknown) => { const message = err instanceof Error ? err.message : String(err); setReleaseGroupError(message); pushError("Release group failed", message); })}
        >
          <p>This stops every agent in the group. Their sessions remain available to resume later.</p>
          {releaseGroupError && <p className="form-error">{releaseGroupError}</p>}
        </ConfirmDialog>
      )}
    </>
  );
}

/** expandPane appends an id under FS-02.R48's cap of four expanded panes, evicting the
 *  least-recently-used entry at the head. A person's click and R61's automatic opening
 *  both go through it, so neither can grow its own cap or recency rule (INV §2). */
function expandPane(current: string[], agentId: string) {
  return current.includes(agentId) ? current : [...current, agentId].slice(-4);
}

export function mergeScopedOrder(globalIDs: string[], scopedIDs: string[], reorderedScopedIDs: string[]) {
  const scoped = new Set(scopedIDs);
  let nextScopedID = 0;
  return globalIDs.map((id) => (scoped.has(id) ? reorderedScopedIDs[nextScopedID++] : id));
}

function groupAgents(items: AgentState[]) {
  const map = new Map<string, AgentState[]>();
  for (const agent of items) {
    const key = agent.group?.trim() || "_ungrouped";
    map.set(key, [...(map.get(key) ?? []), agent]);
  }
  return [...map.entries()]
    .sort(([a], [b]) => {
      if (a === "_ungrouped") return 1;
      if (b === "_ungrouped") return -1;
      return a.localeCompare(b);
    })
    .map(([key, agents]) => ({
      key,
      label: key === "_ungrouped" ? "Ungrouped" : key,
      // Running agents lead each section and the manual order survives inside each
      // block, so supervision starts with live work (FS-02.R45). `running` is the sole
      // test — the live `state` values never move a card (FS-12.R37).
      agents: [...agents.filter((agent) => agent.running), ...agents.filter((agent) => !agent.running)],
    }));
}

function summary(agents: AgentState[]) {
  const counts = agents.reduce<Record<string, number>>((acc, agent) => {
    acc[agent.state] = (acc[agent.state] ?? 0) + 1;
    return acc;
  }, {});
  return Object.entries(counts).map(([state, count]) => `${count} ${state}`).join(" · ");
}

function LiveAgentCard(props: Omit<ComponentProps<typeof AgentCard>, "lastLine">) {
  const lastLine = useTranscriptStore((state) => state.previewByAgent[props.agent.agent_id] ?? "");
  return <AgentCard {...props} lastLine={lastLine}>{props.expanded ? <DashboardChatPane agent={props.agent} /> : null}</AgentCard>;
}
