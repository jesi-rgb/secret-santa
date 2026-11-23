<script lang="ts">
  import { onMount } from 'svelte';
  import {
    errorMessage,
    getRoom,
    isApiError,
    me,
    reveal,
    type RevealResult,
    type RoomInfo,
  } from './api';
  import Combobox from './Combobox.svelte';
  import CopyButton from './CopyButton.svelte';
  import { normalizeName } from './names';
  import { storage, viewKey } from './storage';

  let { roomId }: { roomId: string } = $props();

  let status = $state<'loading' | 'ready' | 'not_found' | 'error'>('loading');
  let loadError = $state('');
  let room = $state<RoomInfo | null>(null);
  let result = $state<RevealResult | null>(null);
  let name = $state('');
  let formError = $state('');
  let submitting = $state(false);

  onMount(() => {
    void load();
  });

  async function load() {
    status = 'loading';
    try {
      room = await getRoom(roomId);
    } catch (err) {
      if (isApiError(err, 'room_not_found')) {
        status = 'not_found';
      } else {
        loadError = errorMessage(err);
        status = 'error';
      }
      return;
    }

    const token = storage.get(viewKey(roomId));
    if (token) {
      try {
        result = await me(roomId, token);
      } catch (err) {
        if (isApiError(err, 'room_not_found')) {
          status = 'not_found';
          return;
        }
        if (isApiError(err) && err.status === 401) {
          storage.remove(viewKey(roomId));
        } else {
          formError = errorMessage(err);
        }
      }
    }
    status = 'ready';
  }

  async function refreshRoom() {
    try {
      room = await getRoom(roomId);
    } catch {
      // Counts are informational; keep the previous ones.
    }
  }

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    formError = '';
    const key = normalizeName(name);
    if (!key) {
      formError = 'Elige tu nombre.';
      return;
    }
    const participant = room?.participants.find((p) => normalizeName(p.name) === key);
    if (!participant) {
      formError = 'Ese nombre no está en este sorteo. Elígelo de la lista.';
      return;
    }
    if (participant.revealed) {
      formError = 'Este nombre ya se ha descubierto. Si no has sido tú, pide al organizador que lo reinicie.';
      return;
    }
    if (submitting) return;

    submitting = true;
    try {
      const res = await reveal(roomId, participant.name);
      storage.set(viewKey(roomId), res.viewToken);
      result = res;
      void refreshRoom();
    } catch (err) {
      if (isApiError(err, 'name_not_found')) {
        formError = 'Ese nombre no está en este sorteo. Revisa cómo está escrito.';
      } else if (isApiError(err, 'already_revealed')) {
        formError = 'Este nombre ya se ha descubierto. Si no has sido tú, pide al organizador que lo reinicie.';
        name = '';
        void refreshRoom();
      } else if (isApiError(err, 'room_not_found')) {
        status = 'not_found';
      } else {
        formError = errorMessage(err);
      }
    } finally {
      submitting = false;
    }
  }
</script>

{#if status === 'loading'}
  <p>Cargando…</p>
{:else if status === 'not_found'}
  <h1>Sorteo no encontrado</h1>
  <p>Comprueba que has abierto el enlace completo que te enviaron.</p>
{:else if status === 'error'}
  <h1>Algo ha fallado</h1>
  <p class="error">{loadError}</p>
  <p><button type="button" onclick={load}>Reintentar</button></p>
{:else if room}
  <h1>{room.name || 'Secret Santa'}</h1>
  <p class="muted">{room.revealedCount}/{room.participantCount} descubiertos</p>

  {#if result}
    <section>
      <div class="gift-tag">
        <p>Hola {result.name}, te toca regalarle a</p>
        <strong class="giftee">{result.giftee}</strong>
      </div>

      <h2>Tu código de comprobación</h2>
      <p class="code">
        {#each result.code as emoji}<span>{emoji}</span>{/each}
      </p>
      <CopyButton text={result.codeText} label="Copiar código" />
      <p>
        Copia y pega este código en el grupete de Whatsapp. 
        Todos deberíamos tener un código distinto. Si no es el caso, probablemente dos personas tienen el mismo destinatario.
      </p>
    </section>
  {:else}
    <form onsubmit={submit} novalidate>
      <div class="field">
        <label for="your-name">Tu nombre</label>
        <div class="row">
          <Combobox
            id="your-name"
            label="Nombres"
            bind:value={name}
            placeholder="Empieza a escribir tu nombre…"
            invalid={Boolean(formError)}
            options={room.participants.map((p) => ({
              value: p.name,
              disabled: p.revealed,
              hint: p.revealed ? 'Ya descubierto' : undefined,
            }))}
          />
          <button type="submit" disabled={submitting}>{submitting ? 'Descubriendo…' : 'Descubrir'}</button>
        </div>
        {#if formError}<p class="error" role="alert">{formError}</p>{/if}
      </div>
    </form>
    <p class="muted">
      Elige tu propio nombre. Cada nombre solo se puede descubrir una vez, así que asegúrate de que es el tuyo.
    </p>
  {/if}
{/if}
