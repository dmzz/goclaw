# AGENTS.md - Local Overlay Tracking

This repository is a moving fork. Treat everything marked with `LOCAL FIX START` / `LOCAL FIX END` as an explicit local overlay that must be reviewed during every upstream sync.

## Source Of Local Overlays

- Base remote: `origin -> https://github.com/dmzz/goclaw`
- Imported patch source: `https://github.com/alsenis/goclaw/tree/upgrade/v3.9.1`
- Imported commits currently carried locally:
  - `f5563388` `fix(telegram): deliver existing files in current chat`
  - `08a834f7` `patch: consolidate telegram and tool-loop fixes`

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
  - drop orphaned `function_call_output` items for Codex/Responses API
- Inline markers live in:
  - `internal/pipeline/prune_stage.go`
  - `internal/pipeline/stages_test.go`
  - `internal/pipeline/think_stage.go`
  - `internal/pipeline/think_stage_trunc_empty_args_test.go`
  - `internal/providers/codex_build.go`
  - `internal/providers/codex_test.go`

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

## Validation After Any Upstream Sync

- `go test ./cmd/... ./internal/channels/telegram ./internal/config ./internal/pipeline ./internal/providers ./internal/tools/...`
- `pnpm -C ui/web install --frozen-lockfile --force`
- `pnpm -C ui/web build`

## Local Toolchain Assumptions In This Workspace

- `go 1.26.0`
- `node 22.22.0`
- `pnpm 10.30.1`
