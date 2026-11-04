import type { NextApiRequest, NextApiResponse } from 'next';

import type { ApiName } from 'lib/api/types';

import { httpLogger } from 'nextjs/utils/logger';

import { RESOURCES } from 'lib/api/resources';
import metrics from 'lib/monitoring/metrics';

export function createInvalidApiSchemaHandler(
  counter = metrics?.invalidApiSchema,
  logger = httpLogger.logger,
) {
  let diagnosticWindowStart = Date.now();
  let diagnosticCount = 0;

  function logDiagnostic(details: { resource?: string; url?: string; errorCategory: 'invalid_response_schema' | 'invalid_report' }) {
    const now = Date.now();
    if (now - diagnosticWindowStart >= 60_000) {
      diagnosticWindowStart = now;
      diagnosticCount = 0;
    }

    if (diagnosticCount < 10) {
      diagnosticCount++;
      logger.warn({ message: 'Invalid API schema report', ...details });
    }
  }

  return async function handler(req: NextApiRequest, res: NextApiResponse) {
    try {
      const payload: unknown = typeof req.body === 'string' ? JSON.parse(req.body) : req.body;

      if (!payload || typeof payload !== 'object' || !('resource' in payload) || typeof payload.resource !== 'string') {
        throw new Error('Resource not found');
      }

      const [ apiName, resourceName, extra ] = payload.resource.split(':');
      const api = Object.prototype.hasOwnProperty.call(RESOURCES, apiName) ? RESOURCES[apiName as ApiName] : undefined;

      if (!api || extra !== undefined || !Object.prototype.hasOwnProperty.call(api, resourceName)) {
        throw new Error('Resource not found');
      }

      let diagnosticUrl: string | undefined;
      if ('url' in payload && payload.url !== undefined && payload.url !== '') {
        if (typeof payload.url !== 'string') {
          throw new Error('Invalid diagnostic URL');
        }
        const url = new URL(payload.url);
        if (url.protocol !== 'https:' && url.protocol !== 'http:') {
          throw new Error('Invalid diagnostic URL');
        }
        url.username = '';
        url.password = '';
        url.search = '';
        url.hash = '';
        diagnosticUrl = url.toString().slice(0, 512);
      }

      counter?.inc({ resource: payload.resource, url: 'invalid_response_schema' });
      logDiagnostic({ resource: payload.resource, errorCategory: 'invalid_response_schema', url: diagnosticUrl });
    } catch {
      logDiagnostic({ errorCategory: 'invalid_report' });
    }
    res.status(200).json({ status: 'ok' });
  };
}

export default createInvalidApiSchemaHandler();
