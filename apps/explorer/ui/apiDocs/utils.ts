import type { SwaggerRequest } from './types';
import { getFeaturePayload } from 'configs/app/features/types';

import config from 'configs/app';
import type { ApiPropsBase } from 'configs/app/apis';

const microserviceRequestInterceptorFactory = (api: ApiPropsBase) => (req: SwaggerRequest) => {
  try {
    const url = new URL(req.url);
    if (api?.basePath && !url.pathname.includes(api.basePath)) {
      url.pathname = (api?.basePath ?? '') + url.pathname;
    }
    req.url = url.toString();
  } catch (error) {}
  return req;
};

const getMicroserviceSwaggerUrl = (api: ApiPropsBase) => `${ api.endpoint }${ api.basePath ?? '' }/api/v1/docs/swagger.yaml`;

// The spec shipped with the frontend build (public/openapi.yaml).
export const CORE_API_SPEC_PUBLIC_URL = '/openapi.yaml';

// The spec the backend renders for its API v2 controllers.
export const getCoreApiSpecBackendUrl = (): string | undefined => {
  if (!config.apis.general) {
    return;
  }

  return `${ config.apis.general.endpoint }${ config.apis.general.basePath ?? '' }/api/v2/openapi`;
};

// Ordered list of locations the core API spec is looked up from:
// an explicitly configured URL, the frontend public dir, then the backend.
export const getCoreApiSpecCandidates = (): Array<string> => {
  return [
    getFeaturePayload(config.features.apiDocs)?.coreApiSpecUrl,
    CORE_API_SPEC_PUBLIC_URL,
    getCoreApiSpecBackendUrl(),
  ].filter(Boolean);
};

export const resolveCoreApiSpecUrl = async(candidates: Array<string>): Promise<string | undefined> => {
  for (const candidate of candidates) {
    try {
      const response = await fetch(candidate, { method: 'HEAD' });
      if (response.ok) {
        return candidate;
      }
    } catch (error) {}
  }

  return candidates[0];
};

export const REST_API_SECTIONS = [
  config.apis.general && {
    id: 'blockscout-core-api',
    title: 'Blockscout core API',
    swagger: {
      // default swagger URL, will be replaced with the first reachable spec location (see getCoreApiSpecCandidates)
      url: CORE_API_SPEC_PUBLIC_URL,
      requestInterceptor: (req: SwaggerRequest) => {
        if (!config.apis.general) {
          return req;
        }

        if (!req.loadSpec) {
          const newUrl = (() => {
            try {
              const DEFAULT_SERVER = 'blockscout.com/poa/core';
              const DEFAULT_SERVER_NEW = 'http://localhost/api';
              const backendApiUrl = `${ config.apis.general.endpoint }${ config.apis.general.basePath }/api`;

              if (req.url.includes(DEFAULT_SERVER)) {
                const url = new URL(req.url.replace(DEFAULT_SERVER, config.apis.general.host));

                url.protocol = config.apis.general.protocol + ':';

                if (config.apis.general.port) {
                  url.port = config.apis.general.port;
                }
                return url;
              }

              if (req.url.includes(DEFAULT_SERVER_NEW)) {
                return new URL(req.url.replace(DEFAULT_SERVER_NEW, backendApiUrl));
              }

              // a spec served from the frontend origin declares the server as "/",
              // so requests land on the frontend host and have to be sent to the backend instead
              if (typeof window !== 'undefined') {
                const url = new URL(req.url, window.location.origin);
                if (url.origin === window.location.origin && url.pathname.startsWith('/api/')) {
                  return new URL(`${ backendApiUrl }${ url.pathname.slice('/api'.length) }${ url.search }`);
                }
              }

            } catch (error) {}
          })();

          if (newUrl) {
            req.url = newUrl.toString();
          }

        }
        return req;
      },
    },
  },
  config.apis.stats && {
    id: 'stats-api',
    title: 'Stats API',
    swagger: {
      url: getMicroserviceSwaggerUrl(config.apis.stats),
      requestInterceptor: microserviceRequestInterceptorFactory(config.apis.stats),
    },
  },
  config.apis.bens && {
    id: 'bens-api',
    title: 'Name service API',
    swagger: {
      url: getMicroserviceSwaggerUrl(config.apis.bens),
      requestInterceptor: microserviceRequestInterceptorFactory(config.apis.bens),
    },
  },
  config.apis.userOps && {
    id: 'user-ops-api',
    title: 'User ops indexer API',
    swagger: {
      url: getMicroserviceSwaggerUrl(config.apis.userOps),
      requestInterceptor: microserviceRequestInterceptorFactory(config.apis.userOps),
    },
  },
  config.apis.tac && {
    id: 'tac-api',
    title: 'TAC operation lifecycle API',
    swagger: {
      url: getMicroserviceSwaggerUrl(config.apis.tac),
      requestInterceptor: microserviceRequestInterceptorFactory(config.apis.tac),
    },
  },
  config.apis.zetachain && {
    id: 'zetachain-api',
    title: 'Zetachain CCTX API',
    swagger: {
      url: getMicroserviceSwaggerUrl(config.apis.zetachain),
      requestInterceptor: microserviceRequestInterceptorFactory(config.apis.zetachain),
    },
  },
  config.apis.interchainIndexer && {
    id: 'interchain-indexer-api',
    title: 'Interchain indexer API',
    swagger: {
      url: getMicroserviceSwaggerUrl(config.apis.interchainIndexer),
      requestInterceptor: microserviceRequestInterceptorFactory(config.apis.interchainIndexer),
    },
  },
].filter(Boolean);
