import http from 'k6/http';
import exec from 'k6/execution';
import { Counter, Rate, Trend } from 'k6/metrics';
import { sleep } from 'k6';

import { loadDataset } from './lib/dataset.js';
import { handlePerfSummary } from './lib/summary.js';

const DATASET_PATH = __ENV.DATASET_PATH || 'tmp/perf/latest/users.json';
const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8081';
const PERF_HEADER_NAME = __ENV.PERF_HEADER_NAME || 'X-LivePass-Perf-Secret';
const PERF_USER_ID_HEADER = __ENV.PERF_USER_ID_HEADER || 'X-LivePass-Perf-User-Id';
const PERF_SECRET = __ENV.PERF_SECRET || 'livepass-perf-secret-0001';
const POLL_INTERVAL_MS = Number(__ENV.POLL_INTERVAL_MS || 200);
const POLL_TIMEOUT_MS = Number(__ENV.POLL_TIMEOUT_MS || 10000);

const DATASET = loadDataset(DATASET_PATH);

export const options = {
  summaryTrendStats: ['avg', 'p(95)', 'p(99)'],
  scenarios: {
    rush_create_order: {
      executor: 'shared-iterations',
      vus: Number(__ENV.VUS || Math.min(DATASET.length, 5000)),
      iterations: Number(__ENV.ITERATIONS || DATASET.length),
      maxDuration: __ENV.MAX_DURATION || '15m',
    },
  },
  thresholds: {
    create_order_success_rate: ['rate>0'],
    poll_success_rate: ['rate>0'],
  },
};

const createOrderDuration = new Trend('create_order_duration');
const createOrderSuccessRate = new Rate('create_order_success_rate');
const pollSuccessRate = new Rate('poll_success_rate');
const createOrderSuccessCount = new Counter('create_order_success_count');
const pollSuccessCount = new Counter('poll_success_count');
const inventoryInsufficientCount = new Counter('inventory_insufficient_count');
const businessFailureCount = new Counter('business_failure_count');

function headersForUser(userId) {
  return {
    'Content-Type': 'application/json',
    [PERF_HEADER_NAME]: PERF_SECRET,
    [PERF_USER_ID_HEADER]: String(userId),
  };
}

function classifyFailure(responseBody) {
  const text = String(responseBody || '');
  if (text.includes('seat inventory insufficient')) {
    inventoryInsufficientCount.add(1);
    return 'inventory_insufficient';
  }

  businessFailureCount.add(1);
  return 'business_failure';
}

function pollUntilDone(orderNumber, userId, showTimeId) {
  const startedAt = Date.now();
  while (Date.now() - startedAt <= POLL_TIMEOUT_MS) {
    const resp = http.post(
      `${BASE_URL}/order/poll`,
      JSON.stringify({ orderNumber, showTimeId }),
      { headers: headersForUser(userId), tags: { endpoint: 'order_poll' } },
    );

    if (resp.status >= 200 && resp.status < 300) {
      const body = resp.json();
      if (body.done === true) {
        pollSuccessRate.add(true);
        pollSuccessCount.add(1);
        return body;
      }
    }
    sleep(POLL_INTERVAL_MS / 1000);
  }

  pollSuccessRate.add(false);
  return null;
}

export default function () {
  const row = DATASET[exec.scenario.iterationInTest % DATASET.length];
  const payload = JSON.stringify({ purchaseToken: row.purchaseToken });
  const response = http.post(
    `${BASE_URL}/order/create`,
    payload,
    { headers: headersForUser(row.userId), tags: { endpoint: 'order_create' } },
  );

  createOrderDuration.add(response.timings.duration);

  if (response.status < 200 || response.status >= 300) {
    createOrderSuccessRate.add(false);
    classifyFailure(response.body);
    return;
  }

  const body = response.json();
  if (!body.orderNumber) {
    createOrderSuccessRate.add(false);
    businessFailureCount.add(1);
    return;
  }

  createOrderSuccessRate.add(true);
  createOrderSuccessCount.add(1);
  pollUntilDone(body.orderNumber, row.userId, row.showTimeId);
}

export function handleSummary(data) {
  return handlePerfSummary(data, { datasetSize: DATASET.length });
}
