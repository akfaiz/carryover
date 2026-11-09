<script lang="ts">
  import { onMount } from 'svelte'
  import { app } from '../lib/state.svelte'
  import { SessionService, formatBytes } from '../lib/api'
  import type { WrapperState } from '../lib/api'

  let w = $state<WrapperState | null>(null)

  onMount(async () => {
    w = await SessionService.Wrapper()
    await SessionService.Start()
  })

  const status = $derived(w ? app.status[w.gameId] ?? w.status : null)
  const prog = $derived(w ? app.progress[w.gameId] : null)
  const pct = $derived(prog?.totalBytes ? Math.min(100, (100 * prog.bytes) / prog.totalBytes) : 0)
</script>

<div class="session" style={w?.coverUrl ? `--cover:url('${w.coverUrl}')` : ''}>
  <div class="bg"></div>
  <div class="content">
    <div class="brand small muted">PlayRelay</div>
    <h1>{w?.gameName ?? '…'}</h1>
    {#if status}
      <p class:err={status.phase === 'error'}>{status.message}</p>
      {#if status.phase === 'sync'}
        <div class="progress" class:indeterminate={!prog?.totalBytes}><div style="width:{pct}%"></div></div>
        {#if prog?.message}<p class="small muted">{prog.message}{prog.totalBytes ? ` · ${formatBytes(prog.bytes)} / ${formatBytes(prog.totalBytes)}` : ''}</p>{/if}
      {/if}
    {:else}
      <p class="muted">Starting…</p>
      <div class="progress indeterminate"><div></div></div>
    {/if}
  </div>
</div>

<style>
  .session { position: relative; height: 100%; display: grid; place-items: center; overflow: hidden; }
  .bg { position: absolute; inset: -20px; background: var(--cover, linear-gradient(135deg, #2a3350, #0e0f14)) center/cover; filter: blur(18px) brightness(.35); }
  .content { position: relative; width: 100%; padding: 1.5em 2em; text-align: center; }
  h1 { font-size: 1.4rem; margin: .3em 0 .8em; }
  .brand { letter-spacing: .15em; text-transform: uppercase; }
  .progress { margin: .8em auto; max-width: 320px; }
</style>
