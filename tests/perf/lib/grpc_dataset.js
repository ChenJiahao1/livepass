export function mapGrpcDatasetRow(row) {
  if (!row || typeof row !== 'object') {
    throw new Error('dataset row must be an object');
  }
  if (row.userId === undefined || row.userId === null || row.userId === '') {
    throw new Error(`dataset item missing userId: ${JSON.stringify(row)}`);
  }
  if (!row.purchaseToken) {
    throw new Error(`dataset item missing purchaseToken: ${JSON.stringify(row)}`);
  }

  return {
    userId: row.userId,
    purchaseToken: row.purchaseToken,
  };
}

export function buildGrpcDataset(rows) {
  if (!Array.isArray(rows) || rows.length === 0) {
    throw new Error('grpc dataset is empty');
  }

  return rows.map((row) => mapGrpcDatasetRow(row));
}
