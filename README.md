# tamowolkin

A webhook-driven agent pool that reacts to Linear issues. Each issue assigned to you is classified as *easy* or *hard*:

- **Easy** → the agent cuts a fresh branch in an isolated git worktree, writes the code, commits, pushes, and opens a pull request.
- **Hard** → the agent produces a plan file under `PLANS_DIR` and stops. No code is written.

Multiple workers run in parallel without stepping on each other: every task gets its own worktree and branch.

## Prerequisites

- Go 1.26+
- `git`
- [`gh`](https://cli.github.com/) — authenticated (`gh auth login`) and with access to the target repo
- An `agent` executable in `PATH` — this is the CLI that actually runs Claude. It is invoked as `agent -p --trust <prompt>` (and `agent -p --plan --trust <prompt>` for planning).
- A Linear workspace with a webhook pointed at `POST /webhook` on this server, signed with a shared secret.

## Install and build

```sh
git clone <this repo>
cd tamowolkin
go build -o tamowolkin ./cmd
```

## Configure

tamowolkin reads config from environment variables. A `.env` file in the working directory is loaded automatically.

### Required

| Variable | Purpose |
|---|---|
| `LINEAR_API_KEY` | Linear personal API key. |
| `LINEAR_WEBHOOK_SECRET` | Shared secret for `Linear-Signature` HMAC verification. Must match the secret configured in Linear. |
| `LINEAR_EMAIL` | Only issues assigned to this user email are processed; everything else returns 200 and is ignored. |

### Optional (with defaults)

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `3000` | HTTP port. |
| `WORKER_COUNT` | `3` | Number of agent workers in the pool. |
| `PROJECT_PATH` | `./` | Path to the target git repository the agent edits. |
| `BASE_BRANCH` | `main` | Branch new work is cut from. |
| `WORKTREES_DIR` | `./tamowolkin-worktrees` | Where per-task git worktrees are created. |
| `PLANS_DIR` | `./tamowolkin-plans` | Where plan files for hard tasks are written. |
| `EASY_DESCRIPTION_PATH` | `./tamowolkin-easy.md` | Override the definition of "easy" (see below). |
| `PLAN_PROMPT_PATH` | `./tamowolkin-plan-prompt.md` | Override the planning prompt. |
| `CODE_PROMPT_PATH` | `./tamowolkin-code-prompt.md` | Override the coding prompt. |

### Prompt overrides

Any of the three files — `tamowolkin-easy.md`, `tamowolkin-plan-prompt.md`, `tamowolkin-code-prompt.md` — will be read from disk if present and passed through to the agent verbatim. If a file is missing, a sensible built-in default is used. The path itself is configurable via the matching `*_PATH` env var.

- **Easy description**: a checklist describing when a task qualifies as easy. Used by the classifier.
- **Plan prompt**: the instruction the agent follows when asked to produce a plan for a hard task.
- **Code prompt**: the instruction the agent follows when asked to implement an easy task inside a worktree.

## Example `.env`

```
LINEAR_API_KEY=lin_api_xxx
LINEAR_WEBHOOK_SECRET=supersecret
LINEAR_EMAIL=you@example.com

PROJECT_PATH=/path/to/your/repo
BASE_BRANCH=main
WORKER_COUNT=3
```

## Run

```sh
./tamowolkin
```

Expected startup output:

```
[tamowolkin] Using easy description from ./tamowolkin-easy.md
[tamowolkin] ./tamowolkin-plan-prompt.md not found, using built-in default plan prompt
[tamowolkin] ./tamowolkin-code-prompt.md not found, using built-in default code prompt
[tamowolkin] Starting a pool with 3 agents
[tamowolkin] Listening on :3000
```

## Linear setup

1. In Linear, go to **Settings → API → Webhooks** and create a webhook:
   - **URL**: `https://your-host/webhook`
   - **Resource types**: `Issues`
   - **Signing secret**: the value of `LINEAR_WEBHOOK_SECRET`.
2. Assign an issue to the user whose email matches `LINEAR_EMAIL`. tamowolkin will classify and route it.

The server verifies every request with an HMAC-SHA256 of the raw body against the `Linear-Signature` header. Unsigned or badly-signed requests get `401`.

## What happens per task

1. Linear sends an `Issue` webhook; tamowolkin verifies the signature and enqueues `(identifier, title, description, branchName)`.
2. A worker picks up the job and runs the easy classifier (`agent -p --trust <easy prompt>`) in `PROJECT_PATH`. Expected response: `{ easy: true }` or `{ easy: false }`.
3. **If easy:**
   1. `git fetch origin $BASE_BRANCH` (best-effort).
   2. `git worktree add $WORKTREES_DIR/<issue-id> -b <branch> origin/$BASE_BRANCH` — branch name comes from Linear (`issue.branchName`), falling back to `tamowolkin/<issue-id>`.
   3. `agent -p --trust <code prompt>` runs inside the worktree.
   4. `git add -A`, commit any remaining changes with `<ID>: <Title>`.
   5. `git push -u origin <branch>`.
   6. `gh pr create --base $BASE_BRANCH --head <branch> --title "<ID>: <Title>" --body "<description>\n\nLinear: <ID>"`.
4. **If hard:** `agent -p --plan --trust <plan prompt>` runs in `PROJECT_PATH` and its stdout is written to `$PLANS_DIR/<issue-id>-<UTC timestamp>.md`.

## Concurrency

Each task runs in its own worktree on its own branch, so N workers can process N tasks truly in parallel. The only shared state is `PROJECT_PATH/.git`, and git handles that safely (per-worktree index locks, atomic ref updates). No mutexes in Go.

If two webhooks arrive for the same issue ID while the first is still in flight, the second sees an existing worktree at `$WORKTREES_DIR/<issue-id>` and skips without touching state.

## Layout

```
cmd/main.go               # entry point: load config, read prompt files, start pool + HTTP server
pkg/config/config.go      # env → Config
pkg/server/                # HTTP server, Linear webhook verification, payload types
pkg/queue/                 # in-memory task queue
pkg/agents/                # worker pool, classifier, planner, coder, git/gh helpers
```

## Troubleshooting

- **`gh pr create` fails** — run `gh auth status` inside `PROJECT_PATH`. The remote must be a GitHub repo the authed user can push to.
- **`worktree add` fails with "invalid reference: origin/main"** — the base branch isn't on the remote. Either push it or set `BASE_BRANCH` to a ref that exists.
- **`unexpected easy response`** in logs — the classifier returned something other than `{ easy: true }`/`{ easy: false }`. Tighten the easy description or check the `agent` CLI output.
- **Stale webhook `401`s** — Linear retries deliveries. The server rejects bodies whose `webhookTimestamp` is older than a minute as replay protection.
