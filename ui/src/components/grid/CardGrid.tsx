import { DndContext, type DragEndEvent } from "@dnd-kit/core";
import { SortableContext, arrayMove, rectSortingStrategy } from "@dnd-kit/sortable";
import { useEffect, useMemo } from "react";
import { getLayout, putLayout } from "../../api/client";
import type { TranscriptEvent } from "../../api/types";
import { useAgentStore } from "../../store/agentStore";
import { useTranscriptStore } from "../../store/transcriptStore";
import { useUiStore } from "../../store/uiStore";
import { AgentCard } from "./AgentCard";
import { CardContextMenu } from "./CardContextMenu";
import { DensityControl } from "./DensityControl";
import { EmptyState } from "./EmptyState";

export function CardGrid() {
  const agents = useAgentStore((state) => state.agents);
  const order = useAgentStore((state) => state.order);
  const setOrder = useAgentStore((state) => state.setOrder);
  const density = useUiStore((state) => state.density);
  const setDensity = useUiStore((state) => state.setDensity);
  const transcripts = useTranscriptStore((state) => state.byAgent);

  useEffect(() => {
    void getLayout().then((layout) => {
      setOrder(layout.order);
      setDensity(layout.density);
    });
  }, [setDensity, setOrder]);

  useEffect(() => {
    const handle = window.setTimeout(() => {
      void putLayout({ order, density });
    }, 400);
    return () => window.clearTimeout(handle);
  }, [density, order]);

  const ids = useMemo(() => {
    const known = new Set(Object.keys(agents));
    return [...order.filter((id) => known.has(id)), ...Object.keys(agents).filter((id) => !order.includes(id))];
  }, [agents, order]);

  const onDragEnd = (event: DragEndEvent) => {
    if (!event.over || event.active.id === event.over.id) return;
    const oldIndex = ids.indexOf(String(event.active.id));
    const newIndex = ids.indexOf(String(event.over.id));
    setOrder(arrayMove(ids, oldIndex, newIndex));
  };

  if (ids.length === 0) return <EmptyState />;

  return (
    <section className="grid-view">
      <div className="grid-toolbar">
        <h1>Agents</h1>
        <DensityControl />
      </div>
      <DndContext onDragEnd={onDragEnd}>
        <SortableContext items={ids} strategy={rectSortingStrategy}>
          <div className="card-grid" style={{ gridTemplateColumns: `repeat(${density.perRow}, minmax(0, 1fr))`, gap: density.gap }}>
            {ids.map((id) => (
              <AgentCard key={id} agent={agents[id]} lastLine={lastAssistantLine(transcripts[id])} />
            ))}
          </div>
        </SortableContext>
      </DndContext>
      <CardContextMenu />
    </section>
  );
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
