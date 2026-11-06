import { Box } from '@chakra-ui/react';
import React from 'react';

import type { Props } from './types';

import AppErrorBoundary from 'ui/shared/AppError/AppErrorBoundary';
import HeaderAlert from 'ui/snippets/header/HeaderAlert';
import HeaderMobile from 'ui/snippets/header/HeaderMobile';

import * as Layout from './components';

const LayoutHome = ({ children }: Props) => {
  return (
    <Layout.Root content={ children }>
      <Layout.Container>
        <Layout.NavBar/>
        <HeaderMobile/>
        <Layout.MainArea>
          <Layout.SideBar/>
          <Layout.MainColumn
            position="relative"
            paddingTop={ 0 }
            maxW="1280px"
            mx="auto"
          >
            <Box
              data-label="hero-band"
              position="absolute"
              top={ 0 }
              left={ 0 }
              right={ 0 }
              height="100%"
              bgColor="bg.primary"
              pointerEvents="none"
              zIndex={ 0 }
            />
            <Box position="relative" zIndex={ 1 }>
              <HeaderAlert mb={ 3 }/>
              <AppErrorBoundary>
                { children }
              </AppErrorBoundary>
            </Box>
          </Layout.MainColumn>
        </Layout.MainArea>
        <Layout.Footer/>
      </Layout.Container>
    </Layout.Root>
  );
};

export default LayoutHome;
