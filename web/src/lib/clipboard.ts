/** Copies text to the clipboard; returns whether it succeeded. */
export async function copyText(text: string): Promise<boolean> {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Fall back to the legacy approach below.
    }
  }
  return legacyCopy(text);
}

function legacyCopy(text: string): boolean {
  const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const input = document.createElement('input');
  input.value = text;
  input.readOnly = true;
  input.setAttribute('aria-hidden', 'true');
  Object.assign(input.style, { position: 'fixed', top: '0', left: '0', opacity: '0' });
  document.body.appendChild(input);
  input.focus();
  input.select();
  input.setSelectionRange(0, text.length);

  let ok = false;
  try {
    ok = document.execCommand('copy');
  } catch {
    ok = false;
  }
  input.remove();
  previousFocus?.focus();
  return ok;
}
