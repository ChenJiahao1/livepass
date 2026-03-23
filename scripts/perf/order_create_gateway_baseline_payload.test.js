import test from 'node:test';
import assert from 'node:assert/strict';

import {
  buildConstantArrivalRateOptions,
  buildOrderCreatePayload,
  parseTicketUserIdLiterals,
  parseUserPool,
  resolveSteadyStartTime,
  selectUserPoolEntry,
} from './order_create_gateway_baseline_payload.js';

test('parseTicketUserIdLiterals preserves 18-digit ticket user ids', () => {
  assert.deepEqual(parseTicketUserIdLiterals('116273384188280835,116273384188280836'), [
    '116273384188280835',
    '116273384188280836',
  ]);
});

test('buildOrderCreatePayload emits exact ticket user id digits', () => {
  const payload = buildOrderCreatePayload({
    programId: 10001,
    ticketCategoryId: 40001,
    ticketUserIdLiterals: ['116273384188280835'],
    distributionMode: 'express',
    takeTicketMode: 'paper',
  });

  assert.equal(
    payload,
    '{"programId":10001,"ticketCategoryId":40001,"ticketUserIds":[116273384188280835],"distributionMode":"express","takeTicketMode":"paper"}',
  );
  assert.ok(payload.includes('[116273384188280835]'));
  assert.ok(!payload.includes('116273384188280830'));
});

test('resolveSteadyStartTime adds warmup duration, sleep, and guard window by default', () => {
  assert.equal(
    resolveSteadyStartTime({
      warmupDuration: '1s',
      iterationSleepSeconds: 2,
      explicitSteadyStartTime: '',
    }),
    '4s',
  );
});

test('resolveSteadyStartTime prefers explicit override', () => {
  assert.equal(
    resolveSteadyStartTime({
      warmupDuration: '1s',
      iterationSleepSeconds: 2,
      explicitSteadyStartTime: '90s',
    }),
    '90s',
  );
});

test('parseUserPool keeps jwt and exact ticket user id digits', () => {
  assert.deepEqual(
    parseUserPool(
      JSON.stringify([
        { jwt: 'token-a', ticketUserId: '116273384188280835' },
        { jwt: 'token-b', ticketUserId: 116273384188280836n.toString() },
      ]),
    ),
    [
      { jwt: 'token-a', ticketUserId: '116273384188280835' },
      { jwt: 'token-b', ticketUserId: '116273384188280836' },
    ],
  );
});

test('selectUserPoolEntry rotates by absolute iteration index', () => {
  const userPool = [
    { jwt: 'token-a', ticketUserId: '1001' },
    { jwt: 'token-b', ticketUserId: '1002' },
    { jwt: 'token-c', ticketUserId: '1003' },
  ];

  assert.deepEqual(selectUserPoolEntry(userPool, 0), userPool[0]);
  assert.deepEqual(selectUserPoolEntry(userPool, 1), userPool[1]);
  assert.deepEqual(selectUserPoolEntry(userPool, 4), userPool[1]);
});

test('buildConstantArrivalRateOptions emits a constant-arrival-rate scenario', () => {
  assert.deepEqual(
    buildConstantArrivalRateOptions({
      targetQps: 80,
      duration: '5s',
      preAllocatedVUs: 80,
      maxVUs: 160,
    }),
    {
      thresholds: {
        http_req_failed: ['rate<0.01'],
        http_req_duration: ['p(99)<10000'],
      },
      scenarios: {
        steady_state: {
          executor: 'constant-arrival-rate',
          exec: 'createOrder',
          rate: 80,
          timeUnit: '1s',
          duration: '5s',
          preAllocatedVUs: 80,
          maxVUs: 160,
        },
      },
    },
  );
});
