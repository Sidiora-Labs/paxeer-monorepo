import React from 'react';

import type { IconName } from 'ui/shared/IconSvg';
import IconSvg from 'ui/shared/IconSvg';

const ICONS: Record<string, IconName> = {
  '403': 'error-pages/403',
  '404': 'error-pages/404',
  '422': 'error-pages/422',
  '429': 'error-pages/429',
  '500': 'error-pages/500',
};

interface Props {
  statusCode: number | undefined;
}

const AppErrorIcon = ({ statusCode }: Props) => {
  return <IconSvg name={ ICONS[String(statusCode)] || ICONS['500'] } width="calc(var(--chakra-spacing-100) / 2)" height="calc(var(--chakra-spacing-100) / 4)" color="text.primary"/>;
};

export default AppErrorIcon;
