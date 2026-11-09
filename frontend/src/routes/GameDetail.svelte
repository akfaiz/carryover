<script lang="ts">
  import { onMount } from 'svelte'
  import { app, go } from '../lib/state.svelte'
  import { LibraryService, SessionService, SteamService, SyncService, errMsg, fmtDate, formatBytes, timeAgo } from '../lib/api'
  import type { Backup, PlanView, SteamInfo } from '../lib/api'
  import StatusBadge from '../components/StatusBadge.svelte'

  let { id }: { id: string } = $props()

  const g = $derived(app.game(id))
  const status = $derived(app.status[id])
  const prog = $derived(app.progress[id])

  let busy = $state('')
  let plan = $state<PlanView | null>(null)
  let backups = $state<Backup[] | null>(null)
  let steam = $state<SteamInfo | null>(null)
  let steamUser = $state('')
  let steamMode = $state('direct')
  let wrapOpts = $state('')
  let confirmDelete = $state(false)

  onMount(async () => {
    steam = await SteamService.Info()
    steamUser = steam.users?.[0]?.id ?? ''
    wrapOpts = await SteamService.WrapLaunchOptions(id).catch(() => '')
  })

  $effect(() => {
    if (g) steamMode = g.device.launchMode === 'steamwrap' ? 'steamwrap' : 'direct'
  })

  async function run(label: string, fn: () => Promise<any>) {
    busy = label
    try {
      return await fn()
    } catch (e) {
      app.toast(errMsg(e), 'err')
    } finally {
      busy = ''
    }
  }

  const play = () => run('play', () => SessionService.Play(id))

  async function sync(resolution = '') {
    const action = await run('sync', () => SyncService.Sync(id, resolution))
    if (action === 'conflict') app.toast('Save conflict: choose which version to keep below', 'err')
    else if (action) app.toast(action === 'none' ? 'Already up to date' : action === 'push' ? 'Uploaded saves' : 'Downloaded saves', 'ok')
    plan = null
    if (backups) loadBackups()
  }

  async function check() {
    plan = (await run('plan', () => SyncService.Plan(id))) ?? null
  }

  async function loadBackups() {
    backups = (await run('backups', () => SyncService.Backups(id))) ?? []
  }

  async function restore(b: Backup) {
    await run('restore', () => SyncService.RestoreBackup(id, b.name))
    app.toast('Backup restored and uploaded', 'ok')
    loadBackups()
  }

  async function addToSteam() {
    const res = await run('steam', () => SteamService.AddToSteam(id, steamUser, steamMode))
    if (res) {
      app.toast(res.steamRunning ? 'Added to Steam. Restart Steam to see it.' : 'Added to Steam', 'ok')
    }
  }

  async function removeFromSteam() {
    const n = await run('steam', () => SteamService.RemoveFromSteam(id, steamUser))
    if (n !== undefined) app.toast(n ? 'Removed from Steam' : 'No shortcut found', 'ok')
  }

  async function del() {
    await run('delete', () => LibraryService.Delete(id))
    go('/')
  }

  function copy(text: string) {
    navigator.clipboard.writeText(text).then(() => app.toast('Copied', 'ok'))
  }

  const planText: Record<string, string> = {
    none: 'Local and cloud saves are identical.',
    push: 'Local saves are newer — they will be uploaded.',
    pull: 'Cloud saves are newer — they will be downloaded.',
    conflict: 'Both changed since the last sync. Choose which to keep.',
  }
</script>

