<script lang="ts">
  import { tick } from 'svelte';
  import { createRoom, errorMessage, type RoomCreated } from './api';
  import CopyButton from './CopyButton.svelte';
  import { canDraw } from './draw';
  import Exclusions, { type ExclusionPair } from './Exclusions.svelte';
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
  let step = $state<'names' | 'exclusions'>('names');
  let exclusions = $state<ExclusionPair[]>([]);
  let listEl: HTMLOListElement | undefined = $state();
  let headingEl: HTMLHeadingElement | undefined = $state();

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

  const people = $derived(
    rows.flatMap((row) => {
      const name = row.value.trim();
      return name ? [{ id: row.id, name }] : [];
    }),
  );

  const impossible = $derived.by(() => {
    const index = new Map(people.map((person, i) => [person.id, i]));
    const pairs = exclusions.flatMap(({ a, b }) => {
      const i = index.get(a);
      const j = index.get(b);
      return i === undefined || j === undefined ? [] : [[i, j] as const];
    });
    return !canDraw(people.length, pairs);
  });

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

  async function goTo(next: typeof step) {
    step = next;
    serverError = '';
    await tick();
    window.scrollTo(0, 0);
    headingEl?.focus();
  }

  function toExclusions(event: SubmitEvent) {
    event.preventDefault();
    attempted = true;
    if (hasErrors) return;
    // Drop exclusions for people removed or blanked since the last visit.
    const ids = new Set(people.map((person) => person.id));
    exclusions = exclusions.filter(({ a, b }) => ids.has(a) && ids.has(b));
    void goTo('exclusions');
  }

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    serverError = '';
    if (impossible || submitting) return;

    submitting = true;
    try {
      const nameOf = new Map(people.map((person) => [person.id, person.name]));
      const result = await createRoom({
        participants: people.map((person) => person.name),
        name: roomName.trim() || undefined,
        exclusions: exclusions.map(({ a, b }) => [nameOf.get(a)!, nameOf.get(b)!]),
      });
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
    exclusions = [];
    step = 'names';
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
{:else if step === 'exclusions'}
  <p class="eyebrow">Paso 2 de 2</p>
  <h1 bind:this={headingEl} tabindex="-1">¿Parejas prohibidas?</h1>
  <p>
    Opcional. Si hay parejas, hermanos u otras personas que no deben regalarse entre sí, añádelas aquí.
    Funciona en ambos sentidos: ninguno de los dos le regalará al otro.
  </p>

  <form onsubmit={submit} novalidate>
    <Exclusions {people} bind:pairs={exclusions} />

    {#if impossible}
      <p class="error" role="alert">Con estas exclusiones no hay forma de hacer el sorteo. Quita alguna.</p>
    {/if}
    {#if serverError}<p class="error" role="alert">{serverError}</p>{/if}

    <div class="row form-actions">
      <button type="button" onclick={() => goTo('names')} disabled={submitting}>Atrás</button>
      <button type="submit" disabled={submitting || impossible}>
        {submitting ? 'Creando…' : 'Crear sorteo'}
      </button>
    </div>
  </form>
{:else}
  <p class="eyebrow">Paso 1 de 2</p>
  <h1 bind:this={headingEl} tabindex="-1">Organiza tu amigo invisible</h1>
  <p>
    Añade el nombre de todos. A nadie le puede tocar él mismo y, en el siguiente paso, podrás evitar
    otros emparejamientos, como parejas o hermanos. Te daremos un enlace para compartir con el grupo.
  </p>

  <form onsubmit={toExclusions} novalidate>
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

    <button type="submit">Siguiente</button>
  </form>
{/if}
