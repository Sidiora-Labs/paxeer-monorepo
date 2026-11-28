import { NextResponse } from 'next/server';
import { launchpadAgentInterfaceDocument } from '@/core/agent-interface';
import { BackendOriginError } from '@/core/backend-origins';

export const dynamic = 'force-dynamic';

export function GET() {
  try {
    return NextResponse.json(launchpadAgentInterfaceDocument(), {
      headers: { 'Cache-Control': 'public, max-age=300', 'X-Content-Type-Options': 'nosniff' },
    });
  } catch (cause) {
    if (!(cause instanceof BackendOriginError)) throw cause;
    return NextResponse.json({ error: 'Agent interface unavailable' }, {
      status: 503, headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' },
    });
  }
}
