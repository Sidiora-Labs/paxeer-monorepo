import type { NextRequest } from 'next/server';
import { NextResponse } from 'next/server';

import * as cookiesLib from 'lib/cookies';

const APP_PROFILE_HEADER = 'x-app-profile';
const APP_PROFILE_QUERY_PARAM = 'app-profile';
const PRIVATE_PROFILE_VALUE = 'private';

export function resolveRequestProfile(req: NextRequest): 'private' | 'default' {
  const headerValue = req.headers.get(APP_PROFILE_HEADER);
  const queryValue = req.nextUrl.searchParams.get(APP_PROFILE_QUERY_PARAM);
  const cookieValue = req.cookies.get(cookiesLib.NAMES.APP_PROFILE)?.value;

  return (headerValue || queryValue || cookieValue) === PRIVATE_PROFILE_VALUE ? 'private' : 'default';
}

export default function appProfileMiddleware(req: NextRequest, res: NextResponse) {
  const profileValue = resolveRequestProfile(req);

  // CSP reads this request; SSR receives the same decision through Next.js request overrides.
  req.headers.set(APP_PROFILE_HEADER, profileValue);
  const forwarded = NextResponse.next({ request: { headers: req.headers } });
  forwarded.headers.forEach((value, name) => {
    if (name === 'x-middleware-override-headers' || name.startsWith('x-middleware-request-')) {
      res.headers.set(name, value);
    }
  });

  if (profileValue === PRIVATE_PROFILE_VALUE) {
    res.cookies.set(cookiesLib.NAMES.APP_PROFILE, PRIVATE_PROFILE_VALUE, cookiesLib.getDefaultAttributes());
  } else {
    res.cookies.delete({
      name: cookiesLib.NAMES.APP_PROFILE,
      ...cookiesLib.getDefaultAttributes(),
    });
  }
}
