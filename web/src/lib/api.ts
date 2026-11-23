// Typed wrappers around the backend JSON API.

import { MAX_PARTICIPANTS, MIN_PARTICIPANTS } from './names';

export interface RoomCreated {
  roomId: string;
  adminToken: string;
}

export interface RoomInfo {
  roomId: string;
  name: string;
  createdAt: string;
  participantCount: number;
  revealedCount: number;
  /** Sorted by name. Never includes the draw. */
  participants: { name: string; revealed: boolean }[];
}

export interface RevealResult {
  /** Canonical participant name as entered by the organizer. */
  name: string;
  giftee: string;
  /** Four emojis. */
  code: string[];
  /** The four emojis concatenated. */
  codeText: string;
  viewToken: string;
}

export interface AdminParticipant {
  id: number;
  name: string;
  revealed: boolean;
  revealedAt: string | null;
}

export interface AdminRoom {
  roomId: string;
  name: string;
  createdAt: string;
  participants: AdminParticipant[];
}

/** Error returned by the API (or a network failure, with status 0). */
export class ApiError extends Error {
  readonly status: number;
  /** Machine-readable error code from the response body, e.g. `room_not_found`. */
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

export function isApiError(err: unknown, code?: string): err is ApiError {
  return err instanceof ApiError && (code === undefined || err.code === code);
}

const GENERIC_ERROR = 'Algo ha fallado. Inténtalo de nuevo.';

// The API's own messages are English; the UI shows these, keyed by error code.
const ERROR_MESSAGES: Record<string, string> = {
  network_error: 'No se ha podido conectar con el servidor. Revisa tu conexión e inténtalo de nuevo.',
  room_not_found: 'Este sorteo no existe.',
  name_not_found: 'Ese nombre no está en este sorteo. Revisa cómo está escrito.',
  already_revealed: 'Este nombre ya se ha descubierto.',
  participant_not_found: 'Esa persona no está en este sorteo.',
  invalid_token: 'Este enlace ya no es válido.',
  invalid_name: 'Algún nombre no es válido. Revisa la lista.',
  invalid_participants: `Un sorteo necesita entre ${MIN_PARTICIPANTS} y ${MAX_PARTICIPANTS} participantes.`,
  duplicate_name: 'Hay nombres repetidos (no se distinguen mayúsculas ni tildes).',
  request_too_large: 'Hay demasiados datos. Prueba con menos nombres o nombres más cortos.',
};

export function errorMessage(err: unknown): string {
  return (err instanceof ApiError && ERROR_MESSAGES[err.code]) || GENERIC_ERROR;
}

interface RequestOptions {
  method?: 'GET' | 'POST';
  body?: unknown;
  token?: string;
}

async function request<T>(path: string, { method = 'GET', body, token }: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (token !== undefined) headers.Authorization = `Bearer ${token}`;

  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, 'network_error', 'Could not reach the server. Check your connection and try again.');
  }

  let data: unknown = null;
  const text = await res.text();
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = null;
    }
  }

  if (!res.ok) {
    const payload = (data ?? {}) as { error?: unknown; message?: unknown };
    const code = typeof payload.error === 'string' ? payload.error : `http_${res.status}`;
    const message =
      typeof payload.message === 'string' && payload.message
        ? payload.message
        : `Request failed (${res.status} ${res.statusText})`;
    throw new ApiError(res.status, code, message);
  }

  return data as T;
}

const room = (roomId: string) => `/api/rooms/${encodeURIComponent(roomId)}`;

export function createRoom(participants: string[], name?: string): Promise<RoomCreated> {
  return request<RoomCreated>('/api/rooms', {
    method: 'POST',
    body: name ? { name, participants } : { participants },
  });
}

export function getRoom(roomId: string): Promise<RoomInfo> {
  return request<RoomInfo>(room(roomId));
}

export function reveal(roomId: string, name: string): Promise<RevealResult> {
  return request<RevealResult>(`${room(roomId)}/reveal`, { method: 'POST', body: { name } });
}

export function me(roomId: string, viewToken: string): Promise<RevealResult> {
  return request<RevealResult>(`${room(roomId)}/me`, { method: 'POST', body: { viewToken } });
}

export function getAdmin(roomId: string, adminToken: string): Promise<AdminRoom> {
  return request<AdminRoom>(`${room(roomId)}/admin`, { token: adminToken });
}

export function resetParticipant(roomId: string, adminToken: string, participantId: number): Promise<{ ok: true }> {
  return request<{ ok: true }>(`${room(roomId)}/admin/reset`, {
    method: 'POST',
    token: adminToken,
    body: { participantId },
  });
}
