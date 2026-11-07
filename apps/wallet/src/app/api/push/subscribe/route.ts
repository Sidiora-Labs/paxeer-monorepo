import { NextRequest, NextResponse } from 'next/server';
import { verifyPushOwnership } from '@/server/push-ownership';
import { parseStoredSubscription, upsertSubscription } from '@/server/push-store';
import { boundaryErrorResponse, HttpBoundaryError, readBoundedJson } from '@/server/http';
import { removeOwnedSubscription } from './unsubscribe';

function record(input: unknown): Record<string, unknown> {
  if (typeof input !== 'object' || input === null || Array.isArray(input)) {
    throw new HttpBoundaryError(400, 'SUBSCRIPTION_INVALID', 'Push subscription is invalid');
  }
  return input as Record<string, unknown>;
}

export async function POST(request: NextRequest) {
  try {
    const body = record(await readBoundedJson(request, 8_192));
    if (typeof body.signature !== 'string' || typeof body.issuedAt !== 'number') {
      throw new HttpBoundaryError(
        401,
        'SUBSCRIPTION_UNPROVEN',
        'Push subscription ownership could not be verified'
      );
    }
    const keys = record(body.keys);
    const subscription = parseStoredSubscription({
      endpoint: body.endpoint,
      keys: { p256dh: keys.p256dh, auth: keys.auth },
      walletAddress: body.walletAddress,
      userAgent: request.headers.get('user-agent') ?? undefined,
      createdAt: Date.now(),
      lastActiveAt: Date.now(),
    });
    verifyPushOwnership(
      {
        endpoint: subscription.endpoint,
        walletAddress: subscription.walletAddress,
        issuedAt: body.issuedAt,
        signature: body.signature,
      },
      Math.floor(Date.now() / 1_000)
    );
    await upsertSubscription(subscription);
    return NextResponse.json({ ok: true }, { headers: { 'Cache-Control': 'no-store' } });
  } catch (error) {
    return boundaryErrorResponse(error);
  }
}

export async function DELETE(request: NextRequest) {
  try {
    const body = record(await readBoundedJson(request, 4_096));
    if (
      typeof body.endpoint !== 'string' ||
      body.endpoint.length < 12 ||
      body.endpoint.length > 2_048
    ) {
      throw new HttpBoundaryError(400, 'SUBSCRIPTION_INVALID', 'Push subscription is invalid');
    }
    let endpoint: URL;
    try {
      endpoint = new URL(body.endpoint);
    } catch {
      throw new HttpBoundaryError(400, 'SUBSCRIPTION_INVALID', 'Push subscription is invalid');
    }
    if (endpoint.protocol !== 'https:') {
      throw new HttpBoundaryError(400, 'SUBSCRIPTION_INVALID', 'Push subscription is invalid');
    }
    if (
      typeof body.walletAddress !== 'string' ||
      !/^0x[0-9a-fA-F]{40}$/.test(body.walletAddress) ||
      typeof body.issuedAt !== 'number' ||
      typeof body.signature !== 'string'
    ) {
      throw new HttpBoundaryError(
        401,
        'SUBSCRIPTION_UNPROVEN',
        'Push subscription ownership could not be verified'
      );
    }
    if (Object.keys(body).length !== 4) {
      throw new HttpBoundaryError(400, 'SUBSCRIPTION_INVALID', 'Push subscription is invalid');
    }
    const removed = await removeOwnedSubscription({
      endpoint: body.endpoint,
      walletAddress: body.walletAddress,
      issuedAt: body.issuedAt,
      signature: body.signature,
    });
    return NextResponse.json({ ok: true, removed }, { headers: { 'Cache-Control': 'no-store' } });
  } catch (error) {
    return boundaryErrorResponse(error);
  }
}
