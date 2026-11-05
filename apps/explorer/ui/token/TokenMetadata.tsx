import type { UseQueryResult } from '@tanstack/react-query';
import React from 'react';

import type { TokenInfo, TokenVerifiedInfo } from 'types/api/token';

import type { ResourceError } from 'lib/api/resources';
import { Link } from 'toolkit/chakra/link';
import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';
import { ScanKeyValue } from 'ui/shared/scan';

import TokenDetails from './TokenDetails';

interface Props {
  tokenQuery: UseQueryResult<TokenInfo, ResourceError<unknown>>;
  verifiedInfoQuery: UseQueryResult<TokenVerifiedInfo, ResourceError<unknown>>;
}

const TokenMetadata = ({ tokenQuery, verifiedInfoQuery }: Props) => (
  <>
    <TokenDetails tokenQuery={ tokenQuery }/>
    { verifiedInfoQuery.data && (
      <DetailedInfo.Group label="VERIFIED INFO">
        <ScanKeyValue label="Project">{ verifiedInfoQuery.data.projectName }</ScanKeyValue>
        <ScanKeyValue label="Description" multiRow>{ verifiedInfoQuery.data.projectDescription }</ScanKeyValue>
        <ScanKeyValue label="Sector">{ verifiedInfoQuery.data.projectSector }</ScanKeyValue>
        <ScanKeyValue label="Website">
          <Link external href={ verifiedInfoQuery.data.projectWebsite }>{ verifiedInfoQuery.data.projectWebsite }</Link>
        </ScanKeyValue>
      </DetailedInfo.Group>
    ) }
  </>
);

export default TokenMetadata;
