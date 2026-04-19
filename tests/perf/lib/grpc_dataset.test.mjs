import test from 'node:test';
import assert from 'node:assert/strict';

import { buildGrpcDataset, mapGrpcDatasetRow } from './grpc_dataset.js';

test('mapGrpcDatasetRow 只保留 userId 与 purchaseToken', () => {
  const mapped = mapGrpcDatasetRow({
    userId: 71001,
    purchaseToken: 'token-1',
    showTimeId: 30001,
    ignored: 'value',
  });

  assert.deepEqual(mapped, {
    userId: 71001,
    purchaseToken: 'token-1',
  });
});

test('buildGrpcDataset 映射整批数据', () => {
  const dataset = buildGrpcDataset([
    { userId: 1, purchaseToken: 'a', extra: true },
    { userId: 2, purchaseToken: 'b', extra: false },
  ]);

  assert.deepEqual(dataset, [
    { userId: 1, purchaseToken: 'a' },
    { userId: 2, purchaseToken: 'b' },
  ]);
});

test('mapGrpcDatasetRow 缺少 userId 时抛错', () => {
  assert.throws(
    () => mapGrpcDatasetRow({ purchaseToken: 'token-1' }),
    /userId/,
  );
});

test('mapGrpcDatasetRow 缺少 purchaseToken 时抛错', () => {
  assert.throws(
    () => mapGrpcDatasetRow({ userId: 1 }),
    /purchaseToken/,
  );
});
