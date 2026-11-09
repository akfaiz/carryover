<script lang="ts">
  import { app, go } from '../lib/state.svelte'
  import { SessionService, SyncService, errMsg, timeAgo } from '../lib/api'
  import type { GameView } from '../lib/api'
  import StatusBadge from '../components/StatusBadge.svelte'
  import SteamImport from '../components/SteamImport.svelte'

  let importing = $state(false)

  let filter = $state('')
  let syncingAll = $state(false)
  const games = $derived(app.games.filter((g) => g.name.toLowerCase().includes(filter.toLowerCase())))

  async function play(g: GameView, e: Event) {
    e.stopPropagation()
    if (!g.ready) return go(`/edit/${g.id}`)
    try {
      await SessionService.Play(g.id)
      go(`/game/${g.id}`)
    } catch (err) {
      app.toast(errMsg(err), 'err')
    }
  }

  async function syncAll() {
    syncingAll = true
    try {
      const res = (await SyncService.SyncAll()) ?? []
      const failed = res.filter((r) => r.error)
      const conflicts = res.filter((r) => r.action === 'conflict')
      if (failed.length) app.toast(`${failed.length} game(s) failed to sync: ${failed[0].name}: ${failed[0].error}`, 'err')
      else if (conflicts.length) app.toast(`${conflicts.length} game(s) have save conflicts`, 'err')
      else app.toast(`Synced ${res.length} game(s)`, 'ok')
    } catch (err) {
      app.toast(errMsg(err), 'err')
    } finally {
      syncingAll = false
      app.refresh()
    }
  }
</script>

<div class="page">
  <div class="page-head">
    <h1>Library</h1>
    <input class="search" placeholder="Search…" bind:value={filter} />
    <div class="spacer"></div>
    <button onclick={syncAll} disabled={syncingAll || !app.settings?.remote}>
      {syncingAll ? 'Syncing…' : '⟳ Sync all'}
    </button>
    <button onclick={() => (importing = true)}>Add from Steam</button>
    <button class="primary" onclick={() => go('/new')}>＋ Add game</button>
  </div>

  {#if app.loaded && app.games.length === 0}
    <div class="empty card">
      <h2>No games yet</h2>
      <p class="muted">
        Add a game, tell PlayRelay where its saves live on this device, and it will keep them in sync
        through your cloud storage every time you play — from here or from Steam.
      </p>
      <div class="row">
        <button class="primary" onclick={() => go('/new')}>＋ Add your first game</button>
        {#if !app.settings?.remote}<button onclick={() => go('/cloud')}>☁ Set up cloud storage</button>{/if}
      </div>
    </div>
  {/if}

  <div class="grid">
    {#each games as g (g.id)}
      <div class="tile" role="button" tabindex="0" onclick={() => go(`/game/${g.id}`)} onkeydown={(e) => e.key === 'Enter' && go(`/game/${g.id}`)}>
        <div class="cover" style={g.coverUrl ? `background-image:url('${g.coverUrl}')` : ''}>
          {#if !g.coverUrl}<span class="initial">{g.name.slice(0, 1)}</span>{/if}
          <div class="overlay">
            <button class="primary play" onclick={(e) => play(g, e)} disabled={g.status.running}>
              {g.status.running ? 'Running' : g.ready ? '▶ Play' : 'Set up'}
            </button>
          </div>
        </div>
        <div class="meta">
          <div class="name" title={g.name}>{g.name}</div>
          <div class="row small">
            <StatusBadge game={g} />
            <span class="muted">{g.ready ? timeAgo(g.status.lastSyncAt) : ''}</span>
          </div>
        </div>
      </div>
    {/each}
  </div>
</div>

{#if importing}<SteamImport onclose={() => (importing = false)} />{/if}

<style>
  .search { max-width: 260px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(170px, 1fr)); gap: 1.2em; }
  .tile { cursor: pointer; border-radius: var(--radius); outline: none; }
  .cover {
    aspect-ratio: 2 / 3; border-radius: var(--radius); background: linear-gradient(135deg, #2a3350, #1a1d27);
    background-size: cover; background-position: center; position: relative; overflow: hidden;
    border: 1px solid var(--line); display: grid; place-items: center; transition: transform .15s, box-shadow .15s;
  }
  .tile:hover .cover, .tile:focus .cover { transform: translateY(-3px); box-shadow: 0 10px 30px #0009; }
  .initial { font-size: 3.5rem; font-weight: 700; color: #ffffff30; }
  .overlay { position: absolute; inset: auto 0 0 0; padding: .8em; background: linear-gradient(transparent, #000c); opacity: 0; transition: opacity .15s; display: flex; justify-content: center; }
  .tile:hover .overlay, .tile:focus .overlay { opacity: 1; }
  .play { width: 100%; justify-content: center; }
  .meta { padding: .55em .15em; }
  .name { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; margin-bottom: .3em; }
  .empty { max-width: 560px; }
</style>
