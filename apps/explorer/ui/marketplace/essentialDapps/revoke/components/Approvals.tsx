import { Box, Flex } from '@chakra-ui/react';

import type { EssentialDappsChainConfig } from 'types/client/marketplace';
import type { AllowanceType } from 'types/client/revoke';

import { Alert } from 'toolkit/chakra/alert';
import { Button } from 'toolkit/chakra/button';
import DataListDisplay from 'ui/shared/DataListDisplay';

import ApprovalsListItem from './ApprovalsListItem';
import ApprovalsTable from './ApprovalsTable';

type Props = {
  selectedChain: EssentialDappsChainConfig | undefined;
  approvals: Array<AllowanceType>;
  isLoading?: boolean;
  isError?: boolean;
  isIncomplete?: boolean;
  isRetrying?: boolean;
  onRetry?: () => void;
  isAddressMatch?: boolean;
  hideApproval: (approval: AllowanceType) => void;
};

export default function Approvals({
  selectedChain,
  approvals,
  isLoading,
  isError,
  isIncomplete,
  isRetrying,
  onRetry,
  isAddressMatch,
  hideApproval,
}: Props) {
  const hasScanFailure = Boolean(isError || isIncomplete);
  const scanAlert = hasScanFailure ? (
    <Alert status="warning" mb={ 4 }>
      <Flex alignItems="center" gap={ 3 } flexWrap="wrap">
        { approvals.length ?
          'Approval scan is incomplete. Only known approvals are shown; other permissions may still be active.' :
          'Unable to complete the approval scan. Your account may still have active approvals.' }
        { onRetry && (
          <Button size="sm" variant="outline" onClick={ onRetry } loading={ isRetrying } loadingText="Retrying scan">
            Retry scan
          </Button>
        ) }
      </Flex>
    </Alert>
  ) : undefined;

  const content = (
    <>
      <Flex hideFrom="lg" flexDirection="column">
        { approvals.map((approval, index) => (
          <ApprovalsListItem
            key={ index }
            selectedChain={ selectedChain }
            approval={ approval }
            isLoading={ isLoading }
            isAddressMatch={ isAddressMatch }
            hideApproval={ hideApproval }
          />
        )) }
      </Flex>
      <Box hideBelow="lg">
        <ApprovalsTable
          selectedChain={ selectedChain }
          approvals={ approvals }
          isLoading={ isLoading }
          isAddressMatch={ isAddressMatch }
          hideApproval={ hideApproval }
        />
      </Box>
    </>
  );

  return (
    <DataListDisplay
      itemsNum={ approvals.length }
      isError={ hasScanFailure && !approvals.length }
      actionBar={ scanAlert }
      showActionBarIfError
      hasActiveFilters
      emptyStateProps={{
        term: 'approval',
      }}
    >
      { content }
    </DataListDisplay>
  );
}
