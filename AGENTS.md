# AGENTS.md - Local Overlay Tracking

This repository is a moving fork. Treat everything marked with `LOCAL FIX START` / `LOCAL FIX END` as an explicit local overlay that must be reviewed during every upstream sync.

## Source Of Local Overlays

- Base remote: `origin -> https://github.com/dmzz/goclaw`
- Imported patch source: `https://github.com/alsenis/goclaw/tree/upgrade/v3.9.1`
- Imported commits currently carried locally:
  - `f5563388` `fix(telegram): deliver existing files in current chat`
  - `08a834f7` `patch: consolidate telegram and tool-loop fixes`

## Recommended Branch Layout

For a fast-moving upstream, do not keep local patches only on the long-lived `dev` branch.

Recommended layout:

- `upstream` remote:
  - `https://github.com/nextlevelbuilder/goclaw`
- `base/vX.Y.Z`:
  - exact local branch pointing to the clean upstream release/tag
- `overlay/vX.Y.Z-dmzz`:
  - local patch branch created from `base/vX.Y.Z`
  - carries only dmzz-specific commits
- `origin/dev`:
  - optional integration branch in your fork
  - update it only from a validated `overlay/*` branch

Practical workflow:

1. Fetch upstream tags.
2. Create `base/vX.Y.Z` from the upstream tag.
3. Create `overlay/vX.Y.Z-dmzz` from that base branch.
4. Cherry-pick or rebase the local overlay commits onto the new overlay branch.
5. Run validation.
6. Only then fast-forward or merge into the branch you actually deploy from.

This keeps the diff against upstream small, makes rebases predictable, and lets you compare:

- `base/vX.Y.Z..overlay/vX.Y.Z-dmzz`
- `overlay/vX.Y.Z-dmzz..overlay/vX.Y.(Z+1)-dmzz`

## Local Overlay Areas

### 1. Telegram message delivery fixes

- Goal: allow sending already-existing files into the current Telegram chat and recover from invalid Telegram `target` placeholders like `current chat`.
- Inline markers live in:
  - `internal/tools/message.go`
  - `internal/tools/message_test.go`

### 2. Telegram multi-bot routing fixes

- Goal: improve multi-bot behavior in Telegram groups.
- Behavior covered:
  - prompt augmentation so exact `@username` handles survive handoff
  - Unicode-safe mention/entity parsing
  - `allow_bot_messages`
  - bot-to-bot loop guard in `yield` mode
  - group/topic config propagation for the above
- Inline markers live in:
  - `cmd/gateway_consumer_normal.go`
  - `cmd/gateway_consumer_prompt_helpers.go`
  - `cmd/gateway_consumer_prompt_helpers_test.go`
  - `internal/channels/telegram/channel.go`
  - `internal/channels/telegram/channel_parse_test.go`
  - `internal/channels/telegram/factory.go`
  - `internal/channels/telegram/handlers.go`
  - `internal/channels/telegram/handlers_utils.go`
  - `internal/channels/telegram/handlers_utils_test.go`
  - `internal/channels/telegram/mentions.go`
  - `internal/channels/telegram/mentions_test.go`
  - `internal/channels/telegram/topic_config.go`
  - `internal/channels/telegram/topic_config_test.go`
  - `internal/config/config_channels.go`
  - `ui/web/src/pages/channels/channel-detail/channel-advanced-dialog.tsx`
  - `ui/web/src/pages/channels/channel-detail/channel-general-tab.tsx`
  - `ui/web/src/pages/channels/channel-schemas.ts`
  - `ui/web/src/pages/channels/telegram-group-fields.tsx`
  - `ui/web/src/pages/config/sections/channels-section.tsx`

### 3. Tool-loop / Responses API fixes

- Goal: stabilize tool-loop behavior around compaction and OpenAI Responses API edge cases.
- Behavior covered:
  - preserve pending tool calls across compaction
  - retry empty-args `web_search` / `web_fetch` calls as truncation
  - clear retry batches out of `LastResponse` so malformed tool-calls do not leak into `ToolStage`
  - emit a user-facing truncation/tool-call error instead of falling through to final `...`
  - drop orphaned `function_call_output` items for Codex/Responses API
  - tolerate cumulative/overlapping streamed `function.arguments` chunks from OpenAI chat-completions routes
  - recover the last valid JSON object when OpenAI-compatible routes stream concatenated or partially repeated tool-call argument snapshots
  - strip persisted internal retry noise (`[System]` truncation hints, empty assistant stubs, fallback `...`) out of session history before it can poison later prompts
