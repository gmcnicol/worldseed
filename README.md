# Worldseed

Local-first, terminal-native universe simulation.

## Phase 1 vertical slice

- `worldseed universe create <name>` creates a universe shard and schema.
- `worldseedd --universe <name>` starts deterministic simulation ticks and a local Unix socket API.
- `worldseed connect <name>` opens the observatory dashboard.
- Press `p` in the dashboard to issue `preserve_archive`; a delayed consequence appears later in the timeline.

Storage root defaults to `/var/lib/worldseed/universes` with one SQLite file per universe shard.
