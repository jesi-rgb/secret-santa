<script lang="ts">
  import { tick } from 'svelte';
  import { createRoom, errorMessage, type RoomCreated } from './api';
  import CopyButton from './CopyButton.svelte';
  import {
    MAX_NAME_LENGTH,
    MAX_PARTICIPANTS,
    MAX_ROOM_NAME_LENGTH,
    MIN_PARTICIPANTS,
    nameLength,
    normalizeName,
  } from './names';
  import { linkClick } from './router';
  import { adminKey, storage } from './storage';

  type Row = { id: number; value: string };

  let nextId = 0;
  const newRow = (): Row => ({ id: nextId++, value: '' });
  const initialRows = () => Array.from({ length: MIN_PARTICIPANTS }, newRow);

  let roomName = $state('');
  let rows = $state<Row[]>(initialRows());
  let attempted = $state(false);
  let submitting = $state(false);
  let serverError = $state('');
  let created = $state<RoomCreated | null>(null);
  let listEl: HTMLOListElement | undefined = $state();

  /** Per-row error message ('' when valid). Blank rows are ignored on submit. */
  const rowErrors = $derived.by(() => {
    const counts = new Map<string, number>();
    for (const row of rows) {
      const key = normalizeName(row.value);
      if (key) counts.set(key, (counts.get(key) ?? 0) + 1);
    }
    return rows.map((row) => {
      const trimmed = row.value.trim();
      if (!trimmed) return '';
      if (nameLength(trimmed) > MAX_NAME_LENGTH) return `Demasiado largo (máx. ${MAX_NAME_LENGTH} caracteres).`;
      if ((counts.get(normalizeName(trimmed)) ?? 0) > 1) {
        return 'Nombre repetido (no se distinguen mayúsculas ni tildes).';
      }
      return '';
    });
  });

  const filledCount = $derived(rows.filter((row) => row.value.trim()).length);

  const countError = $derived(
    filledCount < MIN_PARTICIPANTS
      ? `Añade al menos ${MIN_PARTICIPANTS} nombres.`
      : filledCount > MAX_PARTICIPANTS
        ? `Como mucho ${MAX_PARTICIPANTS} nombres.`
        : '',
  );

  const roomNameError = $derived(
    nameLength(roomName.trim()) > MAX_ROOM_NAME_LENGTH
      ? `El nombre del sorteo es demasiado largo (máx. ${MAX_ROOM_NAME_LENGTH} caracteres).`
      : '',
  );

  const hasErrors = $derived(Boolean(countError || roomNameError || rowErrors.some(Boolean)));

  const shareUrl = $derived(created ? `${location.origin}/${created.roomId}` : '');
  const adminPath = $derived(created ? `/${created.roomId}/admin#${created.adminToken}` : '');
  const adminUrl = $derived(created ? `${location.origin}${adminPath}` : '');

  function focusRow(index: number) {
    listEl?.querySelectorAll<HTMLInputElement>('input')[index]?.focus();
  }

  async function addRow() {
    if (rows.length >= MAX_PARTICIPANTS) return;
    rows.push(newRow());
    await tick();
    focusRow(rows.length - 1);
  }

  function removeRow(id: number) {
    if (rows.length <= MIN_PARTICIPANTS) return;
    rows = rows.filter((row) => row.id !== id);
  }

  function onRowKeydown(event: KeyboardEvent, index: number) {
    if (event.key !== 'Enter' || event.isComposing) return;
    event.preventDefault();
    if (index === rows.length - 1) void addRow();
    else focusRow(index + 1);
  }

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    attempted = true;
    serverError = '';
    if (hasErrors || submitting) return;

    submitting = true;
    try {
      const participants = rows.map((row) => row.value.trim()).filter(Boolean);
      const result = await createRoom(participants, roomName.trim() || undefined);
      storage.set(adminKey(result.roomId), result.adminToken);
      created = result;
    } catch (err) {
      serverError = errorMessage(err);
    } finally {
      submitting = false;
    }
  }

  function startOver() {
    created = null;
    roomName = '';
    rows = initialRows();
    attempted = false;
    serverError = '';
  }

  function selectAll(event: FocusEvent & { currentTarget: HTMLInputElement }) {
    event.currentTarget.select();
  }
</script>

{#if created}
  <h1>¡Sorteo creado!</h1>

  <section>
    <h2>Comparte este enlace con el grupo</h2>
    <div class="row">
      <input readonly value={shareUrl} aria-label="Enlace para compartir" onfocus={selectAll} />
      <CopyButton text={shareUrl} />
    </div>
    <p>Cada persona lo abre, elige su nombre y descubre a quién le regala.</p>
  </section>

  <section>
    <h2>Tu enlace de organizador</h2>
    <div class="row">
      <input readonly value={adminUrl} aria-label="Enlace de organizador" onfocus={selectAll} />
      <CopyButton text={adminUrl} />
    </div>
    <p>
      <strong>No lo compartas.</strong> Con él puedes ver quién ha descubierto ya su nombre y reiniciar
      uno si alguien se equivoca. Nunca muestra quién regala a quién.
    </p>
    <p><a href={adminPath} onclick={linkClick}>Abrir la página de organizador</a></p>
  </section>

  <p><button type="button" onclick={startOver}>Crear otro sorteo</button></p>
{:else}
  <h1>Crea tu Secret Santa</h1>
  <p>
    Añade el nombre de todos. El sorteo se hace al momento y a nadie le puede tocar él mismo. Te
    daremos un enlace para compartir con el grupo.
  </p>

  <form onsubmit={submit} novalidate>
    <div class="field">
      <label for="room-name">Nombre del sorteo (opcional)</label>
      <input
        id="room-name"
        bind:value={roomName}
        placeholder="p. ej. Navidad en familia 2026"
        autocomplete="off"
        aria-invalid={roomNameError ? 'true' : undefined}
      />
      {#if roomNameError}<p class="error">{roomNameError}</p>{/if}
    </div>

    <fieldset class="field">
      <legend>Participantes ({filledCount})</legend>
      <ol class="participants" bind:this={listEl}>
        {#each rows as row, i (row.id)}
          <li>
            <div class="row">
              <input
                bind:value={row.value}
                aria-label={`Participante ${i + 1}`}
                aria-invalid={rowErrors[i] ? 'true' : undefined}
                autocomplete="off"
                onkeydown={(event) => onRowKeydown(event, i)}
              />
              <button
                type="button"
                onclick={() => removeRow(row.id)}
                disabled={rows.length <= MIN_PARTICIPANTS}
                aria-label={`Quitar participante ${i + 1}`}
              >
                Quitar
              </button>
            </div>
            {#if rowErrors[i]}<p class="error">{rowErrors[i]}</p>{/if}
          </li>
        {/each}
      </ol>
      <button type="button" onclick={addRow} disabled={rows.length >= MAX_PARTICIPANTS}>
        Añadir persona
      </button>
      {#if attempted && countError}<p class="error">{countError}</p>{/if}
    </fieldset>

    {#if serverError}<p class="error" role="alert">{serverError}</p>{/if}

    <button type="submit" disabled={submitting}>{submitting ? 'Creando…' : 'Crear sorteo'}</button>
  </form>
{/if}
