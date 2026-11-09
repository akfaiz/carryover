<script lang="ts">
  import { onMount } from 'svelte'
  import { go } from '../lib/state.svelte'
  import { LibraryService, errMsg } from '../lib/api'
  import type { SteamLibraryGame } from '../lib/api'

  let { onclose }: { onclose: () => void } = $props()

  let games = $state<SteamLibraryGame[] | null>(null)
  let error = $state('')
  let filter = $state('')
  const shown = $derived((games ?? []).filter((g) => g.name.toLowerCase().includes(filter.toLowerCase())))

  onMount(async () => {
    try {
      games = (await LibraryService.SteamLibrary()) ?? []
    } catch (e) {
      error = errMsg(e)
      games = []
    }
  })

  function pick(g: SteamLibraryGame) {
    onclose()
    go(g.gameId ? `/game/${g.gameId}` : `/new/steam/${g.appId}`)
  }
</script>

<div class="backdrop">
  <div class="dialog">
    <div class="row"><h2>Add from Steam library</h2><span class="spacer"></span><button class="ghost" onclick={onclose}>✕</button></div>
    <p class="small muted">
      Games installed through Steam on this device. The title and cover come from Steam; you still set the save locations.
      Steam keeps launching the game — PlayRelay syncs around it through the launch options shown on the game page.
    </p>
    <input placeholder="Filter…" bind:value={filter} />
    <div class="list">
      {#if games === null}
        <p class="muted">Reading Steam library…</p>
      {:else if error}
        <p class="err">{error}</p>
      {:else if games.length === 0}
        <p class="muted">No installed Steam games found on this device.</p>
      {/if}
      {#each shown as g (g.appId)}
        <button class="ghost item" onclick={() => pick(g)}>
          <img src={`https://cdn.cloudflare.steamstatic.com/steam/apps/${g.appId}/library_600x900.jpg`} alt="" loading="lazy"
            onerror={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')} />
          <span class="name">{g.name}</span>
          {#if g.gameId}<span class="badge ok">in library</span>{/if}
        </button>
      {/each}
    </div>
  </div>
</div>

<style>
  .backdrop { position: fixed; inset: 0; background: #000a; display: grid; place-items: center; z-index: 40; padding: 1em; }
  .dialog { background: var(--bg-2); border: 1px solid var(--line); border-radius: 14px; padding: 1.4em; width: min(620px, 100%); max-height: 100%; display: flex; flex-direction: column; }
  .list { margin-top: .8em; overflow: auto; max-height: 55vh; }
  .item { width: 100%; justify-content: flex-start; gap: .8em; padding: .4em; }
  .item:hover { background: var(--bg-3); }
  .item img { width: 32px; height: 48px; object-fit: cover; border-radius: 4px; background: var(--bg); flex-shrink: 0; }
  .name { flex: 1; text-align: left; white-space: normal; }
</style>
