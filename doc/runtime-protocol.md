# Runtime protocol

`openl4d` deliberately mirrors the public
[AWS Lambda Runtime API](./runtime-api.yaml) so existing handler code and
"custom runtime" implementations can talk to it with minimal changes. This page
documents the openl4d-specific deviations and the exact JSON shapes used on the
wire.

## Endpoints

All worker endpoints live on the processing server (default `:8081`).

| Verb | Path                                                | Purpose                              |
| ---- | --------------------------------------------------- | ------------------------------------ |
| GET  | `/2018-06-01/runtime/invocation/next`               | Pull the next event (long-polled).    |
| POST | `/runtime/invocation/{request_id}/response`         | Deliver a successful response.        |
| POST | `/runtime/invocation/{request_id}/error`            | Deliver an invocation error.          |

> The "next" route keeps the `/2018-06-01` AWS prefix so off-the-shelf Lambda
> custom runtime clients (e.g. AWS RIC builds) work without rewriting URLs.
> The response and error routes currently sit at the root — that's a known
> gap relative to the AWS spec; see [roadmap.md](./roadmap.md).

## Event payload (`GET …/next` → 200)

The body is a JSON `HttpRequestEvent` modeled after the
[AWS API Gateway v2 / payload-format-version 2.0 event](https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-develop-integrations-lambda.html#http-api-develop-integrations-lambda.proxy-format),
trimmed to the fields that make sense outside of API Gateway:

```json
{
  "version": "2.0",
  "routeKey": "$default",
  "rawPath": "/open-l4d",
  "rawQueryString": "echo=abc123",
  "cookies": ["session=…"],
  "headers": {
    "accept": "*/*",
    "user-agent": "curl/8.4.0"
  },
  "queryStringParameters": { "echo": "abc123" },
  "requestContext": {
    "routeKey": "$default",
    "requestId": "0f3b…-…-9c",
    "domainName": "localhost:8080",
    "domainPrefix": "localhost:8080",
    "time": "17/May/2026:09:00:00 +0000",
    "timeEpoch": 1747468800000,
    "http": {
      "method": "GET",
      "path": "/open-l4d",
      "protocol": "HTTP/1.1",
      "sourceIp": "127.0.0.1",
      "userAgent": "curl/8.4.0"
    }
  },
  "body": "",
  "isBase64Encoded": false
}
```

Notable behaviours (see `internal/function/httpevent.go`):

- Headers are **lowercased**. Multi-valued headers are joined with `,`.
- `Cookie` headers are stripped and lifted into the top-level `cookies` array.
- The request body is read with a **6 MiB cap**. If the bytes are valid UTF-8,
  the body is shipped as a string with `isBase64Encoded: false`. Otherwise it
  is base64-encoded.
- `sourceIp` prefers `X-Forwarded-For`, then `X-Real-Ip`, then `RemoteAddr`.

`GET …/next` returns one of:

| Status              | Meaning                                                                |
| ------------------- | ---------------------------------------------------------------------- |
| `200 OK`            | Event JSON body. Worker must POST a response or error within the deadline. |
| `204 No Content`    | Queue is drained (manager shutting down). Worker can exit.             |
| `499 Client Gone`   | The polling client cancelled the request.                              |
| `405 Method Not Allowed` | Wrong HTTP method.                                                |
| `500 Internal Server Error` | JSON marshal failed or other unexpected error.                |

## Response payload (`POST …/response`)

Workers POST an `HttpResponseEvent` matching the standard Lambda Function URL /
API Gateway v2 response format:

```json
{
  "statusCode": 200,
  "headers": { "content-type": "application/json" },
  "multiValueHeaders": null,
  "body": "{\"ok\":true}",
  "isBase64Encoded": false,
  "cookies": ["session=abc; Path=/; HttpOnly"]
}
```

Rules:

- `statusCode` defaults to `200` when omitted or `0`.
- If `Content-Type` is not set in `headers`, the manager applies
  `application/json`.
- Each entry in `cookies` becomes a separate `Set-Cookie` response header — the
  manager does not parse or merge them.
- `body` may be base64-encoded for binary payloads; set `isBase64Encoded: true`.
- The decoder uses `DisallowUnknownFields`. Unknown keys → `400`.

| Status              | Meaning                                                  |
| ------------------- | -------------------------------------------------------- |
| `202 Accepted`      | Response was forwarded to the waiting client.             |
| `400 Bad Request`   | Malformed JSON or unknown field.                          |
| `404 Not Found`     | No invocation with that `request_id` (already finished or expired). |
| `410 Gone`          | Invocation context is already done (timed out / cancelled). |
| `499 Client Gone`   | The worker's own HTTP connection was cancelled mid-post.   |

## Error payload (`POST …/error`)

Workers POST an `InvocationErrorEvent`:

```json
{
  "errorMessage": "echo handler crashed",
  "errorType": "DummyRuntimeError",
  "stackTrace": [
    "main.handle(/handler.go:42)",
    "main.main(/main.go:18)"
  ]
}
```

The manager translates this into an `HttpResponseEvent` with
`statusCode: 502` and `body: <errorMessage>` and pushes it down the same
`ResponseChan`. The client sees `502 Bad Gateway`.

The response status codes are identical to the success path
(`202`/`400`/`404`/`410`/`499`).

## Differences from AWS Lambda

| Concern                         | AWS Lambda Runtime API              | openl4d                            |
| ------------------------------- | ----------------------------------- | ---------------------------------- |
| `/runtime/init/error`           | Reports a fatal init error           | Not implemented (workers don't initialize through the manager). |
| `Lambda-Runtime-*` headers      | Carry requestId, deadline, trace, ARN | Not emitted yet; data is inside the event JSON. |
| Response endpoint prefix        | `/2018-06-01/…`                     | Currently `/…` (no version prefix).  |
| Function ARNs / contexts        | Real ARNs, Cognito, client context  | Not modelled. The event is a single-tenant HTTP request. |
| Streaming responses             | `application/vnd.amazon.lambda.response+json` chunked transfer encoded | Not implemented. |

These deltas are intentional for the POC — they keep the surface area small
enough to study end-to-end. See [roadmap.md](./roadmap.md) for what changes
when the project moves past investigation.

## Reference

- `internal/function/httpevent.go` — event/response struct definitions and
  request → event mapping.
- `internal/server/processing/nextcontroller.go` — `next` handler.
- `internal/server/processing/sendresponsecontroller.go` — `response` handler.
- `internal/server/processing/invocationerrorcontroller.go` — `error` handler.
- [doc/runtime-api.yaml](./runtime-api.yaml) — original AWS spec for reference.
