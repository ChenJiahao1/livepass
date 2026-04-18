import { SharedArray } from 'k6/data';

const datasetCache = new Map();

function parseDataset(datasetPath) {
  const raw = open(datasetPath);
  const parsed = JSON.parse(raw);

  if (!Array.isArray(parsed) || parsed.length === 0) {
    throw new Error(`dataset is empty: ${datasetPath}`);
  }

  for (const item of parsed) {
    if (!item.purchaseToken) {
      throw new Error(`dataset item missing purchaseToken: ${JSON.stringify(item)}`);
    }
  }

  return parsed;
}

export function loadDataset(datasetPath) {
  if (!datasetCache.has(datasetPath)) {
    datasetCache.set(
      datasetPath,
      new SharedArray(`perf-dataset:${datasetPath}`, () => parseDataset(datasetPath)),
    );
  }

  return datasetCache.get(datasetPath);
}
