<script lang="ts">
  import { app } from '../lib/state.svelte'
  import { formatBytes, fmtDate } from '../lib/api'
  import type { Summary } from '../lib/api'

  const p = $derived(app.prompts[0])

  function newer(a?: Summary | null, b?: Summary | null) {
    if (!a || !b) return ''
    const ta = new Date(a.newest).getTime(), tb = new Date(b.newest).getTime()
    return ta > tb ? 'a' : tb > ta ? 'b' : ''
  }
  const which = $derived(p?.conflict ? newer(p.conflict.local, p.conflict.remote) : '')
</script>

{#if p}
  <div class="backdrop">
    <div class="dialog">
      {#if p.kind === 'conflict' && p.conflict}
        <h2>Save conflict — {p.gameName}</h2>
        <p class="muted">
          Saves changed both on this device and in the cloud since the last sync.
          Choose which to keep. The other version is stored as a backup, so nothing is lost.
        </p>
        <div class="sides">
          {#each [{ k: 'local', label: 'This device', s: p.conflict.local, n: which === 'a' }, { k: 'cloud', label: 'Cloud', s: p.conflict.remote, n: which === 'b' }] as side (side.k)}
            <button class="side" onclick={() => app.respond(p, side.k)}>
              <div class="row"><strong>{side.label}</strong>{#if side.n}<span class="badge ok">newer</span>{/if}</div>
              <div class="small muted">{side.s.deviceName ? `from ${side.s.deviceName}` : ''}</div>
              <div class="small">{side.s.files} files · {formatBytes(side.s.bytes)}</div>
              <div class="small">Last modified {fmtDate(side.s.newest)}</div>
              <div class="keep">Keep {side.k === 'local' ? 'local' : 'cloud'} saves</div>
            </button>
          {/each}
        </div>
        <div class="row end">
          <button onclick={() => app.respond(p, 'skip')}>
            {p.beforePlay ? 'Skip sync and play' : 'Decide later'}
          </button>
        </div>
      {:else if p.kind === 'lock' && p.lock}
        <h2>{p.gameName} is being played elsewhere</h2>
        <p>
          <strong>{p.lock.deviceName}</strong> started playing at {fmtDate(p.lock.since)}.
          If you play here too, you may get a save conflict when both devices upload.
        </p>
        <div class="row end">
          <button onclick={() => app.respond(p, 'no')}>Cancel</button>
          <button class="primary" onclick={() => app.respond(p, 'yes')}>Play anyway</button>
        </div>
      {:else}
        <h2>{p.beforePlay ? 'Could not sync before playing' : 'Could not upload saves'}</h2>
        <p class="err mono small">{p.error}</p>
        {#if p.beforePlay}
          <p class="muted">You can play offline; your saves will be uploaded next time sync succeeds.</p>
          <div class="row end">
            <button onclick={() => app.respond(p, 'no')}>Cancel</button>
            <button class="primary" onclick={() => app.respond(p, 'yes')}>Play offline</button>
          </div>
        {:else}
          <p class="muted">Your saves are safe on this device and will upload on the next sync.</p>
          <div class="row end"><button class="primary" onclick={() => app.respond(p, 'yes')}>OK</button></div>
        {/if}
      {/if}
    </div>
  </div>
{/if}

<style>
  .backdrop { position: fixed; inset: 0; background: #000a; display: grid; place-items: center; z-index: 50; padding: 1em; }
  .dialog { background: var(--bg-2); border: 1px solid var(--line); border-radius: 14px; padding: 1.4em; width: min(560px, 100%); max-height: 100%; overflow: auto; }
  .sides { display: grid; grid-template-columns: 1fr 1fr; gap: .8em; margin: 1em 0; }
  .side { flex-direction: column; align-items: flex-start; gap: .3em; padding: 1em; text-align: left; white-space: normal; }
  .side:hover .keep { color: var(--accent); }
  .keep { margin-top: .5em; font-weight: 600; }
  @media (max-width: 480px) { .sides { grid-template-columns: 1fr; } }
</style>
