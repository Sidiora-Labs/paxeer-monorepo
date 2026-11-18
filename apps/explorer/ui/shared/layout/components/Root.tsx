import React from 'react';

import useIsMounted from 'lib/hooks/useIsMounted';

interface Props {
  children: React.ReactNode;
  content: React.ReactNode;
}

const Root = ({ children }: Props) => {
  const isMounted = useIsMounted();

  return isMounted ? children : null;
};

export default React.memo(Root);
