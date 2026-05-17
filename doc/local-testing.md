# Running and load-testing locally

This page covers the end-to-end POC loop: build the manager, run a worker,
fire traffic, and observe what happens.

## Prerequisites

- Go 1.26+ (the version pinned in `go.mod`).
- Docker (for the dummy runtime image).
- [k6](https://k6.io/) (only needed for load testing).

## 1. Run the manager

From the repo root:

```bash
go run . serve
```

That starts two HTTP servers in the same process:

| Port | Purpose                                    |
| ---- | ------------------------------------------ |
| 8080 | Ingest — accepts external HTTP requests.    |
| 8081 | Processing — speaks the runtime protocol.   |

You should see:

```
Starting ingest server on :8080
Starting process server on :8081
```

Stop it with `Ctrl-C`. The shutdown sequence (close queue → drain → close
servers) is described in [request-lifecycle.md](./request-lifecycle.md).

## 2. Run one or more workers

The reference worker lives in `docker/dummy-runtime/`. Build it once:

```bash
docker build -t openl4d-dummy-runtime ./docker/dummy-runtime
```

Run a single worker:

```bash
docker run --rm --name runtime1 \
  -e PROCESS_BASE_URL=http://host.docker.internal:8081 \
  -e RESPONSE_QUERY_KEY=echo \
  openl4d-dummy-runtime
```

Or fan out a few replicas via Compose:

```bash
docker compose up --build --scale runtime=3
```

The full env-var reference for the dummy runtime
(`PROCESS_BASE_URL`, `BLOCKING_TIME_MS`, `FORCE_ERROR_ON_PATH`,
`RESPONSE_QUERY_KEY`, `LOG_EVERY`, `LOG_PREFIX`) is in
[dummy-runtime.md](./dummy-runtime.md).

## 3. Fire a request by hand

```bash
curl "http://localhost:8080/open-l4d?echo=abc123"
```

You should get back:

```json
{ "ok": true, "requestId": "…", "path": "/open-l4d", "echo": "abc123" }
```

If you set `FORCE_ERROR_ON_PATH=/fail` when starting the worker, any request
whose `rawPath` contains `/fail` is reported through `/runtime/invocation/{id}/error`,
and the client receives `502 Bad Gateway` with the error message as body.

## 4. Step through the runtime protocol with `process.http`

`process.http` (the IntelliJ HTTP Client file at the repo root) lets you
manually:

1. Enqueue a request via `GET /open-l4d` on `:8080`.
2. Pull the event via `GET /2018-06-01/runtime/invocation/next` on `:8081`.
3. POST a hand-crafted response or error to
   `/runtime/invocation/{requestId}/response` or `/error`.

It is the simplest way to confirm the contract documented in
[runtime-protocol.md](./runtime-protocol.md). The file is **gitignored**
because it is intended as scratch.

## 5. Load test with k6

A k6 script is provided in `scripts/k6-echo.js`. It generates per-request
echo strings, sends them as a query parameter, and asserts that the worker
echoed them back.

Default 30-second run with 50 virtual users:

```bash
k6 run scripts/k6-echo.js
```

Tunables (all environment variables):

```bash
BASE_URL=http://localhost:8080 \
TARGET_PATH=/open-l4d \
VUS=200 DURATION=1m TIMEOUT=10s \
  k6 run scripts/k6-echo.js
```

The script enforces:

- `http_req_failed` < 1%
- `checks` pass rate > 99%
- `echo_mismatch` count == 0 (a mismatch means a worker returned the wrong
  body — strong signal of a routing or response-channel bug)
- `invalid_json` count == 0

## Where to look when things go wrong

| Symptom                                                | Most likely cause                                                                 |
| ------------------------------------------------------ | --------------------------------------------------------------------------------- |
| `503 shutting down`                                    | Manager is closing — start a fresh process.                                       |
| `504 function timeout`                                 | No worker dequeued within 30s, or all workers are busy and the queue is full.     |
| `502 Bad Gateway`                                      | Worker reported via `/error`. Body contains the `errorMessage`.                   |
| `204 No Content` on `/next`                            | Queue closed and drained — manager is in shutdown.                                |
| `404 Not Found` on `/response`                         | Invocation already removed (timeout or deadline). Worker was too slow.            |
| `400 Bad Request` on `/response`                       | JSON had an unknown field — decoder uses `DisallowUnknownFields`.                 |
| k6 `echo_mismatch` > 0                                 | Response routed to the wrong waiter — open an issue, this should be impossible.   |

## What the POC does *not* do

- No persistence — restart the manager and in-flight invocations are lost.
- No auth — both ports are open to anyone who can reach them.
- No K8s deployment manifests yet; see [roadmap.md](./roadmap.md).
- No metrics endpoint — adding `/metrics` for Prometheus is on the roadmap.
