import { DndContext, type DragEndEvent } from "@dnd-kit/core";
import { SortableContext, arrayMove, rectSortingStrategy } from "@dnd-kit/sortable";
import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { getLayout, putLayout, releaseGroup } from "../../api/client";
import type { AgentState, TranscriptEvent } from "../../api/types";
import { useAgentStore } from "../../store/agentStore";
import { useTranscriptStore } from "../../store/transcriptStore";
import { useUiStore } from "../../store/uiStore";
import { AgentCard } from "./AgentCard";
import { CardContextMenu } from "./CardContextMenu";
import { DensityControl } from "./DensityControl";
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
  const { data: tasks } = useTasks(projectID);
  const count = (tasks ?? []).filter((task) => needsAttention(task)).length;
  if (!projectID) return null;
  return (
    <Link className="task-attention-link" to={`/tasks?project=${encodeURIComponent(projectID)}`}>
      {count} task{count === 1 ? "" : "s"} need attention
    </Link>
  );
}

export function CardGrid({ projectID, projectTitle, fixedProject }: { projectID?: string; projectTitle?: string; fixedProject?: string } = {}) {
  const agents = useAgentStore((state) => state.agents);
  const order = useAgentStore((state) => state.order);
  const setOrder = useAgentStore((state) => state.setOrder);
  const density = useUiStore((state) => state.density);
  const setDensity = useUiStore((state) => state.setDensity);
  const groupLayout = useUiStore((state) => state.groupLayout);
  const setGroupLayout = useUiStore((state) => state.setGroupLayout);
  const toggleGroupCollapsed = useUiStore((state) => state.toggleGroupCollapsed);
  const transcripts = useTranscriptStore((state) => state.byAgent);
  const pushError = useUiStore((state) => state.pushError);
  const [showNewAgent, setShowNewAgent] = useState(false);
  const [releaseGroupLabel, setReleaseGroupLabel] = useState<string | null>(null);
  const [releaseGroupError, setReleaseGroupError] = useState("");
  const projects = useProjects();

  const loaded = useRef(false);

  useEffect(() => {
    void getLayout().then((layout) => {
      setOrder(layout.order ?? []);
      setDensity(layout.density);
      setGroupLayout(layout.groups ?? {});
      loaded.current = true;
    });
  }, [setDensity, setGroupLayout, setOrder]);

  useEffect(() => {
    if (!loaded.current) return;
    const handle = window.setTimeout(() => {
      putLayout({ order, density, groups: groupLayout }).catch((err: unknown) =>
        pushError("Saving layout failed", err instanceof Error ? err.message : String(err)),
      );
    }, 400);
    return () => window.clearTimeout(handle);
  }, [density, groupLayout, order]);

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

  const onDragEnd = (event: DragEndEvent) => {
    if (!event.over || event.active.id === event.over.id) return;
    const oldIndex = ids.indexOf(String(event.active.id));
    const newIndex = ids.indexOf(String(event.over.id));
    const reordered = arrayMove(ids, oldIndex, newIndex);
    if (!projectID) {
      setOrder(reordered);
      return;
    }
    setOrder(mergeScopedOrder(globalIds, ids, reordered));
  };

  const body =
    ids.length === 0 ? (
		<section className="grid-view" data-ui="dashboard">
			<PageHeader className="grid-toolbar" eyebrow="Live operations" title={projectTitle ?? "Agents"} actions={<><TaskAttentionLink projectID={projectID} /><Button variant="primary" type="button" onClick={() => setShowNewAgent(true)}>New agent</Button><DensityControl /></>} data-slot="header" />
			<EmptyState onNewAgent={() => setShowNewAgent(true)} />
		</section>
    ) : (
      <section className="grid-view" data-ui="dashboard">
      <PageHeader
        className="grid-toolbar"
        eyebrow="Live operations"
        title={projectTitle ?? "Agents"}
        actions={<><TaskAttentionLink projectID={projectID} /><Button variant="primary" type="button" onClick={() => setShowNewAgent(true)}>New agent</Button><DensityControl /></>}
        data-slot="header"
      />
      <DndContext onDragEnd={onDragEnd}>
        <SortableContext items={ids} strategy={rectSortingStrategy}>
          <div className="group-stack" data-slot="groups">
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
                      {group.agents.map((agent) => (
                        <AgentCard
                          key={agent.agent_id}
                          agent={agent}
                          lastLine={lastAssistantLine(transcripts[agent.agent_id])}
                          projectColor={projects.data?.[agent.project]?.color}
                          projectTitle={projects.data?.[agent.project]?.title}
                          showProject={!projectID}
                        />
                      ))}
                    </div>
                  )}
                </section>
              );
            })}
          </div>
        </SortableContext>
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
    .map(([key, agents]) => ({ key, label: key === "_ungrouped" ? "Ungrouped" : key, agents }));
}

function summary(agents: AgentState[]) {
  const counts = agents.reduce<Record<string, number>>((acc, agent) => {
    acc[agent.state] = (acc[agent.state] ?? 0) + 1;
    return acc;
  }, {});
  return Object.entries(counts).map(([state, count]) => `${count} ${state}`).join(" · ");
}

function lastAssistantLine(events: TranscriptEvent[] = []) {
  for (let i = events.length - 1; i >= 0; i--) {
    const event = events[i];
    if ((event.kind ?? event.type) !== "assistant_text") continue;
    const text = String(event.text ?? event.delta ?? "");
    if (text.trim()) return text.trim();
  }
  return "";
}
