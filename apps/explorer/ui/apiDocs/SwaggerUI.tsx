/* eslint-disable @typescript-eslint/naming-convention */
const SwaggerUIReact = dynamic(() => import('swagger-ui-react'), {
  loading: () => <ContentLoader/>,
  ssr: false,
});

import type { SystemStyleObject } from '@chakra-ui/react';
import { Box, useToken } from '@chakra-ui/react';
import dynamic from 'next/dynamic';
import React from 'react';

import type { SwaggerRequest } from './types';

import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';

import 'swagger-ui-react/swagger-ui.css';

const NeverShowInfoPlugin = () => {
  return {
    components: {
      SchemesContainer: () => null,
      ServersContainer: () => null,
      InfoContainer: () => null,
    },
  };
};

interface Props {
  url: string;
  requestInterceptor?: (request: SwaggerRequest) => SwaggerRequest;
}

const SwaggerUI = ({ url, requestInterceptor }: Props) => {
  const [ mainColor, borderColor, mainBgColor, raisedColor, accentColor, errorColor ] = useToken('colors', [
    'text.primary',
    'border.divider',
    'bg.surface',
    'bg.raised',
    'accent',
    'text.error',
  ]);

  const swaggerStyle: SystemStyleObject = {
    '& .swagger-ui .scheme-container, & .opblock-tag': {
      display: 'none',
    },
    '& .swagger-ui': {
      color: mainColor,
    },
    '& .swagger-ui .opblock-summary-control:focus': {
      outline: 'none',
    },
    // eslint-disable-next-line max-len
    '& .swagger-ui .opblock .opblock-summary-path, & .swagger-ui .opblock .opblock-summary-description, & .swagger-ui div, & .swagger-ui p, & .swagger-ui h5, & .swagger-ui .response-col_links, & .swagger-ui h4, & .swagger-ui table thead tr th, & .swagger-ui table thead tr td, & .swagger-ui .parameter__name, & .swagger-ui .parameter__type, & .swagger-ui .response-col_status, & .swagger-ui .tab li, & .swagger-ui .opblock .opblock-section-header h4': {
      color: 'unset',
    },
    '& .swagger-ui input': {
      color: mainColor,
    },
    '& .swagger-ui .opblock .opblock-section-header': {
      background: raisedColor,
    },
    '& .swagger-ui .response-col_description__inner p, & .swagger-ui .parameters-col_description p': {
      margin: 0,
    },
    '& .swagger-ui .wrapper': {
      padding: 0,
    },
    '& .swagger-ui .prop-type': {
      color: accentColor,
    },
    '& .swagger-ui .btn.try-out__btn': {
      borderColor: useToken('colors', 'link.primary'),
      color: useToken('colors', 'link.primary'),
      borderRadius: 'sm',
    },
    '& .swagger-ui .btn.try-out__btn:hover': {
      boxShadow: 'none',
      borderColor: useToken('colors', 'link.primary.hover'),
      color: useToken('colors', 'link.primary.hover'),
    },
    '& .swagger-ui .btn.try-out__btn.cancel': {
      borderColor: errorColor,
      color: errorColor,
    },
    '& .swagger-ui .btn.try-out__btn.cancel:hover': {
      borderColor: errorColor,
      color: errorColor,
    },

    // MODELS
    '& .swagger-ui section.models': {
      borderColor: borderColor,
    },
    '& .swagger-ui section.models h4': {
      color: mainColor,
    },
    '& .swagger-ui section.models .model-container': {
      bgColor: mainBgColor,
    },
    '& .swagger-ui .model-title': {
      wordBreak: 'break-all',
      color: mainColor,
    },
    '& .swagger-ui .model': {
      color: mainColor,
    },
    '& .swagger-ui .model-box-control:focus': {
      outline: 'none',
    },
    '& .swagger-ui .model-toggle': {
      bgColor: 'transparent',
      borderRadius: 'sm',
    },
    '& .swagger-ui .model .property.primitive': {
      color: useToken('colors', 'text.secondary'),
      wordBreak: 'break-all',
    },
  };

  return (
    <Box css={ swaggerStyle }>
      <SwaggerUIReact
        url={ url }
        plugins={ [ NeverShowInfoPlugin ] }
        requestInterceptor={ requestInterceptor }
      />
    </Box>
  );
};

export default SwaggerUI;
