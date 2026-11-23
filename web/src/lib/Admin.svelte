<script lang="ts">
  import { onMount } from 'svelte';
  import {
    errorMessage,
    getAdmin,
    isApiError,
    resetParticipant,
    type AdminParticipant,
    type AdminRoom,
  } from './api';
  import CopyButton from './CopyButton.svelte';
  import { adminKey, storage } from './storage';

  let { roomId }: { roomId: string } = $props();

  let token = '';
  let status = $state<'loading' | 'ready' | 'no_token' | 'invalid' | 'not_found' | 'error'>('loading');
  let loadError = $state('');
  let actionError = $state('');
  let data = $state<AdminRoom | null>(null);
  let refreshing = $state(false);
  let resettingId = $state<number | null>(null);

  const shareUrl = $derived(`${location.origin}/${roomId}`);
  const revealedCount = $derived(data ? data.participants.filter((p) => p.revealed).length : 0);

  onMount(() => {
    const fromHash = location.hash.replace(/^#/, '');
    if (fromHash) {
      token = fromHash;
      storage.set(adminKey(roomId), token);
      history.replaceState(history.state, '', location.pathname + location.search);
    } else {
      token = storage.get(adminKey(roomId)) ?? '';
    }

    if (!token) {
      status = 'no_token';
      return;
    }
    void load();
  });

  /** Maps auth/room errors to page states; returns true if handled. */
  function handleFatal(err: unknown): boolean {
    if (isApiError(err) && err.status === 401) {
      storage.remove(adminKey(roomId));
      status = 'invalid';
      return true;
    }
    if (isApiError(err, 'room_not_found')) {
      status = 'not_found';
      return true;
    }
    return false;
  }

  async function load() {
    refreshing = true;
    try {
      data = await getAdmin(roomId, token);
      actionError = '';
      status = 'ready';
    } catch (err) {
      if (handleFatal(err)) return;
      if (data) {
        actionError = errorMessage(err);
      } else {
        loadError = errorMessage(err);
        status = 'error';
      }
    } finally {
      refreshing = false;
    }
  }

  async function reset(participant: AdminParticipant) {
    const ok = confirm(
      `¿Reiniciar a “${participant.name}”?\n\nPodrá volver a descubrir a quién le regala (desde cualquier dispositivo). El sorteo no cambia.`,
    );
    if (!ok) return;

    resettingId = participant.id;
    actionError = '';
    try {
      await resetParticipant(roomId, token, participant.id);
      await load();
    } catch (err) {
      if (!handleFatal(err)) actionError = errorMessage(err);
    } finally {
      resettingId = null;
    }
  }

  function formatTime(iso: string): string {
    const date = new Date(iso);
    return Number.isNaN(date.getTime()) ? iso : date.toLocaleString('es');
  }

  function selectAll(event: FocusEvent & { currentTarget: HTMLInputElement }) {
    event.currentTarget.select();
  }
</script>

{#if status === 'loading'}
  <p>Cargando…</p>
{:else if status === 'no_token'}
  <h1>Necesitas el enlace de organizador</h1>
  <p>Abre el enlace de organizador completo que recibiste al crear el sorteo.</p>
{:else if status === 'invalid'}
  <h1>Enlace de organizador no válido</h1>
  <p>Asegúrate de haber copiado entero el enlace de organizador que recibiste al crear el sorteo.</p>
{:else if status === 'not_found'}
  <h1>Sorteo no encontrado</h1>
  <p>Comprueba que has abierto el enlace completo.</p>
{:else if status === 'error'}
  <h1>Algo ha fallado</h1>
  <p class="error">{loadError}</p>
  <p><button type="button" onclick={load} disabled={refreshing}>Reintentar</button></p>
{:else if data}
  <h1>{data.name || 'Amigo invisible'} (organizador)</h1>
  <p class="muted">Creado el {formatTime(data.createdAt)}. Esta página nunca muestra quién regala a quién.</p>

  <section>
    <h2>Enlace para compartir</h2>
    <div class="row">
      <input readonly value={shareUrl} aria-label="Enlace para compartir" onfocus={selectAll} />
      <CopyButton text={shareUrl} />
    </div>
  </section>

  <section>
    <h2>Participantes ({revealedCount}/{data.participants.length} descubiertos)</h2>
    <p>
      <button type="button" onclick={load} disabled={refreshing}>
        {refreshing ? 'Actualizando…' : 'Actualizar'}
      </button>
    </p>
    {#if actionError}<p class="error" role="alert">{actionError}</p>{/if}

    <table>
      <thead>
        <tr>
          <th scope="col">Nombre</th>
          <th scope="col">Descubierto</th>
          <th scope="col">Fecha</th>
          <th scope="col"><span class="muted">Acciones</span></th>
        </tr>
      </thead>
      <tbody>
        {#each data.participants as participant (participant.id)}
          <tr>
            <td>{participant.name}</td>
            <td>{participant.revealed ? '✓' : '—'}</td>
            <td>{participant.revealedAt ? formatTime(participant.revealedAt) : ''}</td>
            <td>
              {#if participant.revealed}
                <button
                  type="button"
                  onclick={() => reset(participant)}
                  disabled={resettingId !== null}
                >
                  {resettingId === participant.id ? 'Reiniciando…' : 'Reiniciar'}
                </button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
    <p class="muted">
      Reinicia un nombre si alguien descubrió uno que no era el suyo: esa persona podrá volver a
      descubrirlo. El sorteo no cambia.
    </p>
  </section>
{/if}
