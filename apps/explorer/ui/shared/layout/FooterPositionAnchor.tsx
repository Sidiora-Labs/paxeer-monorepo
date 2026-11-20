import { Global } from '@emotion/react';
import React from 'react';

interface Props {
  children: React.ReactNode;
}

const FooterPositionAnchor = ({ children }: Props) => {
  return (
    <div data-footer-position-anchor style={{ display: 'contents' }}>
      <Global styles={{
        '[data-layout-viewport]:has([data-footer-position-anchor])': {
          overflowX: 'visible',
        },
        'body:has([data-footer-position-anchor]) :has(> footer) > :not(footer)': {
          overflowAnchor: 'none',
        },
      }}/>
      { children }
    </div>
  );
};

export default FooterPositionAnchor;
