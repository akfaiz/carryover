<script lang="ts">
  import { app } from '../lib/state.svelte'
</script>

<div class="toasts">
  {#each app.toasts as t (t.id)}
    <div class="toast {t.kind}">{t.text}</div>
  {/each}
</div>

<style>
  .toasts { position: fixed; right: 1em; bottom: 1em; display: flex; flex-direction: column; gap: .5em; z-index: 100; max-width: 420px; }
  .toast { background: var(--bg-3); border: 1px solid var(--line); border-left: 3px solid var(--accent); padding: .7em 1em; border-radius: 8px; box-shadow: 0 6px 20px #0008; }
  .toast.ok { border-left-color: var(--ok); }
  .toast.err { border-left-color: var(--err); }
</style>
