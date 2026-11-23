// Client-side mirror of the backend's participant validation rules.

export const MIN_PARTICIPANTS = 3;
export const MAX_PARTICIPANTS = 100;
export const MAX_NAME_LENGTH = 40;
export const MAX_ROOM_NAME_LENGTH = 60;

/** NFD, strip combining marks, lowercase, collapse whitespace, trim. */
export function normalizeName(name: string): string {
  return name
    .normalize('NFD')
    .replace(/\p{M}/gu, '')
    .toLowerCase()
    .replace(/\s+/g, ' ')
    .trim();
}

/** Length in Unicode code points (what the Go backend counts). */
export function nameLength(name: string): number {
  return [...name].length;
}
