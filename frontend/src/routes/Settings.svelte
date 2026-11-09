<script lang="ts">
  import { app } from '../lib/state.svelte'
  import { SyncService, errMsg } from '../lib/api'
  import type { Settings } from '../lib/api'

  let form = $state<Settings | null>(null)
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
  .cli { margin-top: 2em; }
  pre { background: var(--bg); padding: .8em; border-radius: 8px; border: 1px solid var(--line); overflow-x: auto; }
</style>
