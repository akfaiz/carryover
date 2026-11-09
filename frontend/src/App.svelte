<script lang="ts">
  import { onMount } from 'svelte'
  import { app, go } from './lib/state.svelte'
  import Library from './routes/Library.svelte'
  import GameDetail from './routes/GameDetail.svelte'
  import GameEditor from './routes/GameEditor.svelte'
  import Cloud from './routes/Cloud.svelte'
  import Settings from './routes/Settings.svelte'
  import Session from './routes/Session.svelte'
  import PromptDialog from './components/PromptDialog.svelte'
  import Toasts from './components/Toasts.svelte'

  onMount(() => app.start())

  const parts = $derived(app.route.split('/').filter(Boolean))
  const section = $derived(parts[0] ?? '')
  const nav = [
    { path: '/', label: 'Library', icon: '▦' },
    { path: '/cloud', label: 'Cloud', icon: '☁' },
    { path: '/settings', label: 'Settings', icon: '⚙' },
  ]
  const active = (p: string) => (p === '/' ? section === '' || section === 'game' || section === 'edit' || section === 'new' : app.route.startsWith(p))
</script>

{#if section === 'session'}
  <Session />
{:else}
  <div class="shell">
    <aside>
      <div class="brand"><span class="logo">▶</span> PlayRelay</div>
      {#each nav as n (n.path)}
        <button class="nav" class:active={active(n.path)} onclick={() => go(n.path)}>
          <span class="icon">{n.icon}</span>{n.label}
        </button>
      {/each}
      <div class="spacer"></div>
      {#if app.settings}
        <div class="device small muted">
          <div>{app.settings.deviceName}</div>
          {#if app.settings.remote}
            <div class="mono">{app.settings.remote}:{app.settings.remoteRoot}</div>
          {:else}
            <button class="ghost small warn" onclick={() => go('/cloud')}>⚠ No cloud configured</button>
          {/if}
        </div>
      {/if}
    </aside>
    <main>
      {#if section === 'game' && parts[1]}
        {#key parts[1]}<GameDetail id={parts[1]} />{/key}
      {:else if section === 'edit' && parts[1]}
        {#key parts[1]}<GameEditor id={parts[1]} />{/key}
      {:else if section === 'new'}
        <GameEditor id="" />
      {:else if section === 'cloud'}
        <Cloud />
      {:else if section === 'settings'}
        <Settings />
      {:else}
        <Library />
      {/if}
    </main>
  </div>
{/if}

<PromptDialog />
<Toasts />

<style>
  .shell { display: flex; height: 100%; }
  aside {
    width: 200px; flex-shrink: 0; background: var(--bg-2); border-right: 1px solid var(--line);
    display: flex; flex-direction: column; padding: 1em .7em; gap: .2em;
  }
  .brand { font-weight: 700; font-size: 1.15rem; padding: .2em .6em 1em; display: flex; align-items: center; gap: .5em; }
  .logo { background: var(--accent); color: #fff; border-radius: 7px; width: 1.6em; height: 1.6em; display: grid; place-items: center; font-size: .8em; }
  .nav { background: transparent; border-color: transparent; justify-content: flex-start; width: 100%; padding: .6em .8em; }
  .nav.active { background: var(--bg-3); border-color: var(--line); }
  .icon { width: 1.4em; text-align: center; opacity: .8; }
  .spacer { flex: 1; }
  .device { padding: .6em; border-top: 1px solid var(--line); word-break: break-all; }
  main { flex: 1; min-width: 0; }
</style>
