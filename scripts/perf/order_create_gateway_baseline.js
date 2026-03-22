import http from 'k6/http';
import { check, sleep } from 'k6';

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

function parseTicketUserIds() {
  return requiredEnv('TICKET_USER_IDS')
    .split(',')
    .map((value) => value.trim())
    .filter(Boolean)
    .map((value) => Number(value));
}

const gatewayBaseUrl = env('GATEWAY_BASE_URL', 'http://127.0.0.1:8081');
const channelCode = env('CHANNEL_CODE', '0001');
const jwt = requiredEnv('JWT');
const ticketUserIds = parseTicketUserIds();
const payload = JSON.stringify({
  programId: integerEnv('PROGRAM_ID', 10001),
  ticketCategoryId: integerEnv('TICKET_CATEGORY_ID', 40001),
  ticketUserIds,
  distributionMode: env('DISTRIBUTION_MODE', 'express'),
  takeTicketMode: env('TAKE_TICKET_MODE', 'paper'),
});

const warmupDuration = env('WARMUP_DURATION', '15s');

export const options = {
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(99)<10000'],
  },
  scenarios: {
    warmup: {
      executor: 'constant-vus',
      exec: 'createOrder',
      vus: integerEnv('WARMUP_VUS', 1),
      duration: warmupDuration,
    },
    steady_state: {
      executor: 'constant-vus',
      exec: 'createOrder',
      vus: integerEnv('STEADY_VUS', 4),
      duration: env('STEADY_DURATION', '60s'),
      startTime: warmupDuration,
    },
  },
};

export function createOrder() {
  const response = http.post(`${gatewayBaseUrl}/order/create`, payload, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${jwt}`,
      'X-Channel-Code': channelCode,
    },
    tags: {
      endpoint: 'order.create',
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

  const sleepSeconds = Number(env('ITERATION_SLEEP_SECONDS', '0'));
  if (sleepSeconds > 0) {
    sleep(sleepSeconds);
  }
}
