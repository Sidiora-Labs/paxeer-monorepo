import { chakra } from '@chakra-ui/react';
import React from 'react';

import type { TokenInfoApplication } from 'types/api/account';

interface Props {
  status?: TokenInfoApplication['status'];
}

const VerifiedAddressesStatus = ({ status }: Props) => {
  switch (status) {
    case 'IN_PROCESS': {
      return <chakra.span fontWeight="medium">In progress</chakra.span>;
    }
    case 'APPROVED': {
      return <chakra.span fontWeight="medium" color="text.success">Approved</chakra.span>;
    }
    case 'UPDATE_REQUIRED': {
      return <chakra.span fontWeight="medium" color="text.warning">Waiting for update</chakra.span>;
    }
    case 'REJECTED': {
      return <chakra.span fontWeight="medium" color="text.error">Rejected</chakra.span>;
    }

    default:
      return null;
  }
};

export default VerifiedAddressesStatus;
