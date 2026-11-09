<script lang="ts">
  import { LibraryService } from '../lib/api'
  import type { Token } from '../lib/api'

  let {
    value = $bindable(''),
    gameId = '',
    tokens = [],
    kind = 'folder',
    placeholder = '',
  }: { value: string; gameId?: string; tokens?: Token[]; kind?: 'folder' | 'file'; placeholder?: string } = $props()

  let preview = $state('')
  let previewErr = $state('')
  let menu = $state(false)
  let input: HTMLInputElement

  $effect(() => {
    const v = value
    if (!v) { preview = ''; previewErr = ''; return }
    const t = setTimeout(async () => {
      try {
        preview = await LibraryService.PreviewPath(gameId, v)
        previewErr = ''
      } catch (e: any) {
        preview = ''
        previewErr = e?.message ?? String(e)
      }
    }, 250)
    return () => clearTimeout(t)
  })

  async function browse() {
    const p = kind === 'folder'
      ? await LibraryService.PickFolder('Choose folder')
      : await LibraryService.PickFile('Choose file', '', '')
    if (p) value = await LibraryService.ContractPath(p)
  }

  function insert(tok: Token) {
    menu = false
    let t = tok.token
    if (t.includes('<appid>')) t = t.replace('<appid>', '')
    const start = input?.selectionStart ?? value.length
    value = value.slice(0, start) + t + value.slice(input?.selectionEnd ?? start)
    queueMicrotask(() => {
      input?.focus()
      const pos = t.endsWith(':}') ? start + t.length - 1 : start + t.length
      input?.setSelectionRange(pos, pos)
    })
  }
</script>

<div class="path">
  <div class="row nowrap">
    <input bind:this={input} bind:value {placeholder} class="mono" spellcheck="false" />
    {#if tokens.length}
      <div class="tok">
        <button type="button" title="Insert path variable" onclick={() => (menu = !menu)}>{'{ }'}</button>
        {#if menu}
          <div class="menu">
            {#each tokens as t (t.token)}
              <button type="button" class="ghost item" onclick={() => insert(t)}>
                <span class="mono">{t.token}</span>
                <span class="small muted">{t.value || t.description}</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
    <button type="button" onclick={browse}>Browse…</button>
  </div>
  {#if previewErr}
    <div class="small err">{previewErr}</div>
  {:else if preview && preview !== value}
    <div class="small muted mono">→ {preview}</div>
  {/if}
</div>

<style>
  .nowrap { flex-wrap: nowrap; }
  .tok { position: relative; }
  .menu {
    position: absolute; right: 0; top: 110%; z-index: 20; background: var(--bg-3); border: 1px solid var(--line);
    border-radius: 8px; padding: .3em; width: 420px; max-height: 320px; overflow: auto; box-shadow: 0 10px 30px #000a;
  }
  .item { flex-direction: column; align-items: flex-start; width: 100%; text-align: left; white-space: normal; gap: .1em; }
  .item:hover { background: var(--bg-2); }
  .small { margin-top: .3em; word-break: break-all; }
</style>
