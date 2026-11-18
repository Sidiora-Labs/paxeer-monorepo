import type { NextApiRequest, NextApiResponse } from 'next';

import type { InferenceMethod } from 'lib/api/services/general/paxeerXInference';
import { isInferenceId, isInferenceMethod, parseInferenceResult } from 'lib/api/services/general/paxeerXInference';

import { isRecord } from './assistant';

export default async function inference(req: NextApiRequest, res: NextApiResponse) {
  res.setHeader('Cache-Control', 'no-store');
  if (req.method !== 'POST') {
    res.setHeader('Allow', 'POST');
    res.status(405).json({ error: 'Method not allowed' });
    return;
  }

  const body: unknown = req.body;
  if (!isRecord(body) || !isInferenceMethod(body.method)) {
    res.status(400).json({ error: 'Unsupported inference method' });
    return;
  }
  const { method, params } = body;
  if (!Array.isArray(params) || (method === 'px_getInferenceRooms' ? params.length !== 0 :
    params.length !== 1 || !isInferenceId(params[0]))) {
    res.status(400).json({ error: 'Invalid inference parameters' });
    return;
  }

  const nodeRoot = process.env.EXPLORER_INFERENCE_NODE_URL;
  let url: URL;
  try {
    if (!nodeRoot) throw new Error('Missing node root');
    url = new URL(nodeRoot);
    if (![ 'http:', 'https:' ].includes(url.protocol) || url.pathname !== '/' || url.username || url.password || url.search || url.hash) {
      throw new Error('Invalid node root');
    }
  } catch (error) {
    res.status(503).json({ error: 'Inference is not configured' });
    return;
  }

  try {
    const response = await fetch(url.toString(), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
      redirect: 'error',
      signal: AbortSignal.timeout(15_000),
    });
    if (!response.ok) throw new Error('Inference upstream failed');
    const payload: unknown = await response.json();
    const answer = inferenceResponse(method, params, payload);
    res.status(answer.status).json(answer.body);
  } catch (error) {
    res.status(502).json({ error: 'Inference request failed. Please try again later.' });
  }
}

export function inferenceResponse(method: InferenceMethod, params: Array<unknown>, payload: unknown) {
  try {
    if (!isRecord(payload) || payload.jsonrpc !== '2.0' || payload.id !== 1) {
      throw new Error('Invalid inference response');
    }
    if ('error' in payload) {
      if (!('result' in payload) && isRecord(payload.error) && isRecord(payload.error.data) &&
        payload.error.data.code === 'inference_program_not_configured') {
        return { status: 200, body: { state: 'not_active', code: 'inference_program_not_configured' } } as const;
      }
      throw new Error('Invalid inference response');
    }
    if (!('result' in payload)) {
      throw new Error('Invalid inference response');
    }
    const result = parseInferenceResult(method, payload.result);
    if ((method === 'px_getInferenceRoom' && 'room_id' in result && result.room_id !== params[0]) ||
      (method === 'px_getInferenceDeal' && 'deal_id' in result && result.deal_id !== params[0]) ||
      (method === 'px_getInferencePeer' && 'peer_key' in result && result.peer_key !== params[0]) ||
      (method === 'px_getInferenceReceipts' && Array.isArray(result) && result.some((receipt) => 'deal_id' in receipt && receipt.deal_id !== params[0]))) {
      throw new Error('Mismatched inference response');
    }
    return { status: 200, body: { result } } as const;
  } catch (error) {
    return { status: 502, body: { error: 'Inference request failed. Please try again later.' } } as const;
  }
}

export const config = { api: { bodyParser: { sizeLimit: '16kb' } } };
