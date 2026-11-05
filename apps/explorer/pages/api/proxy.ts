import type { NextApiRequest, NextApiResponse } from 'next';
import type { RequestInit, Response } from 'node-fetch';
import { Readable } from 'node:stream';

import fetchFactory from 'nextjs/utils/fetchProxy';

import appConfig from 'configs/app';
import isNeedProxy from 'lib/api/isNeedProxy';

function resolveUpstream(selector: string | Array<string> | undefined): URL | undefined {
  if (Array.isArray(selector)) {
    return;
  }

  const upstream = Object.entries(appConfig.apis).find(([ name, api ]) => (
    selector === undefined ? name === 'general' : name === selector || api?.endpoint === selector
  ))?.[1];

  if (!upstream) {
    return;
  }

  try {
    const url = new URL(upstream.endpoint);
    return isSafeDestination(url, url) ? url : undefined;
  } catch (error) {
    return;
  }
}

function isSafeDestination(url: URL, upstream: URL): boolean {
  return [ 'https:', 'http:' ].includes(url.protocol) &&
    url.origin === upstream.origin && !url.username && !url.password;
}

const handler = async(nextReq: NextApiRequest, nextRes: NextApiResponse) => {

  if (!isNeedProxy()) {
    nextRes.status(404).json({ error: 'Not found' });
    return;
  }

  if (!nextReq.url) {
    nextRes.status(500).json({ error: 'No url provided' });
    return;
  }

  const upstream = resolveUpstream(nextReq.headers['x-endpoint']);
  if (!upstream) {
    nextRes.status(400).json({ error: 'Unapproved upstream' });
    return;
  }

  let url: URL;
  try {
    const path = nextReq.url.replace(/^\/(?:explorer\/)?node-api\/proxy(?=\/|\?|$)/, '');
    url = new URL(path || '/', upstream);
    if (!isSafeDestination(url, upstream)) {
      throw new Error('Unapproved destination');
    }
  } catch (error) {
    nextRes.status(400).json({ error: 'Invalid proxy destination' });
    return;
  }

  const fetch = fetchFactory(nextReq);
  let init: Pick<RequestInit, 'body' | 'method'> = {
    body: nextReq.method === 'GET' || nextReq.method === 'HEAD' ? undefined : nextReq.body,
    method: nextReq.method,
  };
  let apiRes: Response;
  try {
    for (let redirects = 0; ; redirects++) {
      apiRes = await fetch(url.toString(), { ...init, redirect: 'manual' });
      if (![ 301, 302, 303, 307, 308 ].includes(apiRes.status)) {
        break;
      }

      const location = apiRes.headers.get('location');
      if (apiRes.body instanceof Readable) {
        apiRes.body.destroy();
      }
      if (!location || redirects >= 5) {
        throw new Error('Invalid upstream redirect');
      }

      const destination = new URL(location, url);
      if (!isSafeDestination(destination, upstream)) {
        throw new Error('Unapproved redirect destination');
      }

      if ((apiRes.status === 303 && init.method !== 'HEAD') ||
        ([ 301, 302 ].includes(apiRes.status) && init.method === 'POST')) {
        init = { method: 'GET' };
      }
      url = destination;
    }
  } catch (error) {
    nextRes.status(502).json({ error: 'Upstream request refused or failed' });
    return;
  }

  // proxy some headers from API
  const HEADERS_TO_PROXY = [
    'x-request-id',
    'content-type',
    'bypass-429-option',
    'x-ratelimit-limit',
    'x-ratelimit-remaining',
    'x-ratelimit-reset',
    'api-v2-temp-token',
  ];

  HEADERS_TO_PROXY.forEach((header) => {
    const value = apiRes.headers.get(header);
    value && nextRes.setHeader(header, value);
  });

  const setCookie = apiRes.headers.raw()['set-cookie'];
  setCookie?.forEach((value) => {
    const scoped = value.replace(/;\s*Domain=[^;]*/gi, '').replace(/;\s*Path=[^;]*/gi, '');
    nextRes.appendHeader('set-cookie', `${ scoped }; Path=/explorer`);
  });

  nextRes.status(apiRes.status).send(apiRes.body);
};

export default handler;

export const config = {
  api: {
    bodyParser: {
      sizeLimit: '100mb',
    },
  },
};
