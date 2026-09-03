export type OverlayKeyHandler = (event: KeyboardEvent) => void

let lockCount = 0
let previousOverflow = ''
const overlayStack: OverlayKeyHandler[] = []
let keyListenerBound = false

function onWindowKeydown(event: KeyboardEvent) {
  const top = overlayStack[overlayStack.length - 1]
  if (!top) return
  top(event)
  if (event.key === 'Escape') event.stopImmediatePropagation()
}

function bindKeyListener() {
  if (keyListenerBound || typeof window === 'undefined') return
  window.addEventListener('keydown', onWindowKeydown)
  keyListenerBound = true
}

function unbindKeyListener() {
  if (!keyListenerBound || typeof window === 'undefined') return
  window.removeEventListener('keydown', onWindowKeydown)
  keyListenerBound = false
}

export function acquireOverlayLock(): void {
  if (typeof document === 'undefined') return
  if (lockCount === 0) {
    previousOverflow = document.body.style.overflow
    document.body.classList.add('modal-open')
    document.body.style.overflow = 'hidden'
  }
  lockCount += 1
}

export function releaseOverlayLock(): void {
  if (typeof document === 'undefined') return
  if (lockCount === 0) return
  lockCount -= 1
  if (lockCount > 0) return
  document.body.classList.remove('modal-open')
  document.body.style.overflow = previousOverflow
  previousOverflow = ''
}

export function pushOverlay(handler: OverlayKeyHandler): void {
  overlayStack.push(handler)
  bindKeyListener()
}

export function popOverlay(handler: OverlayKeyHandler): void {
  const index = overlayStack.lastIndexOf(handler)
  if (index >= 0) overlayStack.splice(index, 1)
  if (overlayStack.length === 0) unbindKeyListener()
}

export function resetOverlayLock(): void {
  lockCount = 0
  previousOverflow = ''
  overlayStack.length = 0
  unbindKeyListener()
  if (typeof document === 'undefined') return
  document.body.classList.remove('modal-open')
  document.body.style.overflow = ''
}
