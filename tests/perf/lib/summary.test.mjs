import test from 'node:test';
import assert from 'node:assert/strict';

import { handlePerfSummary } from './summary.js';

test('handlePerfSummary 在无 poll 指标时仍输出 create 聚合', () => {
  const result = handlePerfSummary({
    metrics: {
      create_order_duration: {
        values: {
          count: 2,
          avg: 12.5,
          'p(95)': 20,
          'p(99)': 25,
        },
      },
      create_order_success_count: {
        values: { count: 2 },
      },
      business_failure_count: {
        values: { count: 1 },
      },
      create_order_success_rate: {
        values: { rate: 1 },
      },
      purchase_token_verify_duration: {
        values: {
          avg: 1,
          'p(95)': 2,
          'p(99)': 3,
        },
      },
      redis_admit_duration: {
        values: {
          avg: 4,
          'p(95)': 5,
          'p(99)': 6,
        },
      },
      async_dispatch_schedule_duration: {
        values: {
          avg: 7,
          'p(95)': 8,
          'p(99)': 9,
        },
      },
    },
  }, { datasetSize: 2 });

  const summary = JSON.parse(result['summary.json']);
  assert.deepEqual(summary, {
    datasetSize: 2,
    successRate: 1,
    pollSuccessRate: 0,
    createTotal: 2,
    createSuccessCount: 2,
    pollSuccessCount: 0,
    inventoryInsufficientCount: 0,
    businessFailureCount: 1,
    p95: 20,
    p99: 25,
    avg: 12.5,
    purchaseTokenVerifyAvg: 1,
    purchaseTokenVerifyP95: 2,
    purchaseTokenVerifyP99: 3,
    redisAdmitAvg: 4,
    redisAdmitP95: 5,
    redisAdmitP99: 6,
    asyncDispatchScheduleAvg: 7,
    asyncDispatchScheduleP95: 8,
    asyncDispatchScheduleP99: 9,
  });
});
