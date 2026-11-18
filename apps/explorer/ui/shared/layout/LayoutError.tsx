import React from 'react';

import type { Props } from './types';

import AppErrorBoundary from 'ui/shared/AppError/AppErrorBoundary';
import HeaderAlert from 'ui/snippets/header/HeaderAlert';
import HeaderMobile from 'ui/snippets/header/HeaderMobile';

import * as Layout from './components';

interface ErrorLayoutProps extends Props {
  mainLabel?: string;
}

const LayoutError = ({ children, mainLabel }: ErrorLayoutProps) => {
  return (
    <Layout.Root content={ children }>
      <Layout.Container>
        <Layout.NavBar/>
        <HeaderMobile/>
        <Layout.MainArea>
          <Layout.MainColumn>
            <HeaderAlert/>
            <AppErrorBoundary>
              <main aria-label={ mainLabel }>
                { children }
              </main>
            </AppErrorBoundary>
          </Layout.MainColumn>
        </Layout.MainArea>
        <Layout.Footer/>
      </Layout.Container>
    </Layout.Root>
  );
};

export default LayoutError;
