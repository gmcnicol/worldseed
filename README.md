# Worldseed

Local-first, terminal-native universe simulation.

## Phase 1 bootstrap

- `worldseed init <universe-name>` creates a universe database and applies migrations.
- `worldseedd --universe <name>` starts a daemon that appends timeline tick events.
- `worldseed connect <universe-name>` opens a minimal Bubble Tea dashboard.

Default universe storage root is `/var/lib/worldseed/universes`.
