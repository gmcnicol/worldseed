# Agent Notes

Worldseed Phase 1 should stay local-first and terminal-native.

Architectural decisions made in this slice:

- The daemon interface is SSH, not HTTP or a Unix socket. `worldseedd` serves a Bubble Tea dashboard over a localhost SSH PTY session.
- The simulation is deterministic by deriving tick RNG from persisted universe seed and universe age. Avoid hidden process-local RNG state.
- SQLite projections are updated from the simulation path, while timeline events remain append-only.
- Federation remains a placeholder package only. Do not add network federation behavior in Phase 1.
- The default archive path is `/var/lib/worldseed`, but commands accept `--data-dir` and respect `WORLDSEED_HOME` for development.
