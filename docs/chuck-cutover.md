# Moving an AgentDeck installation to Chuck

Chuck is AgentDeck renamed. It installs as a new product and never reads, moves, or repairs an
AgentDeck home (FS-10.R25–R26). An existing installation moves through this one supervised,
offline cutover. Nothing here runs automatically, and nothing deletes the source.

| | Source (AgentDeck) | Destination (Chuck) |
|---|---|---|
| Home | `~/.agentdeck` (or `$AGENTDECK_HOME`) | `~/.chuck` (or `$CHUCK_HOME`) |
| Install tree | `~/Library/Application Support/AgentDeck` | `~/Library/Application Support/Chuck` |
| Command | `agentdeck` | `chuck` |
| Resident operator role | `agentdecker` | `chucky` |
| tmux sessions | `agentdeck-*` | `chuck-*` |
| Phone address | `https://agentdeck.<tailnet>.ts.net` | `https://chuck.<tailnet>.ts.net` |

Never run both products against the same state or worktrees. The examples below assume the default
homes; substitute your own paths if either variable is set.

## 1. Stop everything that writes

1. In AgentDeck, stop or let finish every pipeline run and task, and pause automatic work.
2. Send or copy out any unsent chat drafts and annotation trays you care about. (Chuck copies the
   browser's drafts forward on the same origin, but do not rely on that for anything important.)
3. Stop the dashboard: `agentdeck dashboard stop`.
4. Stop every agent process the dashboard does not own, including terminal agents:
   `tmux ls 2>/dev/null | grep '^agentdeck-'` must print nothing; kill any session it lists with
   `tmux kill-session -t <name>`. `pgrep -fl agentdeck` must print nothing.

Live sessions do not continue across the cutover. Chuck starts new sessions; a native provider
session may or may not resume afterwards, and that is not promised.

## 2. Keep a recoverable source

```sh
ts=$(date +%Y%m%d-%H%M%S)
cp -a ~/.agentdeck ~/agentdeck-snapshot-$ts
```

The dashboard is stopped, so `state.db` (with any `-wal`/`-shm` files) is consistent. Do not edit
the snapshot or `~/.agentdeck` from here on; either one is the recovery source.

## 3. Inventory what must survive

Record, from the source:

- roles: `ls ~/.agentdeck/roles` (note any you edited, including `agentdecker.json`);
- projects, pipelines and resources: `ls ~/.agentdeck/projects ~/.agentdeck/pipelines
  ~/.agentdeck/project-resources`;
- history: `ls ~/.agentdeck/sessions | wc -l` and
  `sqlite3 ~/.agentdeck/state.db 'select count(*) from agents; select count(*) from tasks;'`;
- owned worktrees: `sqlite3 ~/.agentdeck/state.db 'select repo_path, checkout_path from project_worktrees;'`
  and `git -C <repo> worktree list` for each repository;
- frozen session paths, which resume and file reads use verbatim:
  `sqlite3 ~/.agentdeck/state.db "select agent_id, cwd, add_dirs from sessions where cwd like '%/.agentdeck/%' or add_dirs like '%/.agentdeck/%';"`;
- every path that names the source home:
  `grep -rl '/.agentdeck' ~/.agentdeck --include='*.json'`.

Starting with an empty Chuck home or leaving any of these behind is a separate, explicit choice.

## 4. Prepare the Chuck home offline

1. Install Chuck without starting it:
   `curl -fsSL https://github.com/AsaphNoam/Chuck/releases/latest/download/install.sh | bash -s -- --no-start`.
2. Copy the source to an absent destination: `test ! -e ~/.chuck && cp -a ~/.agentdeck ~/.chuck`.
3. Remove only derived or per-process files from the destination; their owners recreate them:
   `rm -rf ~/.chuck/cache ~/.chuck/mcp ~/.chuck/hooks ~/.chuck/.pid-* ~/.chuck/remote/tailscale`.
   The operating skill is republished as `operating-chuck` on first start.
4. Adapt the resident role. If you never edited it, delete `~/.chuck/roles/agentdecker.json` and
   let Chuck seed Chucky. If you did, rename it to `chucky.json` (only when that file does
   not exist) and keep its content. Then replace the role id `agentdecker` with `chucky` where
   configuration names it: `default_role` in `config.json` and `orchestrator_role` or role fields in
   `pipelines/*.json`. Leave transcripts and history untouched.
5. Move owned worktrees and rewrite only the paths that must resolve:
   - in JSON configuration, replace the source home path with the destination home path in each
     file step 3 listed;
   - in `~/.chuck/state.db`, rewrite the source-home prefix in worktree ownership rows and in the
     frozen session snapshots (working directory, extra directories, frozen prompt). Transcripts,
     messages and other history stay untouched:
     ```sh
     src="$HOME/.agentdeck/"; dst="$HOME/.chuck/"
     sqlite3 ~/.chuck/state.db "
       update project_worktrees set checkout_path = replace(checkout_path, '$src', '$dst');
       update sessions set cwd = replace(cwd, '$src', '$dst'),
         add_dirs = replace(add_dirs, '$src', '$dst'),
         system_prompt = replace(system_prompt, '$src', '$dst');"
     ```
     Then both of these must print nothing; fix any `launch_config_json` hit by hand, and only
     where the value is a path a resumed process uses:
     ```sh
     sqlite3 ~/.chuck/state.db "select agent_id from sessions where cwd like '%/.agentdeck/%' or add_dirs like '%/.agentdeck/%' or system_prompt like '%/.agentdeck/%';"
     sqlite3 ~/.chuck/state.db "select agent_id from sessions where launch_config_json like '%/.agentdeck/%';"
     ```
   - repair each repository's links: `git -C <repo> worktree repair <new checkout path>…`, then
     confirm `git -C <repo> worktree list` shows the Chuck paths.
   The source checkouts stay registered nowhere once repaired; the snapshot still holds them.

## 5. Verify before starting new work

Start Chuck (`chuck dashboard start`) with automatic work still paused, then confirm:

- archived agents and their transcripts open and read correctly; tasks, pipelines and context
  links list as before;
- edited roles appear unchanged and Chucky is present;
- each project's resources and owned worktrees resolve, and a worktree project shows its checkout;
- a stopped agent that worked in an owned worktree resumes in the `~/.chuck` checkout, and its file
  reads show destination files (rehearse once with the source home moved aside, then put it back);
- a new Chucky chat and a new implementer launch, receive the operating skill, and can message;
- `tmux ls` shows only `chuck-*` sessions for new terminal agents.

Then resume automatic work. Turn remote control back on and re-pair each phone at the new
`chuck` address; the old phone pairing no longer reaches anything.

## Recovery

If any check fails, stop Chuck (`chuck dashboard stop`) and repair the destination, or delete
`~/.chuck` and repeat step 4. To return to AgentDeck instead, stop Chuck, run
`git -C <repo> worktree repair <old checkout path>…` for any repaired repository, and start
AgentDeck against `~/.agentdeck` (or a copy of the snapshot). Remove the AgentDeck install tree,
the source home and the snapshot only after Chuck has run your real work successfully.
