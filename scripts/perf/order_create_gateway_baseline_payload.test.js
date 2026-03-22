import test from 'node:test';
import assert from 'node:assert/strict';

import {
  buildOrderCreatePayload,
  parseTicketUserIdLiterals,
  resolveSteadyStartTime,
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
