// localStorage access that never throws (private mode, disabled storage, ...).

export const adminKey = (roomId: string) => `santa:admin:${roomId}`;
export const viewKey = (roomId: string) => `santa:view:${roomId}`;

export const storage = {
  get(key: string): string | null {
    try {
      return localStorage.getItem(key);
    } catch {
      return null;
    }
  },
  set(key: string, value: string): void {
    try {
      localStorage.setItem(key, value);
    } catch {
      // Storage unavailable: the app still works, just without persistence.
    }
  },
  remove(key: string): void {
    try {
      localStorage.removeItem(key);
    } catch {
      // ignore
    }
  },
};
