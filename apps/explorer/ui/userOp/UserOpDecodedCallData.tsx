import { Flex } from '@chakra-ui/react';
import React from 'react';

import type { UserOp } from 'types/api/userOps';

import useIsMobile from 'lib/hooks/useIsMobile';
import LogDecodedInputData from 'ui/shared/logs/LogDecodedInputData';
import * as DetailedInfo from 'ui/tx/ActivityDetails';

import UserOpCallDataSwitch from './UserOpCallDataSwitch';

interface Props {
  data: UserOp;
}

const UserOpDecodedCallData = ({ data }: Props) => {
  const isMobile = useIsMobile();
  const [ callData, setCallData ] = React.useState(data.decoded_execute_call_data || data.decoded_call_data);

  const handleSwitchChange = React.useCallback((isChecked: boolean) => {
    setCallData(isChecked ? data.decoded_call_data : data.decoded_execute_call_data);
  }, [ data ]);

  if (!callData) {
    return null;
  }

  const toggler = data.decoded_call_data && data.decoded_execute_call_data ? (
    <UserOpCallDataSwitch
      id="decoded-call-data-switch"
      onChange={ handleSwitchChange }
      initialValue={ false }
      ml={{ base: 0, lg: 'auto' }}
    />
  ) : null;

  const labelText = data.call_data && !data.execute_call_data ? 'Decoded external call data' : 'Decoded call data';

  return (
    <>
      <DetailedInfo.ItemLabel
        hint={ labelText }
      >
        <Flex alignItems="center" justifyContent="space-between">
          { labelText }
          { isMobile && toggler }
        </Flex>
      </DetailedInfo.ItemLabel>
      <DetailedInfo.ItemValue
        flexDir={{ base: 'column', lg: 'row' }}
        alignItems={{ base: 'flex-start', lg: 'center' }}
        flexWrap="wrap"
        mt={{ base: 'var(--chakra-spacing-1\\.25)', lg: 'var(--chakra-spacing-1)' }}
      >
        <LogDecodedInputData data={ callData } rightSlot={ !isMobile && toggler }/>
      </DetailedInfo.ItemValue>
    </>
  );
};

export default React.memo(UserOpDecodedCallData);
