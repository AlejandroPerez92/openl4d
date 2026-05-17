# Request lifecycle

This page walks one request from the moment a client calls the ingest port to
the moment they receive a response (or an error). For the data structures
referenced here, see [state-and-data-structures.md](./state-and-data-structures.md).

## Happy path

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant I as Ingest server :8080
    participant Q as PendingQueue
    participant M as ProcessingMap
    participant P as Processing server :8081
    participant W as Worker (runtime)

    C->>I: HTTP request<br/>(GET /open-l4d?echo=x)
    I->>I: Build HttpRequestEvent<br/>+ Invocation{Ctx, Deadline=now+30s,<br/>ResponseChan(cap=1)}
    I->>Q: Enqueue(invocation)
    Note over I: Ingest goroutine blocks<br/>on invocation.ResponseChan
    W->>P: GET /2018-06-01/runtime/invocation/next
    P->>Q: Dequeue(ctx)
    Q-->>P: invocation
    P->>M: Put(invocation) keyed by requestId
    P-->>W: 200 + event JSON
    W->>W: Execute handler
    W->>P: POST /runtime/invocation/{requestId}/response<br/>(HttpResponseEvent JSON)
    P->>M: Get(requestId)
    M-->>P: invocation
    P->>I: invocation.ResponseChan <- response
    P->>M: Delete(requestId)
    P-->>W: 202 Accepted
    I-->>C: HTTP response<br/>(status, headers, body, cookies)
```

## Function-error path

If the handler fails, the worker POSTs to `…/error` instead of `…/response`.
The processing server turns the error into an `HttpResponseEvent` with status
`502 Bad Gateway` and pushes it down the same `ResponseChan`:

```mermaid
sequenceDiagram
    autonumber
    participant W as Worker
    participant P as Processing server
    participant M as ProcessingMap
    participant I as Ingest server
    participant C as Client

    W->>P: POST /runtime/invocation/{requestId}/error<br/>(InvocationErrorEvent JSON)
    P->>M: Get(requestId)
    P->>P: Build HttpResponseEvent{StatusCode: 502, Body: ErrorMessage}
    P->>I: invocation.ResponseChan <- 502 response
    P->>M: Delete(requestId)
    P-->>W: 202 Accepted
    I-->>C: HTTP 502 + error message
```

## Timeout path (worker is too slow or never picks up the event)

Every invocation carries a 30-second deadline. If `Ctx.Done()` fires before the
worker sends a response, the ingest goroutine wakes up first and returns an
error to the client. The invocation is marked cancelled so any worker that
later dequeues it will skip it.

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant I as Ingest server
    participant Q as PendingQueue
    participant M as ProcessingMap
    participant W as Worker

    C->>I: HTTP request
    I->>Q: Enqueue(invocation)
    Note over I: Wait on ResponseChan
    Note over I,Q: ...30s pass without a response...
    Note over I: ctx.Done() fires<br/>(DeadlineExceeded)
    I->>M: Delete(requestId)
    I->>I: invocation.Cancel()  (atomic.Bool = true)
    I-->>C: 504 Gateway Timeout
    W->>Q: (later) Dequeue
    Q-->>W: invocation
    W->>W: IsCancelled() == true → skip, loop again
```

The same flow runs if the client disconnects early — `ctx.Err()` is then
`context.Canceled`, and the response to the (already-gone) client is `499`.

## Late or duplicate responses

A worker may POST a response after the deadline has already triggered ingest
cleanup. In that case the request is no longer in `ProcessingMap`:

| Situation                                  | Response code from `/response` or `/error` |
| ------------------------------------------ | ------------------------------------------ |
| Request ID not found in map                | `404 Not Found`                            |
| Found but `Ctx` already done / cancelled   | `410 Gone`                                 |
| Found and successfully forwarded           | `202 Accepted`                             |
| Worker POSTs while client disconnected     | `499 Client Gone`                          |

Both `/response` and `/error` decode JSON with `DisallowUnknownFields`, so
unexpected payload keys are rejected with `400 Bad Request`.

## Shutdown / drain path

On `SIGINT` / `SIGTERM` the manager:

1. Closes the `PendingQueue` so no new requests are accepted (subsequent
   `Enqueue` returns `ErrQueueClosed` → ingest replies `503`).
2. Shuts down the ingest server with a 5-second grace period.
3. Drains in-flight work for up to 30 seconds: waits until both the queue and
   the processing map are empty (or the deadline hits).
4. Shuts down the processing server with a 5-second grace period.

A worker that calls `next` while the queue is drained gets a `204 No Content`
response (`ErrQueueDrained`).

See `cmd/serve.go` for the actual orchestration.
