import {NextRequest,NextResponse} from 'next/server';
import {pnlApiUrl} from '@/core/sdk-config';
import {validShareId} from '@/widgets/pnl/share-transport';
export const dynamic='force-dynamic';
export const revalidate=0;
type RouteParams={params:Promise<{cardId:string}>};
const headers={'Cache-Control':'no-store','X-Content-Type-Options':'nosniff'};
export async function GET(request: NextRequest, { params }: RouteParams) {
  const { cardId } = await params;
  if (!validShareId(cardId)) return NextResponse.json({ error: 'Card not found' }, { status: 404, headers });

  const deadline = new AbortController();
  const signal = AbortSignal.any([request.signal, deadline.signal]);
  let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
  let rejectRead!: (reason: unknown) => void;
  const interrupted = new Promise<never>((_, reject) => { rejectRead = reject; });
  function cancel() {
    rejectRead(signal.reason);
    void reader?.cancel(signal.reason).catch(() => { });
  }
  signal.addEventListener('abort', cancel, { once: true });
  const timer = setTimeout(() => deadline.abort(new DOMException('Card image deadline exceeded', 'TimeoutError')), 5000);

  async function readImage() {
    const upstream = await fetch(pnlApiUrl(`/v3/portfolio/shares/${cardId}/image.png`), {
      signal, cache: 'no-store', credentials: 'omit', redirect: 'error',
    });
    reader = upstream.body?.getReader();
    try {
      if (signal.aborted) {
        void reader?.cancel(signal.reason).catch(() => { });
        signal.throwIfAborted();
      }
      if (!upstream.ok) {
        await reader?.cancel();
        return NextResponse.json({ error: upstream.status === 410 ? 'Card withdrawn or expired' : 'Card unavailable' }, {
          status: upstream.status >= 400 ? upstream.status : 502, headers,
        });
      }
      if (upstream.headers.get('Content-Type')?.split(';')[0]?.trim().toLowerCase() !== 'image/png') {
        await reader?.cancel();
        return NextResponse.json({ error: 'Invalid card image' }, { status: 502, headers });
      }
      const chunks: Uint8Array[] = [];
      let length = 0;
      if (reader) {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          chunks.push(value);
          length += value.byteLength;
        }
      }
      if (length === 0) return NextResponse.json({ error: 'Invalid card image' }, { status: 502, headers });
      const image = new Uint8Array(length);
      let offset = 0;
      for (const chunk of chunks) {
        image.set(chunk, offset);
        offset += chunk.byteLength;
      }
      return new NextResponse(image.buffer, { status: 200, headers: { ...headers, 'Content-Type': 'image/png' } });
    } finally {
      reader?.releaseLock();
    }
  }

  try {
    signal.throwIfAborted();
    return await Promise.race([readImage(), interrupted]);
  } catch {
    return NextResponse.json({ error: 'Card image service unavailable' }, { status: 502, headers });
  } finally {
    clearTimeout(timer);
    signal.removeEventListener('abort', cancel);
  }
}
