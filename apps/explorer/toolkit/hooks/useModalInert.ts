import React from 'react';

export default function useModalInert(content: HTMLElement | null, open: boolean) {
  React.useEffect(() => {
    if (!open || !content) return;

    const siblings = new Map<HTMLElement, boolean>();
    for (let branch: HTMLElement | null = content; branch?.parentElement; branch = branch.parentElement) {
      for (const sibling of branch.parentElement.children) {
        if (sibling !== branch && sibling instanceof HTMLElement && !sibling.matches(
          '[data-part=positioner]:is([data-scope=popover], [data-scope=menu], [data-scope=tooltip], [data-scope=select], [data-scope=combobox])',
        )) {
          siblings.set(sibling, sibling.inert);
          sibling.inert = true;
        }
      }
      if (branch.parentElement === document.body) break;
    }
    return () => {
      for (const [ sibling, inert ] of siblings) sibling.inert = inert;
    };
  }, [ content, open ]);
}
