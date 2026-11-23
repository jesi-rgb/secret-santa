// Minimal pathname-based router (no dependency).

export type Route =
  | { kind: 'create' }
  | { kind: 'reveal'; roomId: string }
  | { kind: 'admin'; roomId: string }
  | { kind: 'not_found' };

const REVEAL = /^\/([a-z0-9]{10})\/?$/;
const ADMIN = /^\/([a-z0-9]{10})\/admin\/?$/;

export function matchRoute(pathname: string): Route {
  if (pathname === '/' || pathname === '') return { kind: 'create' };
  let m = REVEAL.exec(pathname);
  if (m) return { kind: 'reveal', roomId: m[1] };
  m = ADMIN.exec(pathname);
  if (m) return { kind: 'admin', roomId: m[1] };
  return { kind: 'not_found' };
}

/** Pushes a new history entry and notifies the app (via popstate). */
export function navigate(url: string): void {
  history.pushState(null, '', url);
  window.dispatchEvent(new PopStateEvent('popstate'));
  window.scrollTo(0, 0);
}

/** Click handler for same-origin `<a>` links: navigates without a full page load. */
export function linkClick(event: MouseEvent & { currentTarget: HTMLAnchorElement }): void {
  const a = event.currentTarget;
  if (
    event.defaultPrevented ||
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey ||
    (a.target && a.target !== '_self') ||
    a.origin !== location.origin
  ) {
    return;
  }
  event.preventDefault();
  navigate(a.pathname + a.search + a.hash);
}
