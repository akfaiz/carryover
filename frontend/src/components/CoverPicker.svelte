<script lang="ts">
  import { onMount } from 'svelte'
  import { LibraryService, errMsg } from '../lib/api'
  import type { Grid, GridGame } from '../lib/api'

  let {
    term = '',
    steamAppId = '',
    onpick,
    onclose,
  }: { term?: string; steamAppId?: string; onpick: (g: Grid) => void; onclose: () => void } = $props()

  let query = $state('')
  let games = $state<GridGame[] | null>(null)
  let selected = $state<{ id: number; name: string } | null>(null)
  let grids = $state<Grid[] | null>(null)
  let loading = $state(false)
  let error = $state('')

  onMount(() => {
    query = term
    // A Steam-linked game can show its covers straight away.
    if (steamAppId) loadGrids(0, `Steam app ${steamAppId}`)
    else if (query.trim()) search()
  })

  async function search() {
    if (!query.trim()) return
    loading = true
    error = ''
    selected = null
    grids = null
    try {
      games = (await LibraryService.SearchGridGames(query)) ?? []
    } catch (e) {
      error = errMsg(e)
    } finally {
      loading = false
    }
  }

  async function loadGrids(id: number, name: string) {
    loading = true
    error = ''
    selected = { id, name }
    try {
      grids = (await LibraryService.GridCovers(id, id ? '' : steamAppId)) ?? []
    } catch (e) {
      error = errMsg(e)
      grids = []
    } finally {
      loading = false
    }
  }
</script>

<div class="backdrop">
  <div class="dialog">
    <div class="row"><h2>Find cover on SteamGridDB</h2><span class="spacer"></span><button class="ghost" onclick={onclose}>✕</button></div>
    <div class="row nowrap">
      <input placeholder="Game name" bind:value={query} onkeydown={(e) => e.key === 'Enter' && search()} />
      <button onclick={search} disabled={loading || !query.trim()}>Search</button>
    </div>

    {#if error}<p class="err small">{error}</p>{/if}

    {#if selected}
      <div class="row small crumbs">
        <button class="ghost small" onclick={() => { selected = null; grids = null; if (!games && query.trim()) search() }}>← Results</button>
        <strong>{selected.name}</strong>
      </div>
      {#if loading}
        <p class="muted">Loading covers…</p>
      {:else if grids && grids.length === 0}
        <p class="muted">No portrait covers for this game yet.</p>
      {:else if grids}
        <div class="grid">
          {#each grids as g (g.id)}
            <button class="ghost tile" title={`${g.width}×${g.height}${g.author ? ' · by ' + g.author : ''}`} onclick={() => onpick(g)}>
              <img src={g.thumb || g.url} alt="" loading="lazy" />
            </button>
          {/each}
        </div>
        <p class="small muted">Art by the SteamGridDB community. Click one to use it.</p>
      {/if}
    {:else if loading}
      <p class="muted">Searching…</p>
    {:else if games}
      {#if games.length === 0}<p class="muted">No games found.</p>{/if}
      <div class="list">
        {#each games as g (g.id)}
          <button class="ghost item" onclick={() => loadGrids(g.id, g.name)}>
            <span>{g.name}</span>
            <span class="small muted">{g.year || ''} {#if g.verified}✓{/if}</span>
          </button>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .backdrop { position: fixed; inset: 0; background: #000a; display: grid; place-items: center; z-index: 45; padding: 1em; }
  .dialog { background: var(--bg-2); border: 1px solid var(--line); border-radius: 14px; padding: 1.4em; width: min(760px, 100%); max-height: 100%; overflow: auto; }
  .nowrap { flex-wrap: nowrap; margin-bottom: .8em; }
  .crumbs { margin-bottom: .6em; }
  .list { max-height: 50vh; overflow: auto; }
  .item { width: 100%; justify-content: space-between; }
  .item:hover { background: var(--bg-3); }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(120px, 1fr)); gap: .6em; max-height: 55vh; overflow: auto; }
  .tile { padding: 0; border-radius: 8px; overflow: hidden; aspect-ratio: 2 / 3; border: 2px solid transparent; }
  .tile:hover { border-color: var(--accent); }
  .tile img { width: 100%; height: 100%; object-fit: cover; display: block; }
</style>
