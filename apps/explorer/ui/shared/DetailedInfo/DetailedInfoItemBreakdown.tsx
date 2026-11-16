import type { JsxStyleProps } from '@chakra-ui/react';
import { Grid, GridItem } from '@chakra-ui/react';
import React from 'react';

import type { CollapsibleDetailsProps } from 'toolkit/chakra/collapsible';
import { CollapsibleDetails } from 'toolkit/chakra/collapsible';
import { Hint } from 'toolkit/components/Hint/Hint';
import { TruncatedText } from 'toolkit/components/truncation/TruncatedText';

interface ContainerProps extends CollapsibleDetailsProps {}

export const Container = ({ children, ...rest }: ContainerProps) => {
  return (
    <CollapsibleDetails noScroll variant="secondary" display="block" textStyle={ undefined } fontSize="sm" { ...rest }>
      <Grid
        gridTemplateColumns="fit-content(40%) minmax(0, 1fr)"
        alignItems="start"
        textStyle="sm"
        bgColor={{ _light: 'blackAlpha.50', _dark: 'whiteAlpha.50' }}
        w="100%"
        p={{ base: 2, lg: 3 }}
        mt={ 1 }
        columnGap={ 3 }
        rowGap={ 4 }
        borderBottomRightRadius="base"
        borderBottomLeftRadius="base"
      >
        { children }
      </Grid>
    </CollapsibleDetails>
  );
};

interface RowProps extends JsxStyleProps {
  label: string;
  hint?: string;
  children: React.ReactNode;
}

export const Row = ({ label, hint, children, ...rest }: RowProps) => {
  return (
    <>
      <GridItem color="text.secondary" display="flex" alignItems="center" { ...rest }>
        { hint && <Hint label={ hint } boxSize={ 4 } mr={ 1 }/> }
        <TruncatedText text={ label } maxW="100%"/>
      </GridItem>
      { children }
    </>
  );
};
