export const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])'

function isVisible(el: HTMLElement): boolean {
  if (el instanceof HTMLInputElement && el.type === 'hidden') return false
  if (el.hidden || el.closest('[hidden]')) return false
  let node: HTMLElement | null = el
  while (node && node !== document.documentElement) {
    const style = window.getComputedStyle(node)
    if (style.display === 'none' || style.visibility === 'hidden') return false
    node = node.parentElement
  }
  return true
}

function focusRoot(root: HTMLElement) {
  if (!root.hasAttribute('tabindex')) root.setAttribute('tabindex', '-1')
  root.focus()
}

export function getFocusable(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
    (el) => !el.hasAttribute('disabled') && isVisible(el)
  )
}

export function trapFocus(event: KeyboardEvent, root: HTMLElement | null) {
  if (event.key !== 'Tab' || !root) return
  const nodes = getFocusable(root)
  if (nodes.length === 0) {
    event.preventDefault()
    focusRoot(root)
    return
  }

  const first = nodes[0]
  const last = nodes[nodes.length - 1]
  const active = document.activeElement
  const index = active instanceof HTMLElement ? nodes.indexOf(active) : -1
  const outside = !(active instanceof Node) || !root.contains(active)

  if (event.shiftKey) {
    if (outside || index <= 0) {
      event.preventDefault()
      last.focus()
    }
    return
  }

  if (outside || index === -1 || index === nodes.length - 1) {
    event.preventDefault()
    first.focus()
  }
}

export function focusFirst(root: HTMLElement | null) {
  if (!root) return
  const first = getFocusable(root)[0]
  if (first) {
    first.focus()
    return
  }
  focusRoot(root)
}