{#if !g}
  <div class="page"><p class="muted">{app.loaded ? 'Game not found.' : 'Loading…'}</p></div>
{:else}
  <div class="page">
    <div class="hero">
      <div class="cover" style={g.coverUrl ? `background-image:url('${g.coverUrl}')` : ''}>
        {#if !g.coverUrl}<span>{g.name.slice(0, 1)}</span>{/if}
      </div>
      <div class="info">
        <button class="ghost back" onclick={() => go('/')}>← Library</button>
        <h1>{g.name}</h1>
        <div class="row">
          <StatusBadge game={g} />
          <span class="muted small">Last sync: {timeAgo(g.status.lastSyncAt)}</span>
        </div>
        {#if !g.ready}<p class="warn small">{g.setupHint}</p>{/if}
        {#if g.status.lastError && !g.status.running}<p class="err small mono">{g.status.lastError}</p>{/if}

        <div class="row actions">
          {#if g.status.running}
            <button class="primary big" disabled>● Running</button>
          {:else if g.ready && g.device.launchMode !== 'steamwrap'}
            <button class="primary big" onclick={play} disabled={!!busy}>▶ Play</button>
          {:else if !g.ready}
            <button class="primary big" onclick={() => go(`/edit/${id}`)}>Set up for this device</button>
          {/if}
          <button onclick={() => sync()} disabled={!!busy || !g.ready || !app.settings?.remote || g.status.running}>
            {busy === 'sync' ? 'Syncing…' : '⟳ Sync now'}
          </button>
          <button onclick={() => go(`/edit/${id}`)}>✎ Edit</button>
        </div>
        {#if g.device.launchMode === 'steamwrap' && g.ready}
          <p class="small muted">This game is launched from Steam on this device (Proton). Use “Add to Steam” below, then start it from Steam.</p>
        {/if}

        {#if status && (status.phase !== 'done' || g.status.running) && status.phase !== 'error'}
          <div class="session card">
            <div class="row small"><strong>{status.message}</strong><span class="spacer"></span>
              {#if prog && prog.totalBytes}<span class="muted">{formatBytes(prog.bytes)} / {formatBytes(prog.totalBytes)}</span>{/if}
            </div>
            {#if status.phase !== 'playing'}
              <div class="progress" class:indeterminate={!prog?.totalBytes}>
                <div style="width:{prog?.totalBytes ? (100 * prog.bytes) / prog.totalBytes : 0}%"></div>
              </div>
            {/if}
          </div>
        {/if}
      </div>
    </div>

    <div class="cols">
      <div>
        <div class="card">
          <div class="row"><h2>Saves</h2><span class="spacer"></span>
            <button onclick={check} disabled={!!busy || !g.ready || !app.settings?.remote}>{busy === 'plan' ? 'Checking…' : 'Compare with cloud'}</button>
          </div>
          <table>
            <tbody>
              {#each g.slots ?? [] as s (s.key)}
                <tr>
                  <td><strong>{s.label || s.key}</strong></td>
                  <td class="mono small path">
                    {#if g.pathErrors?.[s.key]}<span class="err">{g.pathErrors?.[s.key]}</span>
                    {:else if g.resolvedPaths?.[s.key]}
                      <button class="ghost link mono" title="Open folder" onclick={() => LibraryService.OpenFolder(g.resolvedPaths?.[s.key] ?? '').catch((e) => app.toast(errMsg(e), 'err'))}>{g.resolvedPaths?.[s.key]}</button>
                    {:else}<span class="warn">not set on this device</span>{/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
          {#if plan}
            <div class="plan">
              <p><strong>{planText[plan.action]}</strong></p>
              <div class="sides small">
                <div><div class="muted">This device</div>{plan.local.files} files · {formatBytes(plan.local.bytes)}<br />modified {fmtDate(plan.local.newest)}</div>
                <div><div class="muted">Cloud {plan.remote.deviceName ? `(from ${plan.remote.deviceName})` : ''}</div>
                  {#if plan.hasRemote}{plan.remote.files} files · {formatBytes(plan.remote.bytes)}<br />uploaded {fmtDate(plan.remote.time)}{:else}empty{/if}</div>
              </div>
              {#if plan.action === 'conflict' || g.status.conflict}
                <div class="row">
                  <button onclick={() => sync('local')}>Keep this device's saves</button>
                  <button onclick={() => sync('cloud')} disabled={!plan.hasRemote}>Keep cloud saves</button>
                </div>
              {/if}
            </div>
          {:else if g.status.conflict}
            <div class="plan">
              <p class="warn">Unresolved save conflict. Compare with cloud to choose a version.</p>
            </div>
          {/if}
        </div>

        <div class="card">
          <div class="row"><h2>Backups</h2><span class="spacer"></span>
            <button onclick={loadBackups} disabled={!!busy || !app.settings?.remote}>{backups ? 'Refresh' : 'Show backups'}</button>
          </div>
          <p class="small muted">A backup is taken in the cloud whenever saves are about to be overwritten (keeping the last {app.settings?.backupRetention}).</p>
          {#if backups}
            {#if backups.length === 0}<p class="muted">No backups yet.</p>{/if}
            <table>
              <tbody>
                {#each backups as b (b.name)}
                  <tr>
                    <td>{fmtDate(b.time)}</td>
                    <td class="small">{b.kind === 'local' ? `${b.deviceName}'s local saves` : `cloud saves (replaced by ${b.deviceName})`}</td>
                    <td class="right"><button class="small" onclick={() => restore(b)} disabled={!!busy || !g.ready}>{busy === 'restore' ? '…' : 'Restore'}</button></td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </div>
      </div>

      <div>
        <div class="card">
          <h2>Steam</h2>
          {#if !steam}
            <p class="muted">Detecting Steam…</p>
          {:else if !steam.found}
            <p class="muted">Steam was not found on this device.</p>
          {:else}
            <p class="small muted">Add {g.name} to Steam as a non-Steam game. Steam launches it through PlayRelay, so saves sync automatically before and after every session — also in Steam Deck Game Mode.</p>
            {#if (steam.users?.length ?? 0) > 1}
              <label class="field"><span>Steam account</span>
                <select bind:value={steamUser}>{#each steam.users ?? [] as u (u.id)}<option value={u.id}>{u.name}</option>{/each}</select>
              </label>
            {/if}
            <label class="field"><span>Shortcut type</span>
              <select bind:value={steamMode}>
                <option value="direct">Run PlayRelay (it starts the game)</option>
                <option value="steamwrap">Run the game exe, wrapped by PlayRelay (pick Proton in Steam)</option>
              </select>
            </label>
            {#if steamMode === 'steamwrap'}
              <p class="small muted">After adding, open the shortcut's Properties → Compatibility in Steam and force a Proton version.
                Its save prefix is available as <code>{'{proton:shortcut}'}</code>{g.device.steamAppId ? ` (app id ${g.device.steamAppId})` : ''}.</p>
            {/if}
            <div class="row">
              <button class="primary" onclick={addToSteam} disabled={!!busy || !steamUser}>{g.device.steamAppId ? 'Update Steam shortcut' : 'Add to Steam'}</button>
              {#if g.device.steamAppId}<button onclick={removeFromSteam} disabled={!!busy}>Remove</button>{/if}
            </div>
            {#if steam.running}<p class="small warn">Steam is running — restart it after adding so it picks up the change.</p>{/if}
          {/if}
          {#if wrapOpts}
            <details open={!!g.storeAppId && g.device.launchMode === 'steamwrap'}>
              <summary class="small muted">Already own it on Steam?</summary>
              <p class="small muted">Paste this into the game's Steam launch options to sync around Steam's own launch:</p>
              <div class="row nowrap"><input class="mono small" readonly value={wrapOpts} /><button onclick={() => copy(wrapOpts)}>Copy</button></div>
            </details>
          {/if}
        </div>

        {#if g.otherDevices?.length}
          <div class="card">
            <h2>Other devices</h2>
            {#each g.otherDevices as d (d.deviceName)}
              <div class="dev">
                <strong>{d.deviceName}</strong>
                {#each g.slots ?? [] as s (s.key)}{#if d.paths?.[s.key]}<div class="small mono muted">{s.label || s.key}: {d.paths[s.key]}</div>{/if}{/each}
              </div>
            {/each}
          </div>
        {/if}

        <div class="card">
          {#if !confirmDelete}
            <button class="danger" onclick={() => (confirmDelete = true)}>Delete from library…</button>
          {:else}
            <p>Remove {g.name} from the library on all devices? Cloud saves and backups are kept.</p>
            <div class="row"><button onclick={() => (confirmDelete = false)}>Cancel</button><button class="danger" onclick={del}>Delete</button></div>
          {/if}
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .hero { display: flex; gap: 1.6em; margin-bottom: 1.4em; align-items: flex-start; }
  .cover { width: 170px; flex-shrink: 0; aspect-ratio: 2/3; border-radius: var(--radius); background: linear-gradient(135deg, #2a3350, #1a1d27) center/cover; border: 1px solid var(--line); display: grid; place-items: center; font-size: 3.5rem; font-weight: 700; color: #ffffff30; }
  .info { flex: 1; min-width: 0; }
  .back { padding-left: 0; margin-bottom: .4em; }
  .actions { margin: 1.2em 0 .6em; }
  .session { margin-top: 1em; max-width: 520px; }
  .session .progress { margin-top: .6em; }
  .cols { display: grid; grid-template-columns: 3fr 2fr; gap: 1em; align-items: start; }
  @media (max-width: 1000px) { .cols { grid-template-columns: 1fr; } }
  .path { word-break: break-all; }
  .link { padding: 0; color: var(--text); text-align: left; white-space: normal; word-break: break-all; }
  .link:hover { color: var(--accent); background: transparent !important; }
  .plan { margin-top: 1em; padding-top: 1em; border-top: 1px solid var(--line); }
  .sides { display: grid; grid-template-columns: 1fr 1fr; gap: 1em; margin-bottom: 1em; }
  .right { text-align: right; }
  .nowrap { flex-wrap: nowrap; }
  .dev { margin-bottom: .7em; }
  details { margin-top: 1em; }
  summary { cursor: pointer; }
</style>
