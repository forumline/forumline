/*
 * DOM utility helpers
 *
 * This file provides lightweight helpers for creating elements, attaching events, and toggling classes without a framework.
 *
 * It must:
 * - Create HTML elements with attributes and children in a single call (h function)
 * - Bind event listeners with automatic "on" prefix handling and return a cleanup function
 * - Toggle CSS classes on an element based on a boolean record, supporting space-separated class strings
 */
/** Create an HTML element with optional attributes and children. */
export function h<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  attrs?: Record<string, string | boolean | number | EventListener | null | undefined>,
  ...children: (Node | string)[]
): HTMLElementTagNameMap[K] {
  const el = document.createElement(tag)

  if (attrs) {
    for (const [key, val] of Object.entries(attrs)) {
      if (val == null || val === false) continue
      if (key.startsWith('on') && typeof val === 'function') {
        el.addEventListener(key.slice(2).toLowerCase(), val as EventListener)
      } else if (val === true) {
        el.setAttribute(key, '')
      } else {
        el.setAttribute(key, String(val))
      }
    }
  }

  for (const child of children) {
    el.append(typeof child === 'string' ? document.createTextNode(child) : child)
  }

  return el
}

/** Add an event listener, return a cleanup function. */
export function on<K extends keyof HTMLElementEventMap>(
  el: EventTarget,
  event: K,
  handler: (e: HTMLElementEventMap[K]) => void,
  options?: AddEventListenerOptions,
): () => void {
  el.addEventListener(event, handler as EventListener, options)
  return () => el.removeEventListener(event, handler as EventListener, options)
}

/** Set classes on an element. Accepts a record of class → boolean. */
export function cls(el: HTMLElement, classes: Record<string, boolean>) {
  for (const [name, active] of Object.entries(classes)) {
    if (name.includes(' ')) {
      // Handle space-separated classes
      for (const c of name.split(' ').filter(Boolean)) {
        el.classList.toggle(c, active)
      }
    } else {
      el.classList.toggle(name, active)
    }
  }
}
