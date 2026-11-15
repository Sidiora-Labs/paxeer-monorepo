import { livenessResponse } from '../route';

export const runtime = 'nodejs';
export const dynamic = 'force-dynamic';

export async function GET() {
  return livenessResponse();
}