- Inline markers live in:
  - `internal/agent/loop_history_sanitize.go`
  - `internal/agent/loop_history_test.go`
  - `internal/pipeline/prune_stage.go`
  - `internal/pipeline/pipeline_test.go`
  - `internal/pipeline/stages_test.go`
  - `internal/pipeline/think_stage.go`
  - `internal/pipeline/think_stage_trunc_empty_args_test.go`
  - `internal/providers/codex_build.go`
  - `internal/providers/openai_chat.go`
  - `internal/providers/openai_http.go`
  - `internal/providers/openai_truncation_test.go`
  - `internal/providers/codex_test.go`

### 4. Team lead routing bias

- Goal: make lead agents actually use the team workflow when the user explicitly asks for team execution, and make the `search -> create` order clearer to weaker models.
- Behavior covered:
  - lead TEAM.md example now shows `team_tasks(search)` before `team_tasks(create)`
  - explicit user requests to use the team/teammates must go through `team_tasks`
  - research/report/summarize workflows are biased toward delegation instead of solo execution
- Inline markers live in:
  - `internal/agent/resolver_helpers.go`
  - `internal/agent/loop_utils_test.go`

### 5. Skill frontmatter normalization fixes

- Goal: let skill creation/publication recover when models send `SKILL.md` frontmatter with escaped newlines like `---\nname: ...` instead of real line breaks.
- Behavior covered:
  - normalize escaped `\n`, `\r\n`, and `\t` sequences before parsing frontmatter
  - reuse the normalized content in both `skill_manage` and `publish_skill`
  - persist normalized `SKILL.md` content when a copied skill directory arrived with escaped frontmatter
  - keep regression coverage for escaped-frontmatter inputs
- Inline markers live in:
  - `internal/skills/helpers.go`
  - `internal/skills/helpers_test.go`
  - `internal/tools/publish_skill.go`
  - `internal/tools/skill_manage.go`

### 6. Sandbox container recovery fixes

- Goal: let sandboxed file/exec tools survive Docker daemon or Docker Desktop restarts without getting stuck on stale container names.
- Behavior covered:
  - detect `docker run ... container name is already in use` for `goclaw-sbx-*`
  - inspect the conflicting container and verify it is our sandbox via label `goclaw.sandbox=true`
  - reuse the existing sandbox when it is still running
  - remove the stale sandbox and retry when it is `exited`/dead after daemon restart
  - keep regression coverage for inspect parsing and name-conflict detection
- Inline markers live in:
  - `internal/sandbox/docker.go`
  - `internal/sandbox/docker_test.go`

### 7. Telegram collapsible blockquote rendering

- Goal: let Telegram answers preserve model-emitted HTML blockquotes, including `<blockquote expandable>...</blockquote>`, instead of escaping them into plain text.
- Behavior covered:
  - preserve raw Telegram `<blockquote>` and `<blockquote expandable>` tags through the markdown-to-HTML pipeline
  - still render inline markdown inside the preserved blockquote body
  - avoid splitting a message chunk in the middle of a blockquote when the whole blockquote still fits in one Telegram message chunk
  - when a single blockquote exceeds one Telegram message, split it into multiple standalone blockquote chunks so each next message starts with a fresh `<blockquote ...>`
  - keep regression coverage for formatter pass-through and chunking safety
- Inline markers live in:
  - `internal/channels/telegram/format.go`
  - `internal/channels/telegram/format_extended_test.go`

### 8. Telegram reply target preservation

- Goal: make Telegram bot answers appear as replies to the inbound Telegram message that triggered the run, including streaming responses that are later edited in place.
- Behavior covered:
  - final outbound metadata carries `reply_to_message_id` for both the default `telegram` channel and DB-backed Telegram channel instance names
  - Telegram streaming receives per-run metadata so the first real `sendMessage` has `ReplyParameters` before later final edits happen
  - regression coverage for metadata propagation and stream reply target parsing
