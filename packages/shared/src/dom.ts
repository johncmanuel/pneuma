// Portal action: moves the node to document.body so it is never clipped
// by any ancestor overflow or contain property.
export function portal(node: HTMLElement) {
  document.body.appendChild(node);
  return {
    destroy() {
      node.remove();
    }
  };
}
