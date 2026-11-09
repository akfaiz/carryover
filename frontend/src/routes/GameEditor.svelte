<script lang="ts">
  import { onMount } from 'svelte'
  import { app, go } from '../lib/state.svelte'
  import { LibraryService, errMsg } from '../lib/api'
  import type { DeviceConfig, Token } from '../lib/api'
  import PathInput from '../components/PathInput.svelte'

  let { id }: { id: string } = $props()

  type SlotForm = { key: string; label: string; include: string; exclude: string; path: string; isNew: boolean }

  let name = $state('')
  let slots = $state<SlotForm[]>([])
  let device = $state<DeviceConfig>({
    deviceName: '', paths: {}, exe: '', args: '', workDir: '', env: {}, waitProcess: '',
    launchMode: 'direct' as any, steamAppId: '', updatedAt: '',
  })
  let envText = $state('')
  let tokens = $state<Token[]>([])
  let coverSrc = $state('')
  let coverUrl = $state('')
  let others = $state<{ deviceName: string; paths: Record<string, string | undefined>; exe: string }[]>([])
  let saving = $state(false)
  let error = $state('')
  let advanced = $state(false)
  const excludeHint = '*.log\nshadercache/**'

  onMount(async () => {
    tokens = (await LibraryService.Tokens(id)) ?? []
    if (!id) {
      slots = [{ key: 'saves', label: 'Saves', include: '', exclude: '', path: '', isNew: true }]
      return
    }
    try {
      const g = await LibraryService.Get(id)
      name = g.name
      coverUrl = g.coverUrl
      others = (g.otherDevices ?? []) as any
      device = { ...g.device, paths: { ...(g.device.paths ?? {}) }, env: { ...(g.device.env ?? {}) } }
      envText = Object.entries(device.env ?? {}).map(([k, v]) => `${k}=${v}`).join('\n')
      advanced = !!(device.workDir || device.waitProcess || envText)
      slots = (g.slots ?? []).map((s) => ({
        key: s.key, label: s.label || s.key,
        include: (s.include ?? []).join('\n'), exclude: (s.exclude ?? []).join('\n'),
        path: device.paths?.[s.key] ?? '', isNew: false,
      }))
    } catch (e) {
      error = errMsg(e)
    }
  })

  function slug(s: string) {
    return s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'saves'
  }

  function addSlot() {
    let n = slots.length + 1
    while (slots.some((s) => s.key === `saves-${n}`)) n++
    slots.push({ key: `saves-${n}`, label: `Saves ${n}`, include: '', exclude: '', path: '', isNew: true })
  }

  function labelChanged(s: SlotForm) {
    if (!s.isNew) return
    let k = slug(s.label), base = k, i = 2
    while (slots.some((o) => o !== s && o.key === k)) k = `${base}-${i++}`
    s.key = k
  }

  const lines = (t: string) => t.split(/[\n,]/).map((x) => x.trim()).filter(Boolean)

  async function pickExe() {
    const p = await LibraryService.PickFile('Choose game executable', '', '')
    if (p) {
      device.exe = p
      if (!name) name = p.split(/[\\/]/).pop()!.replace(/\.(exe|sh|x86_64|AppImage)$/i, '')
    }
  }

  async function pickCover() {
    const p = await LibraryService.PickFile('Choose cover image', 'Images', '*.png;*.jpg;*.jpeg;*.webp')
    if (p) coverSrc = p
  }

  async function save() {
    error = ''
    saving = true
    try {
      const env: Record<string, string> = {}
      for (const l of envText.split('\n')) {
        const i = l.indexOf('=')
        if (i > 0) env[l.slice(0, i).trim()] = l.slice(i + 1)
      }
      const paths: Record<string, string> = {}
      for (const s of slots) paths[s.key] = s.path
      const g = await LibraryService.Save({
        id,
        name,
        slots: slots.map((s) => ({ key: s.key, label: s.label, include: lines(s.include), exclude: lines(s.exclude) })),
        device: { ...device, paths, env },
      })
      if (coverSrc) await LibraryService.SetCover(g.id, coverSrc)
      await app.refresh()
      app.toast('Saved', 'ok')
      go(`/game/${g.id}`)
    } catch (e) {
      error = errMsg(e)
    } finally {
      saving = false
    }
  }
</script>

