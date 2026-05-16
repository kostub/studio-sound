# ADR: IPC Contract — NDJSON over stdin/stdout

**Status:** Accepted  
**Date:** 2026-05-14  
**Deciders:** Studio Sound engineering team

---

## Context

The Studio Sound desktop application consists of a Tauri/React frontend and a
Go sidecar process. The two processes must exchange typed, versioned messages at
runtime. We need a well-defined IPC contract that:

- carries request/response pairs with identifiable correlation IDs,
- supports typed payloads validated against a single source of truth,
- keeps the type definitions in sync across three languages (TypeScript, Go,
  Rust) without manual duplication,
- requires no listening network sockets (no firewall prompts, no port
  conflicts), and
- works identically on macOS and Windows.

Several transport options were considered:

| Option | Description |
|---|---|
| **A — NDJSON over stdin/stdout** | Newline-delimited JSON envelopes exchanged over the sidecar's stdin/stdout pipes. Tauri's `tauri-plugin-shell` manages the child process and its stdio handles. |
| **B — HTTP/WebSocket on localhost** | Sidecar binds a random port; Tauri queries it via `fetch` or a WebSocket. |
| **C — Named pipe / Unix domain socket** | Platform-specific IPC socket; sidecar and Tauri negotiate a path. |

---

## Decision

We adopt **Option A: NDJSON over stdin/stdout**, with JSON Schema (Draft
2020-12) as the single source of truth for all payload and result shapes, and
generated types committed to the repository for TypeScript, Go, and Rust.

**Wire format.** Every message is a JSON object on a single line followed by
`\n` (NDJSON). The envelope carries:

```json
{
  "v": 1,
  "id": "<correlation-id>",
  "kind": "request" | "response" | "event",
  "method": "<namespace.name>",
  "payload": { ... } | null,
  "result":  { ... } | null,
  "error":   { "code": "...", "message": "...", "details": { ... } }
}
```

**Source of truth.** `schemas/*.schema.json` defines every payload and result
shape. A single `npm run gen:schemas` command regenerates the committed types
for all three languages; CI fails if the tree is dirty after running it.

**Sidecar CLI surface.** The sidecar binary exposes two subcommands:
- `studio-sidecar health` — retained for Phase 0 smoke tests; unchanged.
- `studio-sidecar serve` — long-lived IPC loop reading from stdin and writing
  to stdout. Spawned by the Rust supervisor with `args: ["serve"]`.

---

## Consequences

**Positive:**
- Zero network dependencies: no listening sockets, no firewall prompts, no
  port allocation, no TLS plumbing.
- Lifetime is coupled to the parent process by design: when the Tauri app
  exits the sidecar's stdin is closed, causing `serve` to exit cleanly.
- Schema drift is caught in CI: the dirty-tree check after codegen means a
  developer cannot silently diverge Go, Rust, and TS types.
- Typed API surface on all three sides: generated structs prevent misspelled
  field names and wrong types from reaching production.
- Simple to debug locally: `echo '{"v":1,...}' | ./studio-sidecar serve`.

**Negative / trade-offs:**
- Ad-hoc debugging with `curl` is not possible (no HTTP endpoint).
- Protocol upgrades require careful versioning: the `v` field signals the
  protocol version; mismatches produce `PROTOCOL_VERSION_MISMATCH` errors.
- Concurrent request routing is done by the Rust multiplexer (correlation IDs
  in the Rust pending map); the sidecar itself has no cap on in-flight requests
  beyond the Rust 64-request ceiling.
- Option B (HTTP) would have been easier to inspect with browser devtools but
  would introduce a listening socket and require port negotiation.

---

## Alternatives Rejected

**Option B — HTTP/WebSocket on localhost.** Easier ad-hoc inspection but
introduces a listening socket (firewall prompts on Windows, port conflicts),
requires `net/http` server lifecycle management in Go, and adds no meaningful
functionality beyond what stdin/stdout provides for a single-client scenario.

**Option C — Named pipe / Unix domain socket.** Eliminates the network concern
but requires platform-specific path negotiation, more complex lifecycle
management, and additional Tauri permission surface. The gain over stdin/stdout
is marginal for our single-client, request/response workload.

---

## References

- Wire format spec: [docs/ipc-contract.md](../ipc-contract.md)
- Requirements: [docs/reqs/phase1-ipc-contract.md](../reqs/phase1-ipc-contract.md)
- Low-Level Design: [docs/lld/2026-05-14-phase-1-ipc-contract.md](../lld/2026-05-14-phase-1-ipc-contract.md)
