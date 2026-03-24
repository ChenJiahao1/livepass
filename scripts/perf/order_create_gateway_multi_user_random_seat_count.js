import http from 'k6/http';
import { check } from 'k6';
import exec from 'k6/execution';

import {
  buildOrderCreatePayload,
  parseUserPool,
  pickTicketUserIds,
} from './order_create_gateway_baseline_payload.js';

function parseEnvFile(content) {
  return content
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith('#'))
    .reduce((acc, line) => {
      const index = line.indexOf('=');
      if (index === -1) {
        return acc;
      }

      const key = line.slice(0, index).trim();
      let value = line.slice(index + 1).trim();
      if (
        (value.startsWith('"') && value.endsWith('"')) ||
        (value.startsWith("'") && value.endsWith("'"))
      ) {
        value = value.slice(1, -1);
      }
      acc[key] = value;
      return acc;
    }, {});
}

const envFileValues = __ENV.ENV_FILE ? parseEnvFile(open(__ENV.ENV_FILE)) : {};

function env(name, fallback = '') {
  if (__ENV[name] !== undefined && __ENV[name] !== '') {
    return __ENV[name];
  }
  if (envFileValues[name] !== undefined && envFileValues[name] !== '') {
    return envFileValues[name];
  }
  return fallback;
}

function requiredEnv(name) {
  const value = env(name);
  if (!value) {
    throw new Error(`missing required env: ${name}`);
  }
  return value;
}

function integerEnv(name, fallback) {
  return parseInt(env(name, String(fallback)), 10);
}

const gatewayBaseUrl = env('GATEWAY_BASE_URL', 'http://127.0.0.1:8081');
const channelCode = env('CHANNEL_CODE', '0001');
const programId = integerEnv('PROGRAM_ID', 10001);
const ticketCategoryId = integerEnv('TICKET_CATEGORY_ID', 40001);
const distributionMode = env('DISTRIBUTION_MODE', 'express');
const takeTicketMode = env('TAKE_TICKET_MODE', 'paper');
const maxDuration = env('MAX_DURATION', '60s');
const userPool = parseUserPool(open(requiredEnv('USER_POOL_FILE')));
const targetUsers = integerEnv('TARGET_USERS', userPool.length);
const executor = env('EXECUTOR', 'shared-iterations');
const vuCount = integerEnv('VU_COUNT', Math.min(1000, targetUsers));

if (targetUsers > userPool.length) {
  throw new Error(`target users ${targetUsers} exceeds pool size ${userPool.length}`);
}
if (vuCount <= 0) {
  throw new Error(`invalid VU_COUNT: ${vuCount}`);
}

function buildScenario() {
  if (executor === 'per-vu-iterations') {
    return {
      executor: 'per-vu-iterations',
      vus: targetUsers,
      iterations: 1,
      maxDuration,
      exec: 'createOrderOnce',
    };
  }

  if (executor === 'shared-iterations') {
    return {
      executor: 'shared-iterations',
      vus: Math.min(vuCount, targetUsers),
      iterations: targetUsers,
      maxDuration,
      exec: 'createOrderOnce',
    };
  }

  throw new Error(`unsupported EXECUTOR: ${executor}`);
}

export const options = {
  thresholds: {
    http_req_duration: ['p(99)<15000'],
  },
  scenarios: {
    exact_once: buildScenario(),
  },
};

export function createOrderOnce() {
  const userIndex = Math.abs(Number(exec.scenario.iterationInTest) || 0) % userPool.length;
  const userEntry = userPool[userIndex];
  const selectedTicketUserIds = pickTicketUserIds(userEntry);
  const payload = buildOrderCreatePayload({
    programId,
    ticketCategoryId,
    ticketUserIdLiterals: selectedTicketUserIds,
    distributionMode,
    takeTicketMode,
  });

  const response = http.post(`${gatewayBaseUrl}/order/create`, payload, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${userEntry.jwt}`,
      'X-Channel-Code': channelCode,
    },
    tags: {
      endpoint: 'order.create.multi_user_random_seat_count.exact_once',
      seat_count: String(selectedTicketUserIds.length),
    },
  });

  let parsedBody = {};
  try {
    parsedBody = response.json();
  } catch (error) {
    parsedBody = {};
  }

  check(response, {
    'order create status is 2xx': (res) => res.status >= 200 && res.status < 300,
    'order create returns orderNumber': () => Number(parsedBody.orderNumber || 0) > 0,
  });
}
