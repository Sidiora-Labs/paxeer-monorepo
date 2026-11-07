import { Box } from '@chakra-ui/react';
import { debounce } from 'es-toolkit';
import React, { useRef, useEffect, useState, useCallback } from 'react';

const CUT_HEIGHT = 144;

const AccountPageDescription = ({ children, allowCut = true }: { children: React.ReactNode; allowCut?: boolean }) => {
  const ref = useRef<HTMLParagraphElement>(null);

  const [ needCut, setNeedCut ] = useState(false);
  const [ expanded, setExpanded ] = useState(false);

  const calculateCut = useCallback(() => {
    const textHeight = ref.current?.offsetHeight;
    if (!needCut && textHeight && textHeight > CUT_HEIGHT) {
      setNeedCut(true);
    } else if (needCut && textHeight && textHeight < CUT_HEIGHT) {
      setNeedCut(false);
    }
  }, [ needCut ]);

  useEffect(() => {
    if (!allowCut) {
      return;
    }

    calculateCut();
    const resizeHandler = debounce(calculateCut, 300);
    window.addEventListener('resize', resizeHandler);
    return function cleanup() {
      window.removeEventListener('resize', resizeHandler);
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ ]);

  const expand = useCallback(() => {
    setExpanded(true);
  }, []);

  return (
    <Box position="relative" marginBottom={{ base: 6, lg: 8 }}>
      <Box
        ref={ ref }
        maxHeight={ needCut && !expanded ? `${ CUT_HEIGHT }px` : 'auto' }
        overflow="hidden"
        style={ needCut && !expanded ? { WebkitLineClamp: '6', WebkitBoxOrient: 'vertical', display: '-webkit-box' } : {} }
      >
        { children }
      </Box>
      { needCut && !expanded && (
        <Box
          position="absolute"
          bottom="-16px"
          left={ 0 }
          width="100%"
          height="63px"
          background="linear-gradient(360deg, var(--chakra-colors-description-fade) 38.1%, transparent 166.67%)"
          onClick={ expand }
        >
        </Box>
      ) }
    </Box>
  );
};

export default AccountPageDescription;
