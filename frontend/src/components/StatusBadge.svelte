<script lang="ts">
  import type { GameView } from '../lib/api'
  let { game }: { game: GameView } = $props()
</script>

{#if game.status.running}
  <span class="badge info">running</span>
{:else if !game.ready}
  <span class="badge warn" title={game.setupHint}>needs setup</span>
{:else if game.status.conflict}
  <span class="badge err">conflict</span>
{:else if game.status.lastError}
  <span class="badge err" title={game.status.lastError}>sync error</span>
{:else if game.status.lastSyncAt}
  <span class="badge ok">synced</span>
{:else}
  <span class="badge">not synced</span>
{/if}
