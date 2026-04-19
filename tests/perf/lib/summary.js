function metricCount(data, name) {
  if (!data.metrics[name] || !data.metrics[name].values) {
    return 0;
  }

  return data.metrics[name].values.count || 0;
}

function metricRate(data, name) {
  if (!data.metrics[name] || !data.metrics[name].values) {
    return 0;
  }

  return data.metrics[name].values.rate || 0;
}

function metricTrend(data, name, field) {
  if (!data.metrics[name] || !data.metrics[name].values) {
    return 0;
  }

  return data.metrics[name].values[field] || 0;
}

export function handlePerfSummary(data, context = {}) {
  const createSuccessCount = metricCount(data, 'create_order_success_count');
  const pollSuccessCount = metricCount(data, 'poll_success_count');
  const inventoryInsufficientCount = metricCount(data, 'inventory_insufficient_count');
  const businessFailureCount = metricCount(data, 'business_failure_count');
  const createTotal = metricCount(data, 'create_order_duration')
    || createSuccessCount + inventoryInsufficientCount + businessFailureCount;
  const createSuccessRate = metricRate(data, 'create_order_success_rate');
  const pollSuccessRate = metricRate(data, 'poll_success_rate');

  const summary = {
    datasetSize: context.datasetSize || 0,
    successRate: createSuccessRate,
    pollSuccessRate,
    createTotal,
    createSuccessCount,
    pollSuccessCount,
    inventoryInsufficientCount,
    businessFailureCount,
    p95: metricTrend(data, 'create_order_duration', 'p(95)'),
    p99: metricTrend(data, 'create_order_duration', 'p(99)'),
    avg: metricTrend(data, 'create_order_duration', 'avg'),
    purchaseTokenVerifyAvg: metricTrend(data, 'purchase_token_verify_duration', 'avg'),
    purchaseTokenVerifyP95: metricTrend(data, 'purchase_token_verify_duration', 'p(95)'),
    purchaseTokenVerifyP99: metricTrend(data, 'purchase_token_verify_duration', 'p(99)'),
    redisAdmitAvg: metricTrend(data, 'redis_admit_duration', 'avg'),
    redisAdmitP95: metricTrend(data, 'redis_admit_duration', 'p(95)'),
    redisAdmitP99: metricTrend(data, 'redis_admit_duration', 'p(99)'),
    asyncDispatchScheduleAvg: metricTrend(data, 'async_dispatch_schedule_duration', 'avg'),
    asyncDispatchScheduleP95: metricTrend(data, 'async_dispatch_schedule_duration', 'p(95)'),
    asyncDispatchScheduleP99: metricTrend(data, 'async_dispatch_schedule_duration', 'p(99)'),
  };

  return {
    stdout: `${JSON.stringify(summary, null, 2)}\n`,
    'summary.json': JSON.stringify(summary, null, 2),
  };
}
