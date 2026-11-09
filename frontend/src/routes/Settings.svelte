<script lang="ts">
  import { app } from '../lib/state.svelte'
  import { LibraryService, SyncService, errMsg } from '../lib/api'
  import type { Settings } from '../lib/api'

  let form = $state<Settings | null>(null)
  let keyTest = $state<{ ok: boolean; msg: string } | null>(null)
  let testingKey = $state(false)

  async function testKey() {
    if (!form) return
    testingKey = true
    keyTest = null
    try {
      await LibraryService.TestSteamGridKey(form.steamGridDbKey ?? '')
      keyTest = { ok: true, msg: 'Key works.' }
    } catch (e) {
      keyTest = { ok: false, msg: errMsg(e) }
    } finally {
      testingKey = false
    }
  }
  $effect(() => {
    if (app.settings && !form) form = { ...app.settings }
  })

  async function save() {
    if (!form) return
    try {
      form = await SyncService.SaveSettings({ ...form, backupRetention: Number(form.backupRetention) })
      await app.refresh()
      app.toast('Settings saved', 'ok')
    } catch (e) {
      app.toast(errMsg(e), 'err')
    }
  }
</script>

<div class="page">
  <div class="page-head"><h1>Settings</h1></div>
  {#if form}
    <div class="card narrow">
      <h2>This device</h2>
      <label class="field"><span>Device name (shown on other devices and in backups)</span><input bind:value={form.deviceName} /></label>
      <label class="field"><span>Device ID</span><input class="mono" readonly value={form.deviceId} /></label>
    </div>
    <div class="card narrow">
      <h2>Sync</h2>
      <label class="field"><span>Backups to keep per game</span><input type="number" min="0" max="100" bind:value={form.backupRetention} /></label>
      <label class="row check"><input type="checkbox" bind:checked={form.syncLibrary} /> Share the game library between devices through the cloud</label>
      <p class="small muted">When enabled, games added on one device appear on the others; each device only needs to set its own paths.</p>
    </div>
    <div class="card narrow">
      <h2>Cover art</h2>
      <label class="field"><span>SteamGridDB API key (optional)</span>
        <div class="row nowrap">
          <input type="password" class="mono" autocomplete="off" bind:value={form.steamGridDbKey} placeholder="Paste your key" />
          <button onclick={testKey} disabled={testingKey || !form.steamGridDbKey}>{testingKey ? 'Testing…' : 'Test'}</button>
        </div>
      </label>
      {#if keyTest}<p class="small" class:ok={keyTest.ok} class:err={!keyTest.ok}>{keyTest.msg}</p>{/if}
      <p class="small muted">
        Lets you search community covers for any game, including ones not on Steam, and fills in covers Steam doesn't have.
        Get a free key at <span class="mono">steamgriddb.com/profile/preferences/api</span> (log in with Steam).
        The key is stored only on this device.
      </p>
    </div>
    <div class="row"><button class="primary" onclick={save}>Save</button></div>

    <div class="card narrow cli">
      <h2>Command line</h2>
      <p class="small muted">Useful for scripts or other launchers (Lutris, Heroic, Playnite…):</p>
      <pre class="mono small">playrelay run &lt;game&gt; [--headless] [-- command…]
playrelay sync &lt;game&gt;|--all [--push|--pull]
playrelay list</pre>
    </div>
  {/if}
</div>

<style>
  .narrow { max-width: 620px; }
  .check { margin-bottom: .4em; }
  .nowrap { flex-wrap: nowrap; }
  .cli { margin-top: 2em; }
  pre { background: var(--bg); padding: .8em; border-radius: 8px; border: 1px solid var(--line); overflow-x: auto; }
</style>
