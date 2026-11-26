export function observeContentWidth(element: HTMLElement, onWidth: (width: number) => void): ResizeObserver {
  // Read layout pixels before ResizeObserver can deliver the first size.
  const style = getComputedStyle(element)
  const width = parseFloat(style.width)
  if (Number.isFinite(width)) {
    const insets = style.boxSizing === 'border-box'
      ? parseFloat(style.paddingLeft) + parseFloat(style.paddingRight)
        + parseFloat(style.borderLeftWidth) + parseFloat(style.borderRightWidth)
      : 0
    onWidth(Math.max(0, width - insets))
  }

  const observer = new ResizeObserver(([entry]) => onWidth(entry.contentRect.width))
  observer.observe(element)
  return observer
}
