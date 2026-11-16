export function getNetworkUtilizationColor(load: string) {
  switch (load) {
    case 'low': return 'positive';
    case 'medium': return 'warning';
    case 'high': return 'negative';
    default: throw new Error('Unsupported network utilization level');
  }
}
