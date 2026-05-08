# Worldseed

Worldseed is a local-first, terminal-native universe simulation. Each universe is an autonomous SQLite-backed shard maintained by a local daemon.

Phase 1 provides:

- `worldseed universe create <name>` for deterministic universe creation.
- `worldseedd start --universe <name>` for continuous local simulation.
- `worldseed connect` for an SSH-backed terminal dashboard.
- An append-only timeline event log with valid universe time and recorded archive time.
- One intervention path: press `p` in the dashboard to request `preserve_archive`; a delayed consequence is emitted after later ticks.

## Development

The default data directory follows the Phase 1 storage target:

```sh
/var/lib/worldseed/universes/<name>/universe.sqlite
```

For local development without root-owned paths, set `WORLDSEED_HOME` or pass `--data-dir`:

```sh
worldseed --data-dir ./testdata universe create aurora
worldseedd --data-dir ./testdata start --universe aurora
worldseed connect
```

The daemon listens on `127.0.0.1:27411` by default and generates a stable Ed25519 SSH host key inside the universe directory.

## Architecture

The Phase 1 boundary is intentionally small:

- `internal/storage` owns SQLite access and migrations.
- `internal/sim` owns deterministic rule evaluation.
- `internal/daemon` owns the local SSH daemon and simulation loop.
- `internal/tui` owns the Bubble Tea dashboard.
- `internal/universe` owns shard paths and creation.

Federation is explicitly dormant in this phase.
