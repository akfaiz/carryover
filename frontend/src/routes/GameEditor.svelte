<script lang="ts">
  import { onMount } from 'svelte'
  import { app, go } from '../lib/state.svelte'
  import { LibraryService, errMsg, formatBytes, timeAgo } from '../lib/api'
  import type { DeviceInput, Grid, SaveSuggestion, SteamMatch, SuggestResult, Token } from '../lib/api'
  import PathInput from '../components/PathInput.svelte'
  import CoverPicker from '../components/CoverPicker.svelte'

  // steamAppId pre-fills a new game from the installed Steam library.
  let { id, steamAppId = '' }: { id: string; steamAppId?: string } = $props()

  type SlotForm = { key: string; label: string; include: string; exclude: string; path: string; isNew: boolean }

  let name = $state('')
  let slots = $state<SlotForm[]>([])
  let device = $state<DeviceInput>({
    paths: {}, exe: '', args: '', workDir: '', env: {}, waitProcess: '', launchMode: 'direct' as any,
  })
  let envText = $state('')
  let tokens = $state<Token[]>([])
  let coverSrc = $state('')
  // A cover chosen on SteamGridDB; downloaded when the game is saved.
  let gridPick = $state<Grid | null>(null)
  let pickingCover = $state(false)
  let hasGridKey = $state(false)
  let coverUrl = $state('')
  let storeAppId = $state('')
  let originalStoreAppId = $state('')
  let steamResults = $state<SteamMatch[] | null>(null)
  let searching = $state(false)
  let steamPreviewFailed = $state(false)
  let suggest = $state<SuggestResult | null>(null)
  let suggesting = $state(false)
  // Shown when there is no own cover yet but the game is linked to Steam.
  const steamPreview = $derived(storeAppId && !coverUrl && !coverSrc && !gridPick && !steamPreviewFailed
    ? `https://cdn.cloudflare.steamstatic.com/steam/apps/${storeAppId}/library_600x900.jpg` : '')
  let others = $state<{ deviceName: string; paths: Record<string, string | undefined>; exe: string }[]>([])
  let saving = $state(false)
  let error = $state('')
  let advanced = $state(false)
  const excludeHint = '*.log\nshadercache/**'

  onMount(async () => {
    tokens = (await LibraryService.Tokens(id)) ?? []
    hasGridKey = await LibraryService.HasSteamGridKey()
    if (!id) {
      slots = [{ key: 'saves', label: 'Saves', include: '', exclude: '', path: '', isNew: true }]
      if (steamAppId) await prefillFromSteamLibrary(steamAppId)
      return
    }
    try {
      const g = await LibraryService.Get(id)
      name = g.name
      coverUrl = g.coverUrl
      storeAppId = originalStoreAppId = g.storeAppId ?? ''
      others = (g.otherDevices ?? []) as any
      const d = g.device
      device = {
        paths: { ...(d.paths ?? {}) }, exe: d.exe, args: d.args, workDir: d.workDir,
        env: { ...(d.env ?? {}) }, waitProcess: d.waitProcess, launchMode: d.launchMode || ('direct' as any),
      }
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

  async function prefillFromSteamLibrary(appId: string) {
    const lib = (await LibraryService.SteamLibrary()) ?? []
    const a = lib.find((x) => x.appId === appId)
    if (!a) return
    name = a.name
    storeAppId = a.appId
    // A Steam-installed game is started by Steam; PlayRelay wraps it via launch options.
    device.launchMode = 'steamwrap' as any
    await suggestSaves(true)
  }

  // Looks up save locations in the Ludusavi manifest. With autofill, an empty
  // first location is filled when exactly one suggestion has files on disk.
  async function suggestSaves(autofill = false) {
    if (!name.trim() && !storeAppId) return
    suggesting = true
    try {
      suggest = await LibraryService.SuggestSaves({
        gameId: id, name, storeAppId, exe: device.exe, launchMode: device.launchMode,
      })
      const found = (suggest.suggestions ?? []).filter((x) => x.files > 0)
      if (autofill && found.length === 1 && slots.length && !slots[0].path) {
        useSuggestion(found[0], slots[0])
        app.toast(`Found ${suggest.matched} saves on this device`, 'ok')
      }
    } catch (e) {
      app.toast('Save suggestions: ' + errMsg(e), 'err')
    } finally {
      suggesting = false
    }
  }

  function useSuggestion(sg: SaveSuggestion, slot?: SlotForm) {
    const target = slot ?? slots.find((x) => !x.path)
    if (!target) return addSuggestion(sg)
    target.path = sg.path
    target.include = (sg.include ?? []).join('\n')
    target.exclude = ''
  }

  function addSuggestion(sg: SaveSuggestion) {
    const label = sg.path.split('/').filter(Boolean).pop() || 'Saves'
    let key = slug(label), base = key, i = 2
    while (slots.some((o) => o.key === key)) key = `${base}-${i++}`
    slots.push({ key, label, include: (sg.include ?? []).join('\n'), exclude: '', path: sg.path, isNew: true })
  }

  const kindLabel: Record<string, string> = {
    windows: '', native: 'Linux/macOS native', proton: 'Proton prefix',
    install: 'install folder', steamcloud: 'Steam Cloud folder',
  }

  async function searchSteam() {
    const term = name.trim()
    if (term.length < 2) return
    searching = true
    try {
      steamResults = (await LibraryService.SearchSteam(term)) ?? []
    } catch (e) {
      app.toast('Steam search failed: ' + errMsg(e), 'err')
      steamResults = null
    } finally {
      searching = false
    }
  }

  function pickSteam(m: SteamMatch) {
    name = m.name
    storeAppId = m.appId
    steamPreviewFailed = false
    steamResults = null
    suggestSaves(true)
  }

  async function refreshSteamCover() {
    try {
      const g = await LibraryService.FetchSteamCover(id)
      coverUrl = g.coverUrl
      coverSrc = ''
      app.toast('Cover updated from Steam', 'ok')
    } catch (e) {
      app.toast(errMsg(e), 'err')
    }
  }

  function unlinkSteam() {
    storeAppId = ''
    steamResults = null
  }

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
    if (p) {
      coverSrc = p
      gridPick = null
    }
  }

  function pickGrid(g: Grid) {
    gridPick = g
    coverSrc = ''
    pickingCover = false
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
        storeAppId,
      })
      if (coverSrc) {
        await LibraryService.SetCover(g.id, coverSrc)
      } else if (gridPick) {
        await LibraryService.SetCoverFromURL(g.id, gridPick.url).catch((e) => app.toast('Cover: ' + errMsg(e), 'err'))
      } else if (storeAppId && (!coverUrl || storeAppId !== originalStoreAppId)) {
        // Best effort: a missing cover must not block saving the game.
        await LibraryService.FetchSteamCover(g.id).catch((e) => app.toast('Cover: ' + errMsg(e), 'err'))
      }
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
        <label class="field"><span>Name</span>
          <div class="row nowrap">
            <input bind:value={name} placeholder="Game name" onkeydown={(e) => e.key === 'Enter' && searchSteam()} />
            <button onclick={searchSteam} disabled={searching || name.trim().length < 2} title="Look up the official title and cover on Steam">
              {searching ? 'Searching…' : 'Find on Steam'}
            </button>
          </div>
        </label>
        {#if steamResults}
          <div class="results">
            {#if steamResults.length === 0}<p class="small muted">No match on Steam. You can still add the game and choose a cover yourself.</p>{/if}
            {#each steamResults as m (m.appId)}
              <button class="ghost result" onclick={() => pickSteam(m)}>
                <span>{m.name}</span><span class="small muted mono">{m.appId}</span>
              </button>
            {/each}
            <button class="ghost small" onclick={() => (steamResults = null)}>Close</button>
          </div>
        {/if}
        {#if storeAppId}
          <div class="row small linked">
            <span class="badge info">Steam app {storeAppId}</span>
            <button class="ghost small" onclick={unlinkSteam}>Unlink</button>
          </div>
        {/if}
        <div class="row cover-row">
          <div class="cover-prev" style={coverUrl && !coverSrc ? `background-image:url('${coverUrl}')` : ''}>
            {#if coverSrc}<span class="small mono">{coverSrc.split(/[\\/]/).pop()}</span>
            {:else if gridPick}<img src={gridPick.thumb || gridPick.url} alt="" />
            {:else if steamPreview}<img src={steamPreview} alt="" onerror={() => (steamPreviewFailed = true)} />
            {:else if !coverUrl}<span class="muted small">No cover</span>{/if}
          </div>
          <div>
            <div class="row">
              <button onclick={pickCover}>Choose file…</button>
              {#if hasGridKey}<button onclick={() => (pickingCover = true)}>Find cover…</button>{/if}
            </div>
            {#if id && storeAppId && storeAppId === originalStoreAppId && coverUrl}
              <button class="ghost small" onclick={refreshSteamCover}>Use Steam cover</button>
            {/if}
            <p class="small muted">
              {#if storeAppId && !coverSrc && !coverUrl}Steam's cover is downloaded when you save, unless you choose your own.
              {:else if gridPick}SteamGridDB cover — downloaded when you save.
              {:else}Portrait image (600×900 works well). Also used as Steam grid art.{/if}
            </p>
            {#if !hasGridKey}<p class="small muted">Add a <a href="#/settings">SteamGridDB API key</a> to search community covers.</p>{/if}
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
        <div class="row"><h2>Save locations</h2><span class="spacer"></span>
          <button onclick={() => suggestSaves()} disabled={suggesting || (!name.trim() && !storeAppId)} title="Look up known save locations (Ludusavi manifest)">
            {suggesting ? 'Looking…' : 'Suggest'}
          </button>
          <button onclick={addSlot}>＋ Add</button>
        </div>
        {#if suggest}
          <div class="suggest">
            {#if !suggest.matched}
              <p class="small muted">No known save locations for “{name}”. Try “Find on Steam” first, or set the path yourself.</p>
            {:else}
              <div class="row small"><span class="muted">Known save locations for <strong>{suggest.matched}</strong></span><span class="spacer"></span>
                <button class="ghost small" onclick={() => (suggest = null)}>Hide</button></div>
              {#if (suggest.suggestions ?? []).length === 0}<p class="small muted">None that apply to this device.</p>{/if}
              {#each suggest.suggestions ?? [] as sg (sg.path)}
                <div class="sg" class:found={sg.files > 0}>
                  <div class="mono small path">{sg.path}{#if (sg.include ?? []).length}<span class="muted"> · {(sg.include ?? []).filter((x) => !x.endsWith('/**')).join(', ')}</span>{/if}</div>
                  <div class="row small">
                    {#if sg.files > 0}<span class="badge ok">found · {sg.files} files · {formatBytes(sg.bytes)} · {timeAgo(sg.newest)}</span>
                    {:else if sg.exists}<span class="badge">folder exists, no saves</span>
                    {:else}<span class="badge">not on this device</span>{/if}
                    {#if kindLabel[sg.kind]}<span class="badge">{kindLabel[sg.kind]}</span>{/if}
                    {#if sg.msStore}<span class="badge warn">Microsoft Store version</span>{/if}
                    <span class="spacer"></span>
                    <button class="small" onclick={() => useSuggestion(sg)}>Use</button>
                    <button class="ghost small" onclick={() => addSuggestion(sg)}>＋ Add</button>
                  </div>
                  {#if sg.kind === 'steamcloud'}<div class="small muted">Steam Cloud already syncs this for the Steam version; useful for non-Steam copies.</div>{/if}
                </div>
              {/each}
              <p class="small muted">From the <a href="https://github.com/mtkennerly/ludusavi-manifest" target="_blank" rel="noreferrer">Ludusavi manifest</a> (PCGamingWiki).</p>
            {/if}
          </div>
        {/if}
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

{#if pickingCover}
  <CoverPicker term={name} steamAppId={storeAppId} onpick={pickGrid} onclose={() => (pickingCover = false)} />
{/if}

<style>
  .layout { display: grid; grid-template-columns: 1fr 1fr; gap: 1em; align-items: start; }
  @media (max-width: 1000px) { .layout { grid-template-columns: 1fr; } }
  .nowrap { flex-wrap: nowrap; }
  .cover-row { align-items: flex-start; flex-wrap: nowrap; }
  .cover-prev { width: 80px; aspect-ratio: 2/3; border-radius: 8px; border: 1px solid var(--line); background: var(--bg) center/cover; display: grid; place-items: center; text-align: center; overflow: hidden; flex-shrink: 0; padding: 4px; word-break: break-all; }
  .slot { border-top: 1px solid var(--line); padding-top: .9em; margin-top: .9em; }
  .label { font-weight: 600; }
  .cover-prev img { margin: -4px; width: calc(100% + 8px); height: calc(100% + 8px); object-fit: cover; }
  .results { border: 1px solid var(--line); border-radius: 8px; padding: .3em; margin: -.4em 0 .9em; max-height: 260px; overflow: auto; }
  .result { width: 100%; justify-content: space-between; }
  .result:hover { background: var(--bg-3); }
  .linked { margin: -.4em 0 .9em; }
  .suggest { border: 1px solid var(--line); border-radius: 8px; padding: .6em .7em; margin-bottom: .6em; }
  .sg { border-top: 1px solid var(--line); padding: .5em 0; }
  .sg.found .path { color: var(--ok); }
  .sg .path { word-break: break-all; margin-bottom: .3em; }
  .filters { display: grid; grid-template-columns: 1fr 1fr; gap: .6em; margin-top: .5em; }
  .adv { margin-top: .8em; }
  details summary { cursor: pointer; margin-top: .3em; }
</style>
