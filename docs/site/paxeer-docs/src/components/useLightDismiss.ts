'use client'

import { useEffect, type RefObject } from 'react'

export function useLightDismiss(ref: RefObject<HTMLDialogElement>) {
  useEffect(() => {
    const dialog = ref.current
    if (!dialog) return
    dialog.setAttribute('closedby', 'any')
    if (Reflect.has(dialog, 'closedBy')) return
    let startedOutside = false
    const outside = (event: PointerEvent) => {
      const bounds = dialog.getBoundingClientRect()
      return event.target === dialog && (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom)
    }
    const down = (event: PointerEvent) => { startedOutside = outside(event) }
    const up = (event: PointerEvent) => {
      if (startedOutside && outside(event) && dialog.open) dialog.close()
      startedOutside = false
    }
    dialog.addEventListener('pointerdown', down)
    dialog.addEventListener('pointerup', up)
    return () => {
      dialog.removeEventListener('pointerdown', down)
      dialog.removeEventListener('pointerup', up)
    }
  }, [ref])
}
