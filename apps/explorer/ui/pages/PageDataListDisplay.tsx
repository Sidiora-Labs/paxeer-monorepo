import { Box } from '@chakra-ui/react';
import React from 'react';

import type { Props } from 'ui/shared/DataListDisplay';
import DataListDisplay from 'ui/shared/DataListDisplay';

const PageDataListDisplay = (props: Props) => {
  const ref = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    const root = ref.current;
    const actionBar = root?.querySelector<HTMLElement>('[data-action-bar]');
    if (!root || !actionBar) return;

    const updateHeight = () => {
      root.style.setProperty('--page-action-bar-height', `${ actionBar.getBoundingClientRect().height }px`);
    };
    const observer = new ResizeObserver(updateHeight);
    updateHeight();
    observer.observe(actionBar);
    return () => observer.disconnect();
  }, [ props.actionBar, props.itemsNum, props.isError ]);

  return (
    <Box ref={ ref } minW={ 0 } css={{ '& thead:not([data-scroll-region-header])': { top: 'var(--page-action-bar-height)' } }}>
      <DataListDisplay { ...props }/>
    </Box>
  );
};

export default PageDataListDisplay;
