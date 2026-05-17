import http from 'k6/http';
import { check } from 'k6';
import exec from 'k6/execution';
import { Counter } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TARGET_PATH = __ENV.TARGET_PATH || '/open-l4d';

const VUS = Number(__ENV.VUS || 50);
const DURATION = __ENV.DURATION || '30s';

const echoMismatch = new Counter('echo_mismatch');
const invalidJSON = new Counter('invalid_json');

export const options = {
  scenarios: {
    echo_load: {
      executor: 'constant-vus',
      vus: VUS,
      duration: DURATION,
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    checks: ['rate>0.99'],
    echo_mismatch: ['count==0'],
    invalid_json: ['count==0'],
  },
};

export default function () {
  const echo = `vu${exec.vu.idInTest}-it${exec.vu.iterationInScenario}-${Date.now()}`;
  const url = `${BASE_URL}${TARGET_PATH}?echo=${encodeURIComponent(echo)}`;

  const res = http.get(url, {
    timeout: __ENV.TIMEOUT || '10s',
    tags: { endpoint: 'ingest' },
  });

  const isJSON = (res.headers['Content-Type'] || '').includes('application/json');

  let body;
  if (isJSON) {
    try {
      body = JSON.parse(res.body);
    } catch (_) {
      invalidJSON.add(1);
    }
  } else {
    invalidJSON.add(1);
  }

  const okStatus = res.status === 200;
  const hasEcho = body && body.echo === echo;
  const hasRequestID = body && typeof body.requestId === 'string' && body.requestId.length > 0;

  if (!hasEcho) {
    echoMismatch.add(1);
    console.error(
      `[echo-mismatch] expected=${echo} received=${body ? body.echo : '<missing>'} requestId=${body ? body.requestId : '<missing>'} status=${res.status} rawBody=${res.body}`
    );
  }

  check(res, {
    'status is 200': () => okStatus,
    'response is valid json': () => !!body,
    'echo matches request': () => hasEcho,
    'has requestId': () => hasRequestID,
  });
}
