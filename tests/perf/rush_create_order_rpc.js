import exec from 'k6/execution';
import grpc from 'k6/net/grpc';
import { Counter, Rate, Trend } from 'k6/metrics';

import { loadDataset } from './lib/dataset.js';
import { buildGrpcDataset } from './lib/grpc_dataset.js';
import { handlePerfSummary } from './lib/summary.js';

const DATASET_PATH = __ENV.DATASET_PATH || '../../tmp/perf/latest/users.json';
const TARGET = __ENV.ORDER_RPC_TARGET || '127.0.0.1:8082';
const PROTO_IMPORT_PATH = __ENV.PROTO_IMPORT_PATH || '../..';
const PROTO_FILE = __ENV.PROTO_FILE || 'services/order-rpc/order.proto';
const DATASET = buildGrpcDataset(loadDataset(DATASET_PATH));

export const options = {
  summaryTrendStats: ['avg', 'min', 'max', 'p(95)', 'p(99)'],
  scenarios: {
    rush_create_order_rpc: {
      executor: 'shared-iterations',
      vus: Number(__ENV.VUS || Math.min(DATASET.length, 5000)),
      iterations: Number(__ENV.ITERATIONS || DATASET.length),
      maxDuration: __ENV.MAX_DURATION || '15m',
    },
  },
  thresholds: {
    create_order_success_rate: ['rate>0'],
  },
};

const client = new grpc.Client();
client.load([PROTO_IMPORT_PATH], PROTO_FILE);

let connected = false;

const createOrderDuration = new Trend('create_order_duration');
const clientRequestStartEpochMs = new Trend('client_request_start_epoch_ms');
const clientResponseEndEpochMs = new Trend('client_response_end_epoch_ms');
const purchaseTokenVerifyDuration = new Trend('purchase_token_verify_duration');
const redisAdmitDuration = new Trend('redis_admit_duration');
const asyncDispatchScheduleDuration = new Trend('async_dispatch_schedule_duration');
const createOrderSuccessRate = new Rate('create_order_success_rate');
const createOrderSuccessCount = new Counter('create_order_success_count');
const inventoryInsufficientCount = new Counter('inventory_insufficient_count');
const businessFailureCount = new Counter('business_failure_count');

function ensureConnected() {
  if (connected) {
    return;
  }

  client.connect(TARGET, { plaintext: true });
  connected = true;
}

function classifyGrpcFailure(response) {
  const message = String(response && response.message ? response.message : '');
  if (message.includes('seat inventory insufficient')) {
    inventoryInsufficientCount.add(1);
    return;
  }

  businessFailureCount.add(1);
}

function addPerfStageMetrics(message) {
  purchaseTokenVerifyDuration.add(Number(message.purchaseTokenVerifyMs || 0));
  redisAdmitDuration.add(Number(message.redisAdmitMs || 0));
  asyncDispatchScheduleDuration.add(Number(message.asyncDispatchScheduleMs || 0));
}

export default function () {
  ensureConnected();

  const row = DATASET[exec.scenario.iterationInTest % DATASET.length];
  const startedAt = Date.now();
  clientRequestStartEpochMs.add(startedAt);
  const response = client.invoke('order.OrderRpc/PerfCreateOrder', {
    userId: row.userId,
    purchaseToken: row.purchaseToken,
  });
  const endedAt = Date.now();
  clientResponseEndEpochMs.add(endedAt);

  createOrderDuration.add(endedAt - startedAt);

  if (response.status !== grpc.StatusOK) {
    createOrderSuccessRate.add(false);
    classifyGrpcFailure(response);
    return;
  }

  if (!response.message || !response.message.orderNumber) {
    createOrderSuccessRate.add(false);
    businessFailureCount.add(1);
    return;
  }

  createOrderSuccessRate.add(true);
  createOrderSuccessCount.add(1);
  addPerfStageMetrics(response.message);
}

export function teardown() {
  if (!connected) {
    return;
  }

  client.close();
  connected = false;
}

export function handleSummary(data) {
  return handlePerfSummary(data, { datasetSize: DATASET.length });
}
