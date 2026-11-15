'use client'

import { CopyControl } from '@centra/docs-assistant/react'

export function CopyButton({ targetId, resetKey }: { targetId: string; resetKey: string }) {
  return <CopyControl targetId={targetId} resetKey={resetKey} label="Copy code" className="copy-button" />
}
