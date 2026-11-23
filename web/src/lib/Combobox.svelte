<script module lang="ts">
  export type ComboboxOption = { value: string; disabled?: boolean; hint?: string };
</script>

<script lang="ts">
  import { normalizeName } from './names';

  // Editable combobox with list autocomplete (WAI-ARIA APG pattern).
  // `value` is the input text; picking an option replaces it with the option's value.
  let {
    id,
    label,
    options,
    value = $bindable(''),
    placeholder = '',
    invalid = false,
  }: {
    id: string;
    /** Accessible name for the popup list. */
    label: string;
    options: ComboboxOption[];
    value?: string;
    placeholder?: string;
    invalid?: boolean;
  } = $props();

  const listId = $derived(`${id}-listbox`);
  const optionId = (index: number) => `${id}-option-${index}`;

  let open = $state(false);
  let activeIndex = $state(-1);
  /** Text the list is filtered by; cleared after picking so reopening shows everyone. */
  let filterText = $state('');
  let inputEl: HTMLInputElement | undefined = $state();
  let listEl: HTMLUListElement | undefined = $state();

  const matches = $derived.by(() => {
    const query = normalizeName(filterText);
    return query ? options.filter((o) => normalizeName(o.value).includes(query)) : options;
  });
  const showList = $derived(open && matches.length > 0);
  const selectedKey = $derived(normalizeName(value));

  $effect(() => {
    if (!showList || activeIndex < 0) return;
    listEl?.querySelector(`#${CSS.escape(optionId(activeIndex))}`)?.scrollIntoView({ block: 'nearest' });
  });

  function enabledIndices(): number[] {
    return matches.flatMap((option, i) => (option.disabled ? [] : [i]));
  }

  function move(delta: 1 | -1) {
    const enabled = enabledIndices();
    if (!enabled.length) {
      activeIndex = -1;
      return;
    }
    const pos = enabled.indexOf(activeIndex);
    activeIndex =
      pos === -1
        ? enabled[delta > 0 ? 0 : enabled.length - 1]
        : enabled[(pos + delta + enabled.length) % enabled.length];
  }

  function openList() {
    open = true;
    const current = matches.findIndex((o) => !o.disabled && normalizeName(o.value) === selectedKey);
    activeIndex = current;
  }

  function close() {
    open = false;
    activeIndex = -1;
  }

  function pick(option: ComboboxOption) {
    if (option.disabled) return;
    value = option.value;
    filterText = '';
    close();
    inputEl?.focus();
  }

  function onInput(event: Event & { currentTarget: HTMLInputElement }) {
    value = event.currentTarget.value;
    filterText = value;
    open = true;
    // Highlight the first match so Enter picks it.
    activeIndex = filterText.trim() ? (enabledIndices()[0] ?? -1) : -1;
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.isComposing) return;
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        if (!open) {
          openList();
          if (!event.altKey && activeIndex === -1) move(1);
        } else {
          move(1);
        }
        break;
      case 'ArrowUp':
        event.preventDefault();
        if (!open) {
          openList();
          if (activeIndex === -1) move(-1);
        } else {
          move(-1);
        }
        break;
      case 'Enter':
        // With nothing highlighted, Enter falls through and submits the form.
        if (showList && activeIndex >= 0) {
          event.preventDefault();
          pick(matches[activeIndex]);
        } else {
          close();
        }
        break;
      case 'Escape':
        if (open) {
          event.preventDefault();
          close();
        } else if (value) {
          event.preventDefault();
          value = '';
          filterText = '';
        }
        break;
      case 'Tab':
        close();
        break;
    }
  }

  function onFocusout(event: FocusEvent & { currentTarget: HTMLDivElement }) {
    if (!event.currentTarget.contains(event.relatedTarget as Node | null)) close();
  }

  function toggle() {
    if (open) close();
    else openList();
    inputEl?.focus();
  }

  // Keep focus in the input while clicking inside the popup.
  const keepFocus = (event: MouseEvent) => event.preventDefault();
</script>

<div class="combobox" onfocusout={onFocusout}>
  <input
    bind:this={inputEl}
    {id}
    {value}
    {placeholder}
    type="text"
    role="combobox"
    aria-autocomplete="list"
    aria-expanded={showList}
    aria-controls={listId}
    aria-activedescendant={showList && activeIndex >= 0 ? optionId(activeIndex) : undefined}
    aria-invalid={invalid ? 'true' : undefined}
    autocomplete="off"
    autocapitalize="words"
    spellcheck="false"
    oninput={onInput}
    onkeydown={onKeydown}
    onclick={() => {
      if (!open) openList();
    }}
  />
  <button
    type="button"
    class="combobox-toggle"
    tabindex="-1"
    aria-label={`Mostrar ${label.toLowerCase()}`}
    aria-controls={listId}
    aria-expanded={showList}
    onmousedown={keepFocus}
    onclick={toggle}
  >
    <svg viewBox="0 0 12 8" aria-hidden="true">
      <path d="M1 1.5 6 6.5l5-5" />
    </svg>
  </button>

  {#if showList}
    <ul bind:this={listEl} id={listId} class="combobox-popup" role="listbox" aria-label={label}>
      {#each matches as option, i (option.value)}
        <!-- Keyboard selection goes through the input via aria-activedescendant. -->
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <li
          id={optionId(i)}
          class="combobox-option"
          role="option"
          aria-selected={i === activeIndex}
          aria-disabled={option.disabled ? 'true' : undefined}
          onmousedown={keepFocus}
          onmousemove={() => {
            if (!option.disabled) activeIndex = i;
          }}
          onclick={() => pick(option)}
        >
          <span class="combobox-label">{option.value}</span>
          {#if option.hint}
            <span class="combobox-hint">{option.hint}</span>
          {:else if normalizeName(option.value) === selectedKey}
            <span class="combobox-check" aria-hidden="true">✓</span>
          {/if}
        </li>
      {/each}
    </ul>
  {:else if open}
    <div class="combobox-popup combobox-empty" role="status">Ningún nombre coincide con “{filterText.trim()}”</div>
  {/if}
</div>
