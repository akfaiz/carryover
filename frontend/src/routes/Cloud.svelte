<script lang="ts">
  import { onMount } from 'svelte'
  import { app } from '../lib/state.svelte'
  import { CloudService, SyncService, errMsg } from '../lib/api'
  import type { Provider, Question, Remote } from '../lib/api'

  let remotes = $state<Remote[]>([])
  let providers = $state<Provider[]>([])
  let defaultConf = $state('')
  let remote = $state('')
  let root = $state('PlayRelay')
  let testing = $state(false)
  let testResult = $state<{ ok: boolean; msg: string } | null>(null)

  // wizard
  let wizard = $state<'closed' | 'pick' | 'form' | 'question' | 'working'>('closed')
  let prov = $state<Provider | null>(null)
  let newName = $state('')
  let params = $state<Record<string, string>>({})
  let showAll = $state(false)
  let question = $state<Question | null>(null)
  let answer = $state('')
  let wizErr = $state('')

  onMount(async () => {
    await load()
    providers = (await CloudService.Providers()) ?? []
    defaultConf = await CloudService.DefaultRcloneConfig()
  })

  async function load() {
    remotes = (await CloudService.Remotes()) ?? []
    const s = await SyncService.GetSettings()
    remote = s.remote
    root = s.remoteRoot || 'PlayRelay'
  }

  async function saveTarget() {
    if (!app.settings) return
    try {
      await SyncService.SaveSettings({ ...app.settings, remote, remoteRoot: root.trim().replace(/^\/+|\/+$/g, '') || 'PlayRelay' })
      await app.refresh()
      app.toast('Cloud location saved', 'ok')
      if (remote) {
        await SyncService.SyncLibrary().catch((e) => app.toast('Library sync: ' + errMsg(e), 'err'))
      }
    } catch (e) {
      app.toast(errMsg(e), 'err')
    }
  }

  async function test() {
    testing = true
    testResult = null
    try {
      await CloudService.Test(remote, root)
      testResult = { ok: true, msg: 'Connection works.' }
    } catch (e) {
      testResult = { ok: false, msg: errMsg(e) }
    } finally {
      testing = false
    }
  }

  async function del(r: Remote) {
    await CloudService.DeleteRemote(r.name)
    await load()
    await app.refresh()
  }

  async function importConf(path: string) {
    try {
      const names = await CloudService.ImportConfig(path)
      if (names?.length) app.toast(`Imported ${names.join(', ')}`, 'ok')
      await load()
    } catch (e) {
      app.toast(errMsg(e), 'err')
    }
  }

  function pick(p: Provider) {
    prov = p
    newName = p.type === 'drive' ? 'gdrive' : p.type
    let i = 2
    while (remotes.some((r) => r.name === newName)) newName = `${p.type}${i++}`
    params = {}
    for (const o of p.options ?? []) if (o.default && o.default !== '<nil>') params[o.name] = ''
    showAll = false
    wizErr = ''
    wizard = 'form'
  }

  const visibleOptions = $derived(
    (prov?.options ?? []).filter((o) => showAll || o.required || o.name === 'provider' || (o.examples?.length ?? 0) > 0 && o.exclusive)
  )

  async function step(fn: () => Promise<Question | null>) {
    wizErr = ''
    wizard = 'working'
    try {
      const q = await fn()
      if (!q || q.done) {
        await load()
        remote = newName
        wizard = 'closed'
        app.toast(`Remote “${newName}” created`, 'ok')
        return
      }
      question = q
      answer = q.default ?? ''
      if (q.error) wizErr = q.error
      wizard = 'question'
    } catch (e) {
      wizErr = errMsg(e)
      wizard = question ? 'question' : 'form'
    }
  }

  const create = () => step(() => CloudService.CreateRemote(newName.trim(), prov!.type, params))
  const reply = (v: string) => step(() => CloudService.ContinueRemote(newName.trim(), question!.state, v))
</script>

