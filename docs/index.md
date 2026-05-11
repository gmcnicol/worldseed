# Worldseed Development Log

## Phase 1 Vertical Slice

This repository currently implements a local archive daemon and SSH dashboard for one autonomous universe shard.

Run locally with a writable data directory:

```sh
worldseed --data-dir ./testdata universe create aurora
worldseedd --data-dir ./testdata start --universe <id-from-create>
worldseed connect
```

The dashboard exposes universe age, entropy, archive integrity, active civilisations, recent checksummed timeline events, and the `preserve_archive` intervention.