- Inline markers live in:
  - `cmd/gateway_consumer_normal.go`
  - `cmd/gateway_consumer_helpers.go`
  - `cmd/gateway_consumer_routing_test.go`
  - `internal/channels/channel.go`
  - `internal/channels/events.go`
  - `internal/channels/telegram/stream.go`
  - `internal/channels/telegram/stream_test.go`

## Companion Files Without Inline Markers

These belong to the same imported overlay set but cannot be cleanly tracked with inline code comments:

- `docs/05-channels-messaging.md`
- `ui/web/src/i18n/locales/en/channels.json`
- `ui/web/src/i18n/locales/en/config.json`
- `ui/web/src/i18n/locales/vi/channels.json`
- `ui/web/src/i18n/locales/vi/config.json`
- `ui/web/src/i18n/locales/zh/channels.json`
- `ui/web/src/i18n/locales/zh/config.json`

## Update Workflow

When syncing from upstream/community branches:

1. Update/fetch upstream refs as usual.
2. Rebase or merge.
3. Run `rg -n "LOCAL FIX START|LOCAL FIX END" .` and inspect every surviving block.
4. Compare each marked block with upstream code:
   - keep it if upstream still lacks the behavior
   - shrink or remove it if upstream gained the same fix
   - update related tests at the same time
5. Also re-check the companion files listed above, because they are not inline-marked.

## Strict Dmzz Release Pipeline

For local dmzz releases such as `v3.10.0-dmzz.2`, always release from the matching `overlay/vX.Y.Z-dmzz` branch and do not use `dev` as an intermediate integration branch.

The full image for dmzz releases must go through the prebuilt local pipeline below. Do not build the root `Dockerfile` with `ENABLE_EMBEDUI=true` on Docker Desktop / the remote daemon for release packaging, because that path always runs `pnpm build` inside Docker and is prone to hanging or failing during frontend/buildkit bootstrap.

Required order:

1. Validate the overlay checkout first:
   - `go test ./cmd/... ./internal/channels/telegram ./internal/config ./internal/pipeline ./internal/providers ./internal/tools/...`
   - `pnpm -C ui/web install --frozen-lockfile --force`
   - `pnpm -C ui/web build`
2. Build the local prebuilt full image from the overlay checkout:
   - `./scripts/build-dmzz-prebuilt-full.sh --version vX.Y.Z-dmzz.N --image goclaw:vX.Y.Z-dmzz.N-full --docker-host tcp://192.168.11.1:2375 --docker-api-version 1.47`
   - This script rebuilds `ui/web/dist`, syncs it into ignored local `internal/webui/dist`, builds linux `out/goclaw` with `-tags embedui`, builds `out/pkg-helper`, and only then runs `docker build -f Dockerfile.prebuilt-full`.
   - The `pnpm -C ui/web build` part can spend noticeable time inside `tsc -b`; do not treat a busy `tsc` process as a script hang.
   - If a retry is needed after local artifacts are already built, reuse them instead of rerunning the expensive local steps:
     `./scripts/build-dmzz-prebuilt-full.sh --version vX.Y.Z-dmzz.N --image goclaw:vX.Y.Z-dmzz.N-full --docker-host tcp://192.168.11.1:2375 --docker-api-version 1.47 --skip-install --skip-ui-build --skip-go-build`
   - `Dockerfile.prebuilt-full` must stay on top of `ghcr.io/nextlevelbuilder/goclaw:vX.Y.Z-full` or another explicitly reachable runtime base image. Do not introduce a `docker.io`-only base into this strict release path.
   - If the daemon cannot pull `ghcr.io` on this host, retry the same command with `--runtime-base-image` pointing to a cached local full image already present on the remote daemon, for example `goclaw:vX.Y.Z-dmzz.(N-1)-full`.
   - The strict prebuilt path must default to `DOCKER_BUILDKIT=0`, because BuildKit metadata resolution against `docker.io` / `ghcr.io` is known to fail on this Docker Desktop + SOCKS proxy setup even when the legacy builder works.
