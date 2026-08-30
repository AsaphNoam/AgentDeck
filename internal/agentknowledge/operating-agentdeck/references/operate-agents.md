# Operate agents

Use AgentDeck to launch a role in a project, then supervise the resulting agent by its AgentDeck identity. Configuration edits affect future launches; a running or persisted session retains its frozen project, role, permission, directory, backend, model, effort, and provider-resolution snapshot until the operator explicitly chooses a lifecycle action that changes it.

- Launch through the dashboard or `agentdeck <role>@<project>`. A bare matching launch may resume a single inactive session; use the explicit new/resume controls when AgentDeck reports ambiguity.
- Chat provides the managed transcript and inline AgentDeck controls. Terminal runs the supported provider CLI while preserving AgentDeck persistence, hooks, and messaging contracts.
- Resume keeps the frozen session configuration by default. Runtime switch changes the selected interface/backend/model while preserving the logical session; a provider change may use a bounded one-shot history primer.
- Stop ends the live process without deleting its durable session. Archive and resume remain available for known sessions.
- Role and project files are user-owned configuration. Project resources live in AgentDeck's project-scoped resource directory and are composed into supported launches; do not treat state database files as editable configuration.

Read the active tool or CLI help for exact inputs and availability. A prompt cannot grant an interface, provider capability, permission, or lifecycle operation that AgentDeck does not expose.
