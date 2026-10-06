import { createContext, Fragment, useCallback, useContext, useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import type { TranscriptEvent } from "../../api/types";
import { projectTurns, turnOutcome } from "./turnActivity";

// Where a row sits in the turn lifecycle. A reasoning disclosure reads it to
// start open while its turn and child scope are live (FS-03.R73); completed
// activity starts closed (FS-03.R75).
export interface ThoughtScope {
  agentId: string;
  turnKey: string;
  live: boolean;
}

export const ThoughtScopeContext = createContext<ThoughtScope | null>(null);

export function useThoughtScope() {
  return useContext(ThoughtScopeContext);
}

// At most this many remembered choices per mounted transcript (TS-08.R102).
const MAX_CHOICES = 256;

// useTurnChoices holds which completed turns the person opened. It is local to
// the mounted transcript: a reload or another source starts every completed
// turn closed, and a newer turn finishing never closes an older one.
export function useTurnChoices(source: string) {
  const [state, setState] = useState<{ source: string; open: string[] }>({ source, open: [] });
  const open = state.source === source ? state.open : [];
  const setOpen = useCallback((key: string, value: boolean) => {
    setState((current) => {
      const kept = (current.source === source ? current.open : []).filter((item) => item !== key);
      return { source, open: value ? [...kept, key].slice(-MAX_CHOICES) : kept };
    });
  }, [source]);
  return { isOpen: (key: string) => open.includes(key), setOpen };
}

export type TurnChoices = ReturnType<typeof useTurnChoices>;

// TurnList renders the turn projection with a surface's own row renderer. Each
// proven completed turn with activity gets one Show activity control at its
// first activity position; the hidden rows stay in place when opened
// (TS-08.R100, R105).
export function TurnList({ agentId, events, scope = "", choices, renderEvents }: {
  agentId: string;
  events: TranscriptEvent[];
  scope?: string;
  choices: TurnChoices;
  renderEvents: (events: TranscriptEvent[]) => ReactNode;
}) {
  return (
    <>
      {projectTurns(events, scope).map((turn) => {
        const hidden = turn.parts.some((part) => part.hidden);
        const open = choices.isOpen(turn.key);
        const ids = turn.parts.flatMap((part, index) => (part.hidden ? [`turn-${agentId}-${turn.key}-${index}`] : []));
        const outcome = turnOutcome(turn);
        // The terminal row renders after the outcome so the label sits with its turn.
        const parts = turn.completed ? withoutLast(turn.parts) : turn.parts;
        const end = turn.completed ? turn.parts[turn.parts.length - 1].events.slice(-1) : [];
        return (
          <div key={turn.key} data-ui="turn-activity" data-state={hidden ? (open ? "expanded" : "collapsed") : undefined} data-turn={turn.key}>
            <ThoughtScopeContext.Provider value={{ agentId, turnKey: turn.key, live: !turn.completed }}>
              {parts.map((part, index) => {
                if (!part.hidden) return <Fragment key={index}>{renderEvents(part.events)}</Fragment>;
                const id = `turn-${agentId}-${turn.key}-${index}`;
                return (
                  <Fragment key={index}>
                    {id === ids[0] && (
                      <div className="turn-activity-toggle">
                        <button type="button" className="tool-toggle" data-slot="trigger" aria-expanded={open} aria-controls={open ? ids.join(" ") : undefined} onClick={() => choices.setOpen(turn.key, !open)}>
                          {open ? "▾ Hide activity" : "▸ Show activity"}
                        </button>
                      </div>
                    )}
                    {open && <div className="turn-activity-content" data-slot="content" id={id}>{renderEvents(part.events)}</div>}
                  </Fragment>
                );
              })}
              {outcome && <p className="turn-outcome" role="status">{outcome}</p>}
              {renderEvents(end)}
            </ThoughtScopeContext.Provider>
          </div>
        );
      })}
    </>
  );
}

function withoutLast(parts: { hidden: boolean; events: TranscriptEvent[] }[]) {
  const last = parts[parts.length - 1];
  const trimmed = { ...last, events: last.events.slice(0, -1) };
  return trimmed.events.length ? [...parts.slice(0, -1), trimmed] : parts.slice(0, -1);
}

// useFocusReturn keeps keyboard focus when completion hides the row that held
// it: focus moves to that turn's activity control instead of the page body
// (FS-03.R75, TS-08.R105).
export function useFocusReturn(listRef: RefObject<HTMLElement | null>) {
  const focused = useRef<{ element: HTMLElement; turn: string } | null>(null);
  useLayoutEffect(() => {
    const last = focused.current;
    const list = listRef.current;
    if (!last || !list || last.element.isConnected) return;
    focused.current = null;
    const active = document.activeElement;
    if (active && active !== document.body && list.contains(active)) return;
    const trigger = list.querySelector<HTMLElement>(`[data-turn="${last.turn}"] > .turn-activity-toggle > button`);
    trigger?.focus();
  });
  return (event: { target: EventTarget }) => {
    const element = event.target as HTMLElement;
    const turn = element.closest?.("[data-turn]")?.getAttribute("data-turn");
    focused.current = turn ? { element, turn } : null;
  };
}
