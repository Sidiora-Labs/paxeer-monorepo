import React from 'react';

import useIsMobile from 'lib/hooks/useIsMobile';
import { Button } from 'toolkit/chakra/button';
import { DialogRoot, DialogContent, DialogTrigger, DialogHeader } from 'toolkit/chakra/dialog';
import { PopoverBody, PopoverContent, PopoverRoot, PopoverTrigger } from 'toolkit/chakra/popover';
import IconSvg from 'ui/shared/IconSvg';

interface Props {
  children: React.ReactNode;
  isLoading?: boolean;
}

const InfoButton = ({ children, isLoading }: Props) => {
  const isMobile = useIsMobile();

  const triggerButton = (
    <Button
      size="sm"
      variant="dropdown"
      gap={ 0 }
      aria-label="Show info"
      fontWeight="medium"
      pl={ 1 }
      pr={ isMobile ? 1 : 2 }
      loadingSkeleton={ isLoading }
    >
      <IconSvg name="info" boxSize={ 6 } mr={ isMobile ? 0 : 1 }/>
      { !isMobile && <span>Info</span> }
    </Button>
  );

  if (isMobile) {
    return (
      <DialogRoot size="full">
        <DialogTrigger>
          { triggerButton }
        </DialogTrigger>
        <DialogContent>
          <DialogHeader/>
          { children }
        </DialogContent>
      </DialogRoot>
    );
  }

  return (
    <PopoverRoot>
      <PopoverTrigger>
        { triggerButton }
      </PopoverTrigger>
      <PopoverContent w={{ base: 'calc(100vw - var(--chakra-spacing-8))', lg: 'var(--chakra-spacing-100)' }} maxW="100%">
        <PopoverBody>
          { children }
        </PopoverBody>
      </PopoverContent>
    </PopoverRoot>
  );
};

export default React.memo(InfoButton);
