import { route } from 'nextjs-routes';

import config from 'configs/app';

export default function getMarketplaceAppShareUrl(id: string, origin: string) {
  const path = config.app.basePath + route({ pathname: '/apps/[id]', query: { id: encodeURIComponent(id) } });
  return new URL(path, origin).toString();
}
