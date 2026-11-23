<script lang="ts">
  import { copyText } from './clipboard';

  let { text, label = 'Copiar' }: { text: string; label?: string } = $props();

  let status = $state<'idle' | 'copied' | 'failed'>('idle');
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function copy() {
    const ok = await copyText(text);
    status = ok ? 'copied' : 'failed';
    clearTimeout(timer);
    timer = setTimeout(() => {
      status = 'idle';
    }, 2000);
  }

  $effect(() => () => clearTimeout(timer));
</script>

<button type="button" onclick={copy}>{status === 'copied' ? '¡Copiado!' : label}</button>
<span role="status">
  {#if status === 'failed'}
    <span class="error">No se ha podido copiar. Selecciónalo y cópialo a mano.</span>
  {/if}
</span>
