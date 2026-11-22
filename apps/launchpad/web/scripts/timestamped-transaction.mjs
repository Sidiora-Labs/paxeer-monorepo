export const TIMESTAMP_GAS_MARGIN = 200000n;

export async function sendTimestampedTransaction(method, ...args) {
  const estimate = await method.estimateGas(...args);
  return method(...args, { gasLimit: estimate + TIMESTAMP_GAS_MARGIN });
}
