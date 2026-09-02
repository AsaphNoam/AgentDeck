# Build and run pipelines

Pipelines are saved, model-neutral sequential templates executed as durable runs with explicit stage model assignments, text artifacts, routing, and operator recovery.

- Validate a template before starting it. Starting a run selects the project, goal, named inputs, and a backend/model for every stage.
- AgentDecker may propose a template or exact run configuration only for human review. A proposal neither saves nor starts anything; the operator confirms those actions in AgentDeck. Other roles do not gain proposal authority from this skill.
- A stage's attempt ends only when AgentDeck accepts its result. A refused call records nothing and leaves the attempt owing a result; follow retryable repair guidance and report again. Once AgentDeck accepts either a completed result or `blocked`, that attempt is final and further reporting for it is refused.
- `blocked` returns control to the human. Continue begins the next eligible attempt only through AgentDeck's human boundary; the blocked stage cannot continue inside its existing chat turn.
- Use Retry only when AgentDeck exposes the run or stage as retry-eligible. A stale or already-reported result cannot be repeated; other refusals follow their structured retry guidance.
- Supervise durable run state and artifacts through AgentDeck. Stop prevents further stage starts; it does not erase the run record.

Use the pipeline tool definitions and CLI help for exact arguments and current result fields.
