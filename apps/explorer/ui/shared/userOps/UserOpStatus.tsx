import React from 'react';

import StatusTag from 'ui/shared/statusTag/StatusTag';

type Props = {
  status?: boolean;
  isLoading?: boolean;
};

const UserOpStatus = ({ status, isLoading }: Props) => {
  if (status === undefined) {
    return null;
  }

  return (
    <StatusTag role="status" loading={ isLoading } type={ status === true ? 'ok' : 'error' } text={ status === true ? 'Success' : 'Failed' }/>
  );
};

export default UserOpStatus;
