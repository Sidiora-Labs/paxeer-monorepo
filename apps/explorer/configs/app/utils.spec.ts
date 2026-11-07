import { describe, expect, test } from 'vitest';

import { buildExternalAssetFileName, buildExternalAssetFilePath } from './utils';

describe('external asset filenames and browser paths', () => {
  test.each([
    [ 'NEXT_PUBLIC_FEATURED_NETWORKS', 'featured_networks.json' ],
    [ 'NEXT_PUBLIC_MARKETPLACE_CONFIG_URL', 'marketplace_config.json' ],
    [ 'NEXT_PUBLIC_MARKETPLACE_CATEGORIES_URL', 'marketplace_categories.json' ],
    [ 'NEXT_PUBLIC_MARKETPLACE_GRAPH_LINKS_URL', 'marketplace_graph_links.json' ],
    [ 'NEXT_PUBLIC_FOOTER_LINKS', 'footer_links.json' ],
    [ 'NEXT_PUBLIC_ADDRESS_3RD_PARTY_WIDGETS_CONFIG_URL', 'address_3rd_party_widgets_config.json' ],
    [ 'NEXT_PUBLIC_ZETACHAIN_SERVICE_CHAINS_CONFIG_URL', 'zetachain_service_chains_config.json' ],
    [ 'NEXT_PUBLIC_HOMEPAGE_HIGHLIGHTS_CONFIG', 'homepage_highlights_config.json' ],
  ])('keeps the browser basePath out of the filename for %s', (name, fileName) => {
    const value = 'https://example.com/config%20file.json?token=a%2Fb#section';

    expect(buildExternalAssetFileName(name, value)).toBe(fileName);
    expect(buildExternalAssetFilePath(name, value)).toBe(`/explorer/assets/configs/${ fileName }`);
  });

  test.each([ '[]', '{}' ])('preserves inline JSON handling for %s', (value) => {
    expect(buildExternalAssetFileName('NEXT_PUBLIC_FOOTER_LINKS', value)).toBe('footer_links.json');
    expect(buildExternalAssetFilePath('NEXT_PUBLIC_FOOTER_LINKS', value)).toBe('/explorer/assets/configs/footer_links.json');
  });

  test.each([
    'https://example.com/logo%20dark.svg?size=1%2F2#preview',
    'file:///assets/logo%20dark.svg',
  ])('preserves extension parsing for encoded source URLs: %s', (value) => {
    expect(buildExternalAssetFileName('NEXT_PUBLIC_NETWORK_LOGO', value)).toBe('network_logo.svg');
    expect(buildExternalAssetFilePath('NEXT_PUBLIC_NETWORK_LOGO', value)).toBe('/explorer/assets/configs/network_logo.svg');
  });

  test('preserves the existing source extension case', () => {
    expect(buildExternalAssetFilePath('NEXT_PUBLIC_NETWORK_LOGO', 'https://example.com/logo.PNG'))
      .toBe('/explorer/assets/configs/network_logo.PNG');
  });

  test.each([ '', 'not JSON', 'null', 'false', '0', 'https://example.com/no-extension' ])(
    'preserves an undefined result for unsupported asset value %s',
    (value) => {
      expect(buildExternalAssetFileName('NEXT_PUBLIC_FOOTER_LINKS', value)).toBeUndefined();
      expect(buildExternalAssetFilePath('NEXT_PUBLIC_FOOTER_LINKS', value)).toBeUndefined();
    },
  );
});
