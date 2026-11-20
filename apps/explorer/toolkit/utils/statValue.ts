import type { SystemStyleObject } from '@chakra-ui/react';
import React from 'react';

import { fontSizes } from 'toolkit/theme/foundations/typography';

const steps = [ 'textMd', 'headingMd', 'textXl', 'statValue' ] as const;

export function getStatValueText(value: React.ReactNode): string {
  return React.Children.toArray(value).map((child) => {
    if (typeof child === 'string' || typeof child === 'number') return String(child);
    return React.isValidElement<{ children?: React.ReactNode }>(child) ? getStatValueText(child.props.children) : '';
  }).join('');
}

export function useStatValue(value: React.ReactNode) {
  const ref = React.useRef<HTMLDivElement>(null);
  const [ text, setText ] = React.useState(() => getStatValueText(value));

  React.useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return;
    const update = () => setText(element.textContent ?? '');
    update();
    const observer = new MutationObserver(update);
    observer.observe(element, { childList: true, characterData: true, subtree: true });
    return () => observer.disconnect();
  }, [ value ]);

  return { ref, characters: Array.from(text).length };
}

export function getStatValueStyles(characters: number): SystemStyleObject {
  return {
    fontSize: 'body',
    ...Object.fromEntries(steps.map((step) => {
      const scale = parseFloat(fontSizes[step].value) / parseFloat(fontSizes.textMd.value);
      return [ `@container (min-width: ${ characters * scale }ch)`, { fontSize: step } ];
    })),
  };
}