<div class="page">
  <div class="page-head">
    <button class="ghost" onclick={() => history.back()}>←</button>
    <h1>{id ? 'Edit game' : 'Add game'}</h1>
  </div>

  <div class="layout">
    <div>
      <div class="card">
        <h2>Game</h2>
        <label class="field"><span>Name</span><input bind:value={name} placeholder="Game name" /></label>
        <div class="row cover-row">
          <div class="cover-prev" style={coverUrl && !coverSrc ? `background-image:url('${coverUrl}')` : ''}>
            {#if coverSrc}<span class="small mono">{coverSrc.split(/[\\/]/).pop()}</span>{:else if !coverUrl}<span class="muted small">No cover</span>{/if}
          </div>
          <div>
            <button onclick={pickCover}>Choose cover…</button>
            <p class="small muted">Portrait image (600×900 works well). Also used as Steam grid art.</p>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="row"><h2>Launch on this device</h2><span class="spacer"></span><span class="badge">{app.settings?.deviceName}</span></div>
        <label class="field">
          <span>How is the game started?</span>
          <select bind:value={device.launchMode}>
            <option value="direct">PlayRelay starts the executable (Windows, native Linux/macOS games)</option>
            <option value="steamwrap">Steam starts it via Proton; PlayRelay wraps %command% (Steam Deck / Linux)</option>
          </select>
        </label>
        <label class="field">
          <span>Executable</span>
          <div class="row nowrap">
            <input class="mono" bind:value={device.exe} placeholder={device.launchMode === 'steamwrap' ? '/home/deck/Games/A/Game.exe' : 'C:\\Games\\A\\Game.exe'} />
            <button onclick={pickExe}>Browse…</button>
          </div>
        </label>
        <label class="field"><span>Arguments</span><input class="mono" bind:value={device.args} placeholder="-windowed" /></label>
        <button class="ghost small" onclick={() => (advanced = !advanced)}>{advanced ? '▾' : '▸'} Advanced</button>
        {#if advanced}
          <div class="adv">
            <label class="field"><span>Working directory (default: executable's folder)</span>
              <PathInput bind:value={device.workDir} gameId={id} {tokens} /></label>
            <label class="field"><span>Wait for process (when the exe is a launcher that exits immediately)</span>
              <input class="mono" bind:value={device.waitProcess} placeholder="Game-Win64-Shipping.exe" /></label>
            <label class="field"><span>Environment variables (KEY=value per line)</span>
              <textarea class="mono" rows="3" bind:value={envText}></textarea></label>
          </div>
        {/if}
      </div>
    </div>

    <div>
      <div class="card">
        <div class="row"><h2>Save locations</h2><span class="spacer"></span><button onclick={addSlot}>＋ Add</button></div>
        <p class="small muted">
          Each location is synced to its own cloud folder. Every device sets its own path for the same location —
          e.g. <code>{'{appdata}'}/GameA</code> on Windows and <code>{'{proton:shortcut}'}/AppData/Roaming/GameA</code> on a Steam Deck.
        </p>
        {#each slots as s, i (s.key + i)}
          <div class="slot">
            <div class="row nowrap">
              <input class="label" bind:value={s.label} oninput={() => labelChanged(s)} placeholder="Label" />
              <span class="badge mono" title="Cloud folder name">{s.key}</span>
              {#if slots.length > 1}<button class="ghost danger" title="Remove" onclick={() => slots.splice(i, 1)}>✕</button>{/if}
            </div>
            <label class="field"><span>Path on this device</span>
              <PathInput bind:value={s.path} gameId={id} {tokens} placeholder={'{appdata}/Game/Saves'} />
            </label>
            {#each others.filter((o) => o.paths?.[s.key]) as o (o.deviceName)}
              <div class="small muted">On {o.deviceName}: <span class="mono">{o.paths[s.key]}</span></div>
            {/each}
            <details>
              <summary class="small muted">Filters</summary>
              <div class="filters">
                <label class="field"><span>Include only (globs, one per line)</span><textarea class="mono" rows="2" bind:value={s.include} placeholder="*.sav"></textarea></label>
                <label class="field"><span>Exclude (globs)</span><textarea class="mono" rows="2" bind:value={s.exclude} placeholder={excludeHint}></textarea></label>
              </div>
            </details>
          </div>
        {/each}
      </div>
    </div>
  </div>

  {#if error}<p class="err">{error}</p>{/if}
  <div class="row end">
    <button onclick={() => history.back()}>Cancel</button>
    <button class="primary" onclick={save} disabled={saving || !name.trim()}>{saving ? 'Saving…' : 'Save'}</button>
  </div>
</div>

<style>
  .layout { display: grid; grid-template-columns: 1fr 1fr; gap: 1em; align-items: start; }
  @media (max-width: 1000px) { .layout { grid-template-columns: 1fr; } }
  .nowrap { flex-wrap: nowrap; }
  .cover-row { align-items: flex-start; flex-wrap: nowrap; }
  .cover-prev { width: 80px; aspect-ratio: 2/3; border-radius: 8px; border: 1px solid var(--line); background: var(--bg) center/cover; display: grid; place-items: center; text-align: center; overflow: hidden; flex-shrink: 0; padding: 4px; word-break: break-all; }
  .slot { border-top: 1px solid var(--line); padding-top: .9em; margin-top: .9em; }
  .label { font-weight: 600; }
  .filters { display: grid; grid-template-columns: 1fr 1fr; gap: .6em; margin-top: .5em; }
  .adv { margin-top: .8em; }
  details summary { cursor: pointer; margin-top: .3em; }
</style>
