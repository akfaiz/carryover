export {
  CloudService,
  LibraryService,
  SessionService,
  SteamService,
  SyncService,
} from '../../bindings/github.com/akfaiz/playrelay/services'
export type {
  GameView,
  GameInput,
  DeviceInput,
  SteamLibraryGame,
  SaveSuggestion,
  SuggestResult,
  Prompt,
  Status,
  PlanView,
  SteamInfo,
  WrapperState,
} from '../../bindings/github.com/akfaiz/playrelay/services'
export type { Settings } from '../../bindings/github.com/akfaiz/playrelay/internal/appdir/models'
export type { Provider, ProviderOption, Question, Remote } from '../../bindings/github.com/akfaiz/playrelay/internal/cloud/models'
export type { SaveSlot, DeviceConfig } from '../../bindings/github.com/akfaiz/playrelay/internal/library/models'
export type { Game as GridGame, Grid } from '../../bindings/github.com/akfaiz/playrelay/internal/steamgrid/models'
export type { Match as SteamMatch } from '../../bindings/github.com/akfaiz/playrelay/internal/steamstore/models'
export type { Token } from '../../bindings/github.com/akfaiz/playrelay/internal/pathvar/models'
export type { Backup, Event as SyncEvent, Summary } from '../../bindings/github.com/akfaiz/playrelay/internal/syncer/models'
export * as Ev from '../../bindings/events'

export function errMsg(e: unknown): string {
  if (!e) return 'Unknown error'
  if (typeof e === 'string') return e
  const anyE = e as any
  return anyE.message ?? anyE.error ?? String(e)
}

export function formatBytes(n: number): string {
  if (!n) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB']
  let i = 0
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++ }
  return `${n.toFixed(i ? 1 : 0)} ${u[i]}`
}

export function timeAgo(iso: string | undefined | null): string {
  if (!iso || iso.startsWith('0001')) return 'never'
  const t = new Date(iso).getTime()
  if (isNaN(t)) return 'never'
  const s = Math.round((Date.now() - t) / 1000)
  if (s < 60) return 'just now'
  if (s < 3600) return `${Math.round(s / 60)} min ago`
  if (s < 86400) return `${Math.round(s / 3600)} h ago`
  return new Date(t).toLocaleDateString()
}

export function fmtDate(iso: string | undefined | null): string {
  if (!iso || iso.startsWith('0001')) return '—'
  const d = new Date(iso)
  return isNaN(d.getTime()) ? '—' : d.toLocaleString()
}
