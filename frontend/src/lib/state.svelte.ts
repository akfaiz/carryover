import { Events } from '@wailsio/runtime'
import { Ev, LibraryService, SessionService, SyncService, errMsg } from './api'
import type { GameView, Prompt, Settings, Status, SyncEvent } from './api'

type Toast = { id: number; text: string; kind: 'info' | 'ok' | 'err' }

class AppState {
  games = $state<GameView[]>([])
  settings = $state<Settings | null>(null)
  prompts = $state<Prompt[]>([])
  status = $state<Record<string, Status>>({})
  progress = $state<Record<string, SyncEvent>>({})
  toasts = $state<Toast[]>([])
  route = $state(location.hash.slice(1) || '/')
  loaded = $state(false)
  private toastId = 0
  private started = false

  start() {
    if (this.started) return
    this.started = true
    window.addEventListener('hashchange', () => (this.route = location.hash.slice(1) || '/'))
    Events.On(Ev.LibraryChanged, () => this.refresh())
    Events.On(Ev.SessionPrompt, (e: any) => {
      const p = e.data as Prompt
      if (!this.prompts.find((x) => x.id === p.id)) this.prompts = [...this.prompts, p]
    })
    Events.On(Ev.SessionStatus, (e: any) => {
      const s = e.data as Status
      this.status = { ...this.status, [s.gameId]: s }
      if (s.phase === 'error') this.toast(s.message, 'err')
    })
    Events.On(Ev.SyncProgress, (e: any) => {
      const ev = e.data as SyncEvent
      this.progress = { ...this.progress, [ev.gameId]: ev }
    })
    SessionService.Prompts().then((p) => (this.prompts = p ?? []))
    this.refresh()
  }

  async refresh() {
    try {
      const [games, settings] = await Promise.all([LibraryService.List(), SyncService.GetSettings()])
      this.games = games ?? []
      this.settings = settings
    } catch (e) {
      this.toast(errMsg(e), 'err')
    } finally {
      this.loaded = true
    }
  }

  game(id: string) {
    return this.games.find((g) => g.id === id)
  }

  async respond(p: Prompt, answer: string) {
    this.prompts = this.prompts.filter((x) => x.id !== p.id)
    await SessionService.Respond(p.id, answer)
  }

  toast(text: string, kind: Toast['kind'] = 'info') {
    const id = ++this.toastId
    this.toasts = [...this.toasts, { id, text, kind }]
    setTimeout(() => (this.toasts = this.toasts.filter((t) => t.id !== id)), kind === 'err' ? 7000 : 3500)
  }
}

export const app = new AppState()

export function go(path: string) {
  location.hash = path
}
