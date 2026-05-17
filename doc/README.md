# Documentation

This folder is the long-form reference for `openl4d`. The top-level
[`README.md`](../README.md) is the quick start; everything here goes deeper.

| Page | What it covers |
| ---- | -------------- |
| [architecture.md](./architecture.md) | Top-down diagram of the manager, the two HTTP servers, the queue/map, and the worker fleet. |
| [request-lifecycle.md](./request-lifecycle.md) | Sequence diagrams: happy path, error path, timeout, drain/shutdown. |
| [runtime-protocol.md](./runtime-protocol.md) | Wire-level HTTP contract between manager and workers, with payload examples and status codes. |
| [state-and-data-structures.md](./state-and-data-structures.md) | `Invocation`, `PendingQueue`, `ProcessingMap` — semantics, concurrency, tuning knobs. |
| [local-testing.md](./local-testing.md) | Running the manager + workers + k6 locally. |
| [dummy-runtime.md](./dummy-runtime.md) | Detailed env-var reference for the reference worker image. |
| [runtime-api.yaml](./runtime-api.yaml) | The original AWS Lambda Runtime API OpenAPI spec for comparison. |
| [roadmap.md](./roadmap.md) | What is intentionally missing, and what would change to take this past POC. |
