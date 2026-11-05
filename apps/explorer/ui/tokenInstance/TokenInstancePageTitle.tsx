import { Flex } from '@chakra-ui/react';
import React from 'react';

import type { TokenInfo, TokenInstance } from 'types/api/token';

import { useMultichainContext } from 'lib/contexts/multichain';
import { getTokenTypeName } from 'lib/token/tokenTypes';
import { Link } from 'toolkit/chakra/link';
import * as regexp from 'toolkit/utils/regexp';
import AccountActionsMenu from 'ui/shared/AccountActionsMenu/AccountActionsMenu';
import AddressAddToWallet from 'ui/shared/address/AddressAddToWallet';
import TokenEntityDefault, * as TokenEntity from 'ui/shared/entities/token/TokenEntity';
import PageTitle from 'ui/shared/Page/PageTitle';
import { ScanMethodChip } from 'ui/shared/scan';

interface Props {
  isLoading: boolean;
  token: TokenInfo | undefined;
  instance: TokenInstance | undefined;
  hash: string | undefined;
}

const TokenInstancePageTitle = ({ isLoading, token, instance, hash }: Props) => {
  const multichainContext = useMultichainContext();

  const title = (() => {
    if (typeof instance?.metadata?.name === 'string') {
      return instance.metadata.name;
    }

    if (!instance) {
      return `Unknown token instance`;
    }

    if (token?.name || token?.symbol) {
      return (token.name || token.symbol) + ' #' + instance.id;
    }

    return `ID ${ instance.id }`;
  })();

  const standard = token ? getTokenTypeName(token.type, multichainContext?.chain?.app_config) : undefined;

  const appLink = (() => {
    if (!instance?.external_app_url) {
      return null;
    }

    try {
      const url = regexp.URL_PREFIX.test(instance.external_app_url) ?
        new URL(instance.external_app_url) :
        new URL('https://' + instance.external_app_url);

      return (
        <Link external href={ url.toString() } variant="underlaid" loading={ isLoading } data-token-instance-app-link>
          { url.hostname || instance.external_app_url }
        </Link>
      );
    } catch (error) {
      return (
        <Link external href={ instance.external_app_url } variant="underlaid" loading={ isLoading } data-token-instance-app-link>
          View in app
        </Link>
      );
    }
  })();

  const chipRow = (
    <Flex
      data-token-chip-row
      alignItems="center"
      justifyContent="space-between"
      w="100%"
      minW={ 0 }
      columnGap={ 3 }
      rowGap={ 3 }
      flexWrap="wrap"
    >
      <Flex alignItems="center" minW={ 0 } columnGap={ 2 } rowGap={ 2 } flexWrap="wrap" data-token-chips>
        { standard && <ScanMethodChip method={ standard } isLoading={ isLoading }/> }
        { token && (
          <TokenEntityDefault
            token={ token }
            isLoading={ isLoading }
            noSymbol
            noCopy
            jointSymbol
            variant="subheading"
            w="auto"
            maxW="400px"
            chain={ multichainContext?.chain }
          />
        ) }
      </Flex>
      <Flex alignItems="center" columnGap={ 2 } rowGap={ 2 } flexWrap="wrap" data-token-actions>
        { appLink }
        { !isLoading && token && <AddressAddToWallet token={ token } tokenId={ instance?.id } variant="button"/> }
        <AccountActionsMenu isLoading={ isLoading } showUpdateMetadataItem/>
      </Flex>
    </Flex>
  );

  return (
    <PageTitle
      kind="TOKEN"
      title={ title }
      identifier={ hash }
      beforeTitle={ token ? (
        <TokenEntity.Icon
          token={ token }
          isLoading={ isLoading }
          variant="heading"
          chain={ multichainContext?.chain }
        />
      ) : null }
      tags={ chipRow }
      isLoading={ isLoading }
    />
  );
};

export default React.memo(TokenInstancePageTitle);
