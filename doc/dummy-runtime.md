## Dummy runtime image

This runtime is a native Go worker (keep-alive HTTP client, no artificial sleeps).

Build:

```bash
docker build -t openl4d-dummy-runtime ./docker/dummy-runtime
```

Run one worker:

```bash
docker run --rm --name runtime1 \
  -e PROCESS_BASE_URL=http://host.docker.internal:8081 \
  -e RESPONSE_QUERY_KEY=echo \
  openl4d-dummy-runtime
```

Simulate blocking work per invocation:

```bash
docker run --rm --name runtime1 \
  -e PROCESS_BASE_URL=http://host.docker.internal:8081 \
  -e BLOCKING_TIME_MS=200 \
  openl4d-dummy-runtime
```

Run multiple workers:

```bash
docker run --rm --name runtime1 -e LOG_PREFIX=runtime1 openl4d-dummy-runtime
docker run --rm --name runtime2 -e LOG_PREFIX=runtime2 openl4d-dummy-runtime
docker run --rm --name runtime3 -e LOG_PREFIX=runtime3 openl4d-dummy-runtime
```

Per-container progress logs:

```bash
docker run --rm \
  -e LOG_EVERY=500 \
  openl4d-dummy-runtime
```

The runtime logs `host=<container-hostname> processed=<n> ...`, which helps verify all replicas are serving requests.

Force runtime errors for matching paths:

```bash
docker run --rm \
  -e FORCE_ERROR_ON_PATH=/fail \
  openl4d-dummy-runtime
```

Echo a query parameter back in response body:

```bash
curl "http://localhost:8080/open-l4d?echo=abc123"
```

Response body includes the same value under `echo`.

## Stress ingest

Use the host script:

```bash
chmod +x ./scripts/stress.sh
./scripts/stress.sh
```

Useful overrides:

```bash
TOTAL_REQUESTS=5000 CONCURRENCY=200 ./scripts/stress.sh
INGEST_URL=http://localhost:8080 PATH_PREFIX=/api ./scripts/stress.sh
```
