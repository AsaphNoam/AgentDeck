# Multi-tab stress fixture

This fixture starts the production AgentDeck HTTP server and embedded UI with an isolated temporary
home. It launches one `agentdecker` orchestrator and six `teammate` workers, all displayed as the
`claude` backend with the `haiku` model. The provider is the repository's deterministic fake ACP, so
the run needs no credentials, network access, or provider spend.

From the repository root:

```sh
go run ./scripts/stress-fixture
```

Open the printed URL in one tab, then in four or more tabs. Useful load controls:

```sh
go run ./scripts/stress-fixture --workers 6 --chunks 1000 --chunk-bytes 128 --delay-ms 3 --port 4399
```

On the current build, five populated tabs load normally. A sixth tab reaches the AgentDeck shell and
shows `SSE open`, but remains on `Loading project…`; closing one of the older tabs lets it complete.
That is the connection-starvation reproduction. The exact threshold is browser-specific, so start at
four and add tabs one at a time.

The fixture rejects unbounded values, writes only beneath the printed temporary home, never removes
that home, and stops on Ctrl-C.