<div class="page">
  <div class="page-head"><h1>Cloud storage</h1></div>

  <div class="card">
    <h2>Sync location</h2>
    <p class="small muted">All devices must use the same remote storage and folder. Saves go to <code>&lt;folder&gt;/games/&lt;game&gt;/</code>.</p>
    <div class="grid2">
      <label class="field"><span>Remote</span>
        <select bind:value={remote}>
          <option value="">— none (sync disabled) —</option>
          {#each remotes as r (r.name)}<option value={r.name}>{r.name} ({r.type})</option>{/each}
        </select>
      </label>
      <label class="field"><span>Folder</span><input class="mono" bind:value={root} /></label>
    </div>
    <div class="row">
      <button class="primary" onclick={saveTarget}>Save</button>
      <button onclick={test} disabled={!remote || testing}>{testing ? 'Testing…' : 'Test connection'}</button>
      {#if testResult}<span class={testResult.ok ? 'ok' : 'err'} class:small={true}>{testResult.msg}</span>{/if}
    </div>
  </div>

  <div class="card">
    <div class="row"><h2>Remotes</h2><span class="spacer"></span>
      {#if defaultConf}<button onclick={() => importConf(defaultConf)} title={defaultConf}>Import from rclone</button>{/if}
      <button onclick={() => importConf('')}>Import rclone.conf…</button>
      <button class="primary" onclick={() => (wizard = 'pick')}>＋ Add remote</button>
    </div>
    {#if remotes.length === 0}
      <p class="muted">No remotes yet. Add one (Google Drive, OneDrive, Dropbox, S3, WebDAV, SFTP, …) or import your existing rclone config.</p>
    {:else}
      <table>
        <tbody>
          {#each remotes as r (r.name)}
            <tr>
              <td><strong>{r.name}</strong> {#if r.name === app.settings?.remote}<span class="badge ok">active</span>{/if}</td>
              <td class="muted">{r.type}</td>
              <td class="right"><button class="ghost danger small" onclick={() => del(r)}>Delete</button></td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>

{#if wizard !== 'closed'}
  <div class="backdrop">
    <div class="dialog">
      {#if wizard === 'pick'}
        <h2>Choose storage provider</h2>
        <div class="providers">
          {#each providers as p (p.type)}
            <button class="prov" onclick={() => pick(p)}>
              <strong>{p.type}</strong><span class="small muted">{p.description}</span>
            </button>
          {/each}
        </div>
        <div class="row end"><button onclick={() => (wizard = 'closed')}>Cancel</button></div>
      {:else if wizard === 'form' && prov}
        <h2>New {prov.description} remote</h2>
        <label class="field"><span>Name</span><input bind:value={newName} /></label>
        {#each visibleOptions as o (o.name)}
          <label class="field">
            <span>{o.name}{o.required ? ' *' : ''} — {o.help}</span>
            {#if o.examples?.length && o.exclusive}
              <select bind:value={params[o.name]}>
                <option value="">(default{o.default ? `: ${o.default}` : ''})</option>
                {#each o.examples as ex (ex.value)}<option value={ex.value}>{ex.value} — {ex.help}</option>{/each}
              </select>
            {:else}
              <input type={o.isPassword ? 'password' : 'text'} bind:value={params[o.name]} placeholder={o.default} list={o.examples?.length ? `ex-${o.name}` : undefined} />
              {#if o.examples?.length}<datalist id={`ex-${o.name}`}>{#each o.examples as ex (ex.value)}<option value={ex.value}>{ex.help}</option>{/each}</datalist>{/if}
            {/if}
          </label>
        {/each}
        {#if (prov.options?.length ?? 0) > visibleOptions.length}
          <button class="ghost small" onclick={() => (showAll = true)}>Show all options</button>
        {/if}
        {#if prov.oauth}<p class="small muted">After you continue, your browser opens to sign in. Come back here when done.</p>{/if}
        {#if wizErr}<p class="err small pre">{wizErr}</p>{/if}
        <div class="row end">
          <button onclick={() => (wizard = 'pick')}>Back</button>
          <button class="primary" onclick={create} disabled={!newName.trim()}>Continue</button>
        </div>
      {:else if wizard === 'question' && question}
        <h2>{prov?.description}</h2>
        <p class="pre">{question.help}</p>
        {#if question.isBool}
          {#if wizErr}<p class="err small pre">{wizErr}</p>{/if}
          <div class="row end">
            <button onclick={() => reply('false')}>No</button>
            <button class="primary" onclick={() => reply('true')}>Yes</button>
          </div>
        {:else}
          {#if question.examples?.length && question.exclusive}
            <select bind:value={answer}>{#each question.examples as ex (ex.value)}<option value={ex.value}>{ex.value} — {ex.help}</option>{/each}</select>
          {:else}
            <input type={question.isPassword ? 'password' : 'text'} bind:value={answer} list="q-ex" />
            {#if question.examples?.length}<datalist id="q-ex">{#each question.examples as ex (ex.value)}<option value={ex.value}>{ex.help}</option>{/each}</datalist>{/if}
          {/if}
          {#if wizErr}<p class="err small pre">{wizErr}</p>{/if}
          <div class="row end" style="margin-top:1em">
            <button onclick={() => (wizard = 'closed')}>Cancel</button>
            <button class="primary" onclick={() => reply(answer)}>Continue</button>
          </div>
        {/if}
      {:else}
        <h2>Configuring…</h2>
        <p class="muted">If a browser window opened, finish signing in there.</p>
        <div class="progress indeterminate"><div></div></div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 1em; }
  .right { text-align: right; }
  .backdrop { position: fixed; inset: 0; background: #000a; display: grid; place-items: center; z-index: 40; padding: 1em; }
  .dialog { background: var(--bg-2); border: 1px solid var(--line); border-radius: 14px; padding: 1.4em; width: min(620px, 100%); max-height: 100%; overflow: auto; }
  .providers { display: grid; grid-template-columns: repeat(auto-fill, minmax(170px, 1fr)); gap: .6em; margin-bottom: 1em; }
  .prov { flex-direction: column; align-items: flex-start; white-space: normal; text-align: left; gap: .2em; }
  .pre { white-space: pre-wrap; }
</style>
