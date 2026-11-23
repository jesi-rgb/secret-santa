<script module lang="ts">
  /** Two participants (by row id) who must not gift each other, in either direction. */
  export type ExclusionPair = { a: number; b: number };
</script>

<script lang="ts">
  let {
    people,
    pairs = $bindable(),
  }: {
    people: { id: number; name: string }[];
    pairs: ExclusionPair[];
  } = $props();

  let first = $state<number | null>(null);
  let second = $state<number | null>(null);

  const nameOf = $derived(new Map(people.map((person) => [person.id, person.name])));

  const paired = (x: number, y: number) =>
    pairs.some((pair) => (pair.a === x && pair.b === y) || (pair.a === y && pair.b === x));

  const canAdd = $derived(first !== null && second !== null && first !== second && !paired(first, second));

  function setFirst(id: number | null) {
    first = id;
    // Keep the second pick only if it still makes a new pair with the first.
    if (second !== null && (second === id || (id !== null && paired(id, second)))) second = null;
  }

  function add() {
    if (!canAdd || first === null || second === null) return;
    pairs.push({ a: first, b: second });
    // Keep the first person: excluding several people for one is common.
    second = null;
  }

  function remove(index: number) {
    pairs.splice(index, 1);
  }
</script>

<fieldset class="field">
  <legend>Añadir exclusión</legend>
  <div class="row">
    <select bind:value={() => first, setFirst} aria-label="Primera persona">
      <option value={null} disabled>Elige a alguien</option>
      {#each people as person (person.id)}
        <option value={person.id}>{person.name}</option>
      {/each}
    </select>
    <select bind:value={second} disabled={first === null} aria-label="Segunda persona">
      <option value={null} disabled>Y a quién</option>
      {#each people as person (person.id)}
        {#if person.id !== first}
          <option value={person.id} disabled={first !== null && paired(first, person.id)}>{person.name}</option>
        {/if}
      {/each}
    </select>
    <button type="button" onclick={add} disabled={!canAdd}>Excluir</button>
  </div>
</fieldset>

{#if pairs.length}
  <ul class="exclusions" aria-label="Exclusiones">
    {#each pairs as pair, i (`${pair.a}-${pair.b}`)}
      {@const a = nameOf.get(pair.a)}
      {@const b = nameOf.get(pair.b)}
      <li>
        <span class="pair">
          {a}<span class="pair-sep" aria-hidden="true">↔</span><span class="sr-only"> y </span>{b}
        </span>
        <button type="button" onclick={() => remove(i)} aria-label={`Quitar exclusión entre ${a} y ${b}`}>
          Quitar
        </button>
      </li>
    {/each}
  </ul>
{:else}
  <p class="muted">Sin exclusiones: a cualquiera le puede tocar cualquiera.</p>
{/if}
