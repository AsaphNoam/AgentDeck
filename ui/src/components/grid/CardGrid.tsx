import { DndContext, type DragEndEvent } from "@dnd-kit/core";
import { SortableContext, arrayMove, rectSortingStrategy } from "@dnd-kit/sortable";
import { useEffect, useMemo, useRef, useState } from "react";
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

export function CardGrid() {
  const agents = useAgentStore((state) => state.agents);
  const order = useAgentStore((state) => state.order);
  const setOrder = useAgentStore((state) => state.setOrder);
  const density = useUiStore((state) => state.density);
  const setDensity = useUiStore((state) => state.setDensity);
  const groupLayout = useUiStore((state) => state.groupLayout);
  const setGroupLayout = useUiStore((state) => state.setGroupLayout);
  const toggleGroupCollapsed = useUiStore((state) => state.toggleGroupCollapsed);
  const transcripts = useTranscriptStore((state) => state.byAgent);
  const [showNewAgent, setShowNewAgent] = useState(false);

  const loaded = useRef(false);

  useEffect(() => {
    void getLayout().then((layout) => {
      setOrder(layout.order);
      setDensity(layout.density);
      setGroupLayout(layout.groups ?? {});
      loaded.current = true;
    });
  }, [setDensity, setGroupLayout, setOrder]);

  useEffect(() => {
    if (!loaded.current) return;
    const handle = window.setTimeout(() => {
      void putLayout({ order, density, groups: groupLayout });
    }, 400);
    return () => window.clearTimeout(handle);
  }, [density, groupLayout, order]);

  const ids = useMemo(() => {
    const known = new Set(Object.keys(agents));
    return [...order.filter((id) => known.has(id)), ...Object.keys(agents).filter((id) => !order.includes(id))];
  }, [agents, order]);

  const grouped = useMemo(() => groupAgents(ids.map((id) => agents[id]).filter(Boolean)), [agents, ids]);

  const onDragEnd = (event: DragEndEvent) => {
    if (!event.over || event.active.id === event.over.id) return;
    const oldIndex = ids.indexOf(String(event.active.id));
    const newIndex = ids.indexOf(String(event.over.id));
    setOrder(arrayMove(ids, oldIndex, newIndex));
  };

  if (ids.length === 0) return (
    <>
      <EmptyState onNewAgent={() => setShowNewAgent(true)} />
      <NewAgentModal open={showNewAgent} onClose={() => setShowNewAgent(false)} />
    </>
  );

  return (
    <section className="grid-view">
      <div className="grid-toolbar">
        <h1>Agents</h1>
        <button type="button" onClick={() => setShowNewAgent(true)}>New agent</button>
        <DensityControl />
      </div>
      <DndContext onDragEnd={onDragEnd}>
        <SortableContext items={ids} strategy={rectSortingStrategy}>
          <div className="group-stack">
            {grouped.map((group) => {
              const collapsed = groupLayout[group.key]?.collapsed ?? false;
              return (
                <section className="agent-group" key={group.key}>
                  <header className="agent-group-header">
                    <button type="button" onClick={() => toggleGroupCollapsed(group.key)} aria-expanded={!collapsed}>
                      {collapsed ? ">" : "v"}
                    </button>
                    <strong>{group.label}</strong>
                    <span>{group.agents.length} agents</span>
                    <span>{summary(group.agents)}</span>
                    {group.key !== "_ungrouped" && (
                      <button
                        type="button"
                        className="group-release"
                        onClick={() => {
                          if (window.confirm(`Release group ${group.label}?`)) void releaseGroup(group.key);
                        }}
                      >
                        Release group
                      </button>
                    )}
                  </header>
                  {!collapsed && (
                    <div className="card-grid" style={{ gridTemplateColumns: `repeat(${density.perRow}, minmax(0, 1fr))`, gap: density.gap }}>
                      {group.agents.map((agent) => (
                        <AgentCard key={agent.agent_id} agent={agent} lastLine={lastAssistantLine(transcripts[agent.agent_id])} />
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
      <NewAgentModal open={showNewAgent} onClose={() => setShowNewAgent(false)} />
    </section>
  );
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
