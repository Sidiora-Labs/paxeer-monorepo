export function pushUnsubscribeMessage(
  endpoint: string,
  walletAddress: string,
  issuedAt: number
): string {
  return JSON.stringify({
    action: 'paxport:push-unsubscribe',
    endpoint,
    walletAddress: walletAddress.toLowerCase(),
    issuedAt,
  });
}
