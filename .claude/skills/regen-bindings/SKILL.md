---
name: regen-bindings
description: Regenerate Wails TypeScript bindings and typecheck the Svelte frontend. Use after changing exported methods or returned types in services/ or internal models exposed to the UI.
---

Run from the repo root:

1. `go build ./...` — fix Go errors first (if it fails on `frontend/dist`, create `frontend/dist/index.html` placeholder).
2. `"$(go env GOPATH)/bin/wails3" generate bindings -clean=true -ts -i` (same flags as `wails3 build`)
   - Treat any `[warn]` line as a problem to fix (e.g. a channel/func type on an exported service method — move it to a package-level function).
3. `cd frontend && npm run check && npm run lint` — fix every TypeScript/Svelte error caused by changed signatures or models (imports come from `frontend/src/lib/api.ts`; add new model types there).
4. Report: services/methods/models counts from the generator output and the svelte-check result.

Never hand-edit files under `frontend/bindings/`.