3. Build the thin wrapper image from `/home/dmzz/project` against the freshly built base image:
   - `DOCKER_API_VERSION=1.47 docker -H tcp://192.168.11.1:2375 build -t goclaw:full-sandbox --build-arg GOCLAW_BASE_IMAGE=goclaw:vX.Y.Z-dmzz.N-full -f /home/dmzz/project/Dockerfile.remote-full-sandbox /home/dmzz/project`
4. Restart the deployed stack through the full `goclaw` compose project so existing named volumes and port bindings are preserved:
   - `DOCKER_API_VERSION=1.47 docker -H tcp://192.168.11.1:2375 compose --env-file goclaw/.env --project-directory /home/dmzz/project -p goclaw -f goclaw/docker-compose.yml -f goclaw/docker-compose.postgres.yml -f goclaw/docker-compose.sandbox.yml -f docker-compose.postgres.remote-pgvector.yml -f docker-compose.remote-full-sandbox.yml up -d --no-build --force-recreate goclaw`
   - This must recreate `goclaw-goclaw-1` inside the existing `goclaw` project so it keeps the old named volumes such as `goclaw_goclaw-data`, `goclaw_goclaw-workspace`, and `goclaw_goclaw-skills`.
   - `docker-compose.remote-full-sandbox.yml` must stay only as the last override file in that full stack; do not deploy it alone with an implicit project name from the current directory, because that creates a separate `project-goclaw-1` container without the existing persistent volumes and without the published external port.
   - `docker-compose.remote-full-sandbox.yml` must keep `env_file: ./goclaw/.env` so runtime variables from `/home/dmzz/project/goclaw/.env` are injected into the container, while the CLI `--env-file goclaw/.env` continues to drive compose interpolation and build args.
   - If the compose state is in doubt, inspect the resolved stack before restart with the same flags plus `config`; the resolved project name must be `goclaw`.
   - The remote compose stack does not synthesize `GOCLAW_POSTGRES_DSN`; `/home/dmzz/project/goclaw/.env` itself must already contain a real DSN before restart.
   - For the current remote host-gateway scheme, the DSN shape is `postgres://USER:PASSWORD@asus-note:5432/DB?sslmode=disable`.
5. Generated release artifacts under `out/` and `internal/webui/dist/` are local-only and must not be committed.

### Sandbox Containers During Core Updates

- `goclaw-sbx-*` containers are ephemeral sandbox runtimes created by GoClaw from `goclaw-sandbox:bookworm-slim`; they are not long-lived deploy services.
- When updating `goclaw` core or restarting Docker Desktop / the remote daemon, do not preserve old `goclaw-sbx-*` containers as state. They may survive as `Exited` containers and then block new tool runs by name conflict.
- Before or immediately after restarting the main `goclaw` service, prune stale exited sandbox containers:
  - `DOCKER_API_VERSION=1.47 sh -c 'docker -H tcp://192.168.11.1:2375 ps -aq --filter label=goclaw.sandbox=true --filter status=exited | xargs -r docker -H tcp://192.168.11.1:2375 rm -f'`
- Verify sandbox tail state with:
  - `DOCKER_API_VERSION=1.47 docker -H tcp://192.168.11.1:2375 ps -a --filter label=goclaw.sandbox=true`
- Rebuild `goclaw-sandbox:bookworm-slim` only when `Dockerfile.sandbox.remote` or the sandbox toolchain changed. A normal GoClaw core update does not require rebuilding every sandbox container.
- If file tools (`read_file`, `write_file`, `list_files`, `exec`) suddenly start failing with `container name is already in use`, treat that first as a stale sandbox cleanup problem, not as an agent logic regression.

## Validation After Any Upstream Sync

- `go test ./cmd/... ./internal/channels/telegram ./internal/config ./internal/pipeline ./internal/providers ./internal/tools/...`
- `pnpm -C ui/web install --frozen-lockfile --force`
- `pnpm -C ui/web build`

## Local Toolchain Assumptions In This Workspace

- `go 1.26.0`
- `node 22.22.0`
- `pnpm 10.30.1`
