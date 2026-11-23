<script lang="ts">
  import { onMount } from 'svelte';
  import Admin from './lib/Admin.svelte';
  import CreateRoom from './lib/CreateRoom.svelte';
  import Reveal from './lib/Reveal.svelte';
  import { linkClick, matchRoute } from './lib/router';

  let path = $state(location.pathname);
  const route = $derived(matchRoute(path));

  onMount(() => {
    const onPopState = () => {
      path = location.pathname;
    };
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  });
</script>

<header class="site-header">
  <div class="snow" aria-hidden="true"></div>
  <a class="brand" href="/" onclick={linkClick}>Secret Santa</a>
  <p class="tagline">Sortea nombres · Guarda el secreto</p>
</header>
<div class="ribbon" aria-hidden="true"></div>

<main class="container">
  <div class="card">
    {#key path}
      {#if route.kind === 'create'}
        <CreateRoom />
      {:else if route.kind === 'reveal'}
        <Reveal roomId={route.roomId} />
      {:else if route.kind === 'admin'}
        <Admin roomId={route.roomId} />
      {:else}
        <h1>Página no encontrada</h1>
        <p>Aquí no hay nada. <a href="/" onclick={linkClick}>Crea un sorteo</a>.</p>
      {/if}
    {/key}
  </div>
</main>
