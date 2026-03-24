function assertDigits(value, fieldName) {
  if (!/^\d+$/.test(value)) {
    throw new Error(`invalid ${fieldName}: ${value}`);
  }
}

function parseDurationToMilliseconds(value) {
  const normalized = String(value).trim();
  if (!normalized) {
    throw new Error('missing duration');
  }

  const pattern = /(\d+(?:\.\d+)?)(ms|h|m|s)/g;
  let total = 0;
  let cursor = 0;
  let matched = false;

  for (const match of normalized.matchAll(pattern)) {
    if (match.index !== cursor) {
      throw new Error(`invalid duration: ${value}`);
    }

    const amount = Number(match[1]);
    const unit = match[2];
    const unitMilliseconds = unit === 'h' ? 3600000 : unit === 'm' ? 60000 : unit === 's' ? 1000 : 1;

    total += amount * unitMilliseconds;
    cursor += match[0].length;
    matched = true;
  }

  if (!matched || cursor !== normalized.length) {
    throw new Error(`invalid duration: ${value}`);
  }

  return total;
}

function formatDurationMilliseconds(value) {
  if (!Number.isFinite(value) || value <= 0) {
    throw new Error(`invalid duration milliseconds: ${value}`);
  }

  if (value % 1000 === 0) {
    return `${value / 1000}s`;
  }

  return `${value}ms`;
}

export function parseTicketUserIdLiterals(rawValue) {
  return rawValue
    .split(',')
    .map((value) => value.trim())
    .filter(Boolean)
    .map((value) => {
      assertDigits(value, 'ticket user id');
      return value;
    });
}

export function parseUserPool(rawValue) {
  const parsed = JSON.parse(rawValue);
  if (!Array.isArray(parsed) || parsed.length === 0) {
    throw new Error('missing user pool');
  }

  return parsed.map((entry, index) => {
    if (!entry || typeof entry !== 'object') {
      throw new Error(`invalid user pool entry at index ${index}`);
    }

    const jwt = String(entry.jwt || '').trim();
    if (!jwt) {
      throw new Error(`missing user pool jwt at index ${index}`);
    }

    const rawTicketUserIds = Array.isArray(entry.ticketUserIds)
      ? entry.ticketUserIds
      : [entry.ticketUserId];
    const ticketUserIds = rawTicketUserIds
      .map((value) => String(value || '').trim())
      .filter(Boolean);
    if (ticketUserIds.length === 0) {
      throw new Error(`missing user pool ticket user ids at index ${index}`);
    }

    ticketUserIds.forEach((ticketUserId) => {
      assertDigits(ticketUserId, `user pool ticket user id at index ${index}`);
    });

    return {
      jwt,
      ticketUserId: ticketUserIds[0],
      ticketUserIds,
    };
  });
}

export function selectUserPoolEntry(userPool, iterationIndex) {
  if (!Array.isArray(userPool) || userPool.length === 0) {
    throw new Error('missing user pool');
  }

  const normalizedIndex = Math.abs(Number(iterationIndex) || 0) % userPool.length;
  return userPool[normalizedIndex];
}

export function pickTicketUserIds(userEntry, randomFn = Math.random) {
  const hasTicketUserIds =
    userEntry &&
    Array.isArray(userEntry.ticketUserIds) &&
    userEntry.ticketUserIds.length > 0;
  const ticketUserIds = hasTicketUserIds
    ? userEntry.ticketUserIds
    : [String((userEntry && userEntry.ticketUserId) || '').trim()].filter(Boolean);
  if (ticketUserIds.length === 0) {
    throw new Error('missing ticket user ids in user pool entry');
  }

  const maxSelectable = Math.min(3, ticketUserIds.length);
  const rawRandom = Number(randomFn());
  const normalizedRandom = Number.isFinite(rawRandom) ? Math.min(Math.max(rawRandom, 0), 0.999999999999) : 0;
  const selectedCount = Math.min(maxSelectable, Math.floor(normalizedRandom * maxSelectable) + 1);
  return ticketUserIds.slice(0, selectedCount);
}

export function buildOrderCreatePayload({
  programId,
  ticketCategoryId,
  ticketUserIdLiterals,
  distributionMode,
  takeTicketMode,
}) {
  if (!Array.isArray(ticketUserIdLiterals) || ticketUserIdLiterals.length === 0) {
    throw new Error('missing ticket user ids');
  }

  const ticketUserIdsJson = ticketUserIdLiterals
    .map((value) => {
      assertDigits(value, 'ticket user id');
      return value;
    })
    .join(',');

  return `{"programId":${JSON.stringify(programId)},"ticketCategoryId":${JSON.stringify(ticketCategoryId)},"ticketUserIds":[${ticketUserIdsJson}],"distributionMode":${JSON.stringify(distributionMode)},"takeTicketMode":${JSON.stringify(takeTicketMode)}}`;
}

export function buildConstantArrivalRateOptions({
  targetQps,
  duration,
  preAllocatedVUs,
  maxVUs,
}) {
  return {
    thresholds: {
      http_req_failed: ['rate<0.01'],
      http_req_duration: ['p(99)<10000'],
    },
    scenarios: {
      steady_state: {
        executor: 'constant-arrival-rate',
        exec: 'createOrder',
        rate: targetQps,
        timeUnit: '1s',
        duration,
        preAllocatedVUs,
        maxVUs,
      },
    },
  };
}

export function resolveSteadyStartTime({
  warmupDuration,
  iterationSleepSeconds,
  explicitSteadyStartTime,
  guardMilliseconds = 1000,
}) {
  if (explicitSteadyStartTime) {
    return explicitSteadyStartTime;
  }

  const totalMilliseconds =
    parseDurationToMilliseconds(warmupDuration) +
    Math.max(0, Number(iterationSleepSeconds) || 0) * 1000 +
    guardMilliseconds;

  return formatDurationMilliseconds(totalMilliseconds);
}
