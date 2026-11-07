import { publicError } from '@/server/http';

export async function GET() {
  return publicError(
    503,
    'CANDLE_UPSTREAM_RETIRED',
    'Candle data is unavailable because its upstream has been retired'
  );
}
