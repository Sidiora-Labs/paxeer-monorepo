import type { NextApiRequest, NextApiResponse } from 'next';
import type { LookupAddress } from 'node:dns';
import { lookup } from 'node:dns/promises';
import { request as httpRequest } from 'node:http';
import { request as httpsRequest } from 'node:https';
import { BlockList, isIP } from 'node:net';

import fetchApi from 'nextjs/utils/fetchApi';
import { httpLogger } from 'nextjs/utils/logger';

import metrics from 'lib/monitoring/metrics';
import getQueryParamString from 'lib/router/getQueryParamString';
import { SECOND } from 'toolkit/utils/consts';

const blockedIpv4 = new BlockList();
for (const [ address, prefix ] of [
  [ '0.0.0.0', 8 ], [ '10.0.0.0', 8 ], [ '100.64.0.0', 10 ], [ '127.0.0.0', 8 ],
  [ '169.254.0.0', 16 ], [ '172.16.0.0', 12 ], [ '192.0.0.0', 24 ], [ '192.0.2.0', 24 ],
  [ '192.88.99.0', 24 ], [ '192.168.0.0', 16 ], [ '198.18.0.0', 15 ], [ '198.51.100.0', 24 ],
  [ '203.0.113.0', 24 ], [ '224.0.0.0', 3 ],
] as const) {
  blockedIpv4.addSubnet(address, prefix, 'ipv4');
}

const globalIpv6 = new BlockList();
globalIpv6.addSubnet('2000::', 3, 'ipv6');
const blockedIpv6 = new BlockList();
blockedIpv6.addSubnet('2001::', 23, 'ipv6');
blockedIpv6.addSubnet('2001:db8::', 32, 'ipv6');
blockedIpv6.addSubnet('2002::', 16, 'ipv6');
blockedIpv6.addSubnet('3fff::', 20, 'ipv6');

export function isPublicMediaAddress(address: string): boolean {
  switch (isIP(address)) {
    case 4:
      return !blockedIpv4.check(address, 'ipv4');
    case 6:
      return globalIpv6.check(address, 'ipv6') && !blockedIpv6.check(address, 'ipv6');
    default:
      return false;
  }
}

export async function resolveMediaDestination(input: string, signal: AbortSignal): Promise<{ url: URL; address: LookupAddress }> {
  signal.throwIfAborted();
  if (input.length > 8192) {
    throw new Error('Media URL is too long');
  }
  const url = new URL(input);
  if (![ 'http:', 'https:' ].includes(url.protocol) || url.username || url.password) {
    throw new Error('Invalid media URL');
  }
  const hostname = url.hostname.replace(/^\[|\]$/g, '');
  const family = isIP(hostname);
  let addresses: Array<LookupAddress>;
  if (family) {
    addresses = [ { address: hostname, family } ];
  } else {
    let onAbort: () => void = () => {};
    try {
      addresses = await Promise.race([
        lookup(hostname, { all: true, verbatim: true }),
        new Promise<never>((_resolve, reject) => {
          onAbort = () => reject(new Error('Media DNS lookup aborted'));
          signal.addEventListener('abort', onAbort, { once: true });
          if (signal.aborted) {
            onAbort();
          }
        }),
      ]);
    } finally {
      signal.removeEventListener('abort', onAbort);
    }
  }
  signal.throwIfAborted();
  if (!addresses.length || addresses.length > 64 || addresses.some(({ address }) => !isPublicMediaAddress(address))) {
    throw new Error('Media destination is not public');
  }
  return { url, address: addresses[0] };
}

export async function resolveMediaRedirect(current: URL, location: string | null, redirects: number, signal: AbortSignal) {
  if (redirects >= 3 || !location) {
    throw new Error('Media redirect limit or missing destination');
  }
  return resolveMediaDestination(new URL(location, current).href, signal);
}

export async function fetchOutboundMediaHeaders(input: string, signal: AbortSignal) {
  let destination = await resolveMediaDestination(input, signal);
  for (let redirects = 0; ; redirects++) {
    const { url, address } = destination;
    const pinnedUrl = new URL(url);
    pinnedUrl.hostname = address.family === 6 ? `[${ address.address }]` : address.address;
    const request = url.protocol === 'https:' ? httpsRequest : httpRequest;
    const response = await new Promise<{ status: number; contentType: string | null; location: string | null }>((resolve, reject) => {
      // Connect to the validated IP, retaining the original HTTP host and TLS identity.
      const requestOptions = {
        method: 'HEAD',
        headers: { host: url.host },
        servername: isIP(url.hostname.replace(/^\[|\]$/g, '')) ? undefined : url.hostname,
        agent: false as const,
        signal,
        maxHeaderSize: 16 * 1024,
      };
      const outgoing = request(pinnedUrl, requestOptions, (incoming) => {
        const result = {
          status: incoming.statusCode ?? 0,
          contentType: incoming.headers['content-type'] ?? null,
          location: incoming.headers.location ?? null,
        };
        incoming.destroy();
        resolve(result);
      });
      outgoing.on('error', reject);
      outgoing.end();
    });
    if (![ 301, 302, 303, 307, 308 ].includes(response.status)) {
      return response;
    }
    destination = await resolveMediaRedirect(url, response.location, redirects, signal);
  }
}

export default async function tokenInstanceMediaTypeHandler(req: NextApiRequest, res: NextApiResponse) {
  const controller = new AbortController();
  const timeout = setTimeout(() => {
    controller.abort('Request to media asset timed out');
  }, 10 * SECOND);

  try {
    const field = getQueryParamString(req.query.field) as 'animation_url' | 'image_url';
    if (![ 'animation_url', 'image_url' ].includes(field)) {
      throw new Error('Invalid field parameter');
    }

    const apiData = await fetchApi({
      resource: 'general:token_instance',
      pathParams: { hash: getQueryParamString(req.query.hash), id: getQueryParamString(req.query.id) },
      timeout: SECOND,
    });
    if (!apiData) {
      throw new Error('Failed to fetch token instance');
    }

    const mediaUrl = apiData[field];
    if (!mediaUrl) {
      throw new Error('No media URL found');
    }

    const mediaRequestEndTime = metrics?.apiRequestDuration.startTimer();
    const mediaResponse = await fetchOutboundMediaHeaders(mediaUrl, controller.signal);
    const mediaRequestDuration = mediaRequestEndTime?.({ route: '/media-type', code: mediaResponse.status });

    if (mediaResponse.status === 200) {
      httpLogger.logger.info({ message: 'API fetch', url: mediaUrl, code: mediaResponse.status, duration: mediaRequestDuration });
    } else {
      httpLogger.logger.error({ message: 'API fetch', url: mediaUrl, code: mediaResponse.status, duration: mediaRequestDuration });
      throw new Error();
    }

    const contentType = mediaResponse.contentType;
    const mediaType = (() => {
      if (contentType?.startsWith('video')) {
        return 'video';
      }

      if (contentType?.startsWith('image')) {
        return 'image';
      }

      if (contentType?.startsWith('text/html')) {
        return 'html';
      }
    })();

    res.status(200).json({ type: mediaType });
  } catch (error) {
    res.status(200).json({ type: undefined });
  } finally {
    clearTimeout(timeout);
  }
}
