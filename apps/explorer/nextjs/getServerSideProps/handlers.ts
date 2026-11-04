import type { GetServerSidePropsContext, GetServerSidePropsResult } from 'next';

import type { AdBannerProviders } from 'types/client/adProviders';

import type { Route } from 'nextjs-routes';
import { httpLogger } from 'nextjs/utils/logger';

import config from 'configs/app';
import * as cookies from 'lib/cookies';
import type * as metadata from 'lib/metadata';

import { isLikelyHumanBrowser, isKnownBotRequest } from '../utils/checkRealBrowser';

const adBannerFeature = config.features.adsBanner;

export interface Props<Pathname extends Route['pathname'] = never> {
  query: Route['query'];
  cookies: string;
  referrer: string;
  adBannerProvider: AdBannerProviders | null;
  // if apiData is undefined, Next.js will complain that it is not serializable
  // so we force it to be always present in the props but it can be null
  apiData: metadata.ApiData<Pathname> | null;
  uuid: string;
}

async function reportPageview(body: string): Promise<void> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 1_000);

  try {
    const response = await fetch('https://monitor.blockscout.com/count', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body,
      signal: controller.signal,
    });
    if (!response.ok) {
      httpLogger.logger.warn({ message: 'Pageview reporting failed', code: response.status });
    }
    await response.body?.cancel();
  } catch (error) {
    httpLogger.logger.warn({ message: 'Pageview reporting failed', err: error });
  } finally {
    clearTimeout(timeout);
  }
}

export const base = async <Pathname extends Route['pathname'] = never>({ req, res, query }: GetServerSidePropsContext):
Promise<GetServerSidePropsResult<Props<Pathname>>> => {
  const appProfile = req.headers?.['x-app-profile'] || cookies.getFromCookieString(req.headers.cookie || '', cookies.NAMES.APP_PROFILE);
  const adBannerProvider = (() => {
    if (adBannerFeature.isEnabled) {
      if ('additionalProvider' in adBannerFeature && adBannerFeature.additionalProvider) {
        // we need to get a random ad provider on the server side to keep it consistent with the client side
        const randomIndex = Math.round(Math.random());
        return [ adBannerFeature.provider, adBannerFeature.additionalProvider ][randomIndex];
      } else {
        return adBannerFeature.provider;
      }
    }
    return null;
  })();

  let uuid = cookies.getFromCookieString(req.headers.cookie || '', cookies.NAMES.UUID);
  if (!uuid && appProfile !== 'private') {
    uuid = crypto.randomUUID();
    res.setHeader('Set-Cookie', `${ cookies.NAMES.UUID }=${ uuid }; Path=/explorer${ config.app.protocol === 'https' ? '; Secure' : '' }`);
  }

  const isTrackingDisabled = process.env.DISABLE_TRACKING === 'true' || appProfile === 'private';

  if (!isTrackingDisabled) {
    const isRealUser = isLikelyHumanBrowser(req) && !isKnownBotRequest(req);
    if (isRealUser) {
    // log pageview
      const hostname = req.headers.host;
      const timestamp = new Date().toISOString();
      const chainId = process.env.NEXT_PUBLIC_NETWORK_ID;
      const chainName = process.env.NEXT_PUBLIC_NETWORK_NAME;
      const publicRPC = process.env.NEXT_PUBLIC_NETWORK_RPC_URL;

      void reportPageview(JSON.stringify({
        hostname,
        timestamp,
        chainId,
        chainName,
        publicRPC,
        uuid,
      }));
    }
  }

  return {
    props: {
      query,
      cookies: cookies.onlyExplorerCookies(req.headers.cookie || ''),
      referrer: req.headers.referer || '',
      adBannerProvider: adBannerProvider,
      apiData: null,
      uuid,
    },
  };
};
