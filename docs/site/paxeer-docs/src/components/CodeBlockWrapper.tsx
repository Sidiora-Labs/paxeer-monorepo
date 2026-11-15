'use client'

import { Children, cloneElement, isValidElement, useId, type ReactElement, type ReactNode } from 'react'
import { CopyButton } from './CopyButton'

function plainText(node: ReactNode): string {
  if (typeof node === 'string' || typeof node === 'number') return String(node)
  if (isValidElement<{ children?: ReactNode }>(node)) return plainText(node.props.children)
  return Children.toArray(node).map(plainText).join('')
}

export function CodeBlockWrapper({ children, label }: { children: ReactNode; label: string }): ReactNode {
  return Children.map(children, child => {
    if (!isValidElement<{ children?: ReactNode; scope?: string; 'data-copy-managed'?: string }>(child) || typeof child.type !== 'string') return child
    if (child.type === 'pre') {
      if (child.props['data-copy-managed']) return child
      return <ManagedCodeBlock element={child} label={label} />
    }
    if (child.type === 'table') return <div className="table-scroll" role="region" aria-label={`${label} table`} tabIndex={0}>{cloneElement(child, {}, <CodeBlockWrapper label={label}>{child.props.children}</CodeBlockWrapper>)}</div>
    if (child.type === 'th') return cloneElement(child, { scope: child.props.scope || 'col' }, child.props.children)
    if (child.type === 'td' && /^[-+]?\d[\d,.%\s]*$/.test(plainText(child.props.children).trim())) return cloneElement(child, { 'data-numeric': true } as React.HTMLAttributes<HTMLElement>, child.props.children)
    if (child.props.children == null) return child
    return cloneElement(child, {}, <CodeBlockWrapper label={label}>{child.props.children}</CodeBlockWrapper>)
  })
}

function ManagedCodeBlock({ element, label }: { element: ReactElement<{ children?: ReactNode; id?: string }>; label: string }) {
  const generated = useId()
  const targetId = element.props.id || 'code-' + generated
  return <div className="code-block-frame"><div className="code-block-tools"><CopyButton targetId={targetId} resetKey={plainText(element.props.children)} /></div>{cloneElement(element, { id: targetId, tabIndex: 0, role: 'region', 'aria-label': `${label} code example` } as React.HTMLAttributes<HTMLElement>)}</div>
}
