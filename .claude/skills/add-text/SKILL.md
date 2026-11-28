---
name: add-text
description: Add or change user-visible text in Carryover with every translation in place, for the desktop UI (i18n.L/i18n.N), the Android UI (strings.xml) or a core message (internal/app/msg). Use whenever a change adds, rewords or removes text a user sees, a status/progress line, or an error a user can act on.
---

Languages: English (source), Spanish, Bahasa Indonesia, Brazilian Portuguese. Every new or changed text gets all four in the same change; write real translations, not English copies. Technical errors, the log and the CLI stay English and need none of this.

## 1. Pick the kind

- **Desktop UI text** (a page, dialog, button in `ui/`): go to section 2.
- **Android UI text** (a screen in `android/.../ui/` or `launch/`): go to section 3.
- **Core message**: text the Go core produces and either app shows (session/sync status, progress, an error people can act on, a hint, a preset's description). Go to section 4; it covers both apps.

Rewording English means changing the key everywhere it appears (desktop keys *are* the English text); grep for the old text in `ui/` and every catalog.

## 2. Desktop UI text

- Code: `i18n.L("English text")`; with values `i18n.L("Hi {{.Name}}", map[string]any{"Name": n})`; with a count `i18n.N("Synced {{.Count}} game", n)` (the key is whatever string you pass). Call it in the view, inside a function: never at package level, in a `var`/map/const, or in a `viewmodel.go` (view models hold no translated text; depguard enforces it).
- Catalogs `ui/i18n/translations/{en,es,id,pt-br}.json`, same key in each, kept in the file's existing order:
  - Plain: `"Key": "Text"` (en: the text equals the key).
  - Plural: `"Key": {"one": ..., "other": ...}` in en and id (Indonesian forms may be identical), `{"one": ..., "many": ..., "other": ...}` in es and pt-br.
  - Placeholders (`{{.Name}}`) must be the same set in every language.
- Tests: `go test ./ui/i18n/` (`TestTranslationCatalog` scans `ui/` for every `i18n.L/N` key, `TestLanguageCatalogsMatchEnglish`, `TestPluralsCoverEachLanguagesForms`).

## 3. Android UI text

- Code: `stringResource(R.string.x)` / `pluralStringResource(R.plurals.x, n, n)`; never a literal in `Text(...)` (`CatalogTest` checks `ui/` and `launch/`). View models get text through `Texts`, not `android.*`.
- Resources: `android/app/src/main/res/values/strings.xml` plus `values-es/`, `values-in/`, `values-pt-rBR/`, same name in each, in the matching section. Placeholders are positional `%1$s`, `%2$s`; escape apostrophes as `\'`; use `…` and `—` as the English does.
- Plurals: `one` + `other` in values and values-in, `one` + `many` + `other` in values-es and values-pt-rBR.
- Tests: from `android/`, `./gradlew testDebugUnitTest --tests '*CatalogTest*'` (Windows `.\gradlew.bat`).

## 4. Core message (internal/app/msg)

1. **Define** it in `internal/app/msg/catalog.go`, in the block for its topic, one line: `Name = define("topic.code", "English with {{.Arg}}")` or `plural("topic.code", "one form", "other form")` (the count placeholder is `{{.Count}}`, passed as `msg.CountArg`). Codes are stable (apps save and read them): never rename or reuse one; a changed meaning gets a new code.
2. **Use** it: `msg.X.Msg("Arg", v)` for status, `msg.X.Err(...)` / `msg.X.Wrap(err, ...)` for errors instead of `errors.New`/`fmt.Errorf`. A view struct field carrying such text gets a `...Msg` field next to it, set and cleared together. Desktop shows it with `i18n.Msg/Text/Err`, Android with `Texts.core/error`/`coreText`; never `err.Error()` or `e.message`.
3. **Desktop catalogs**: add to the four `ui/i18n/translations/*.json` with the English *other* text as the key: `"English with {{.Arg}}": "..."`, or for a plural `"<other text>": {"one": "<one text>", "other": "<other text>"}` (plus `many` in es/pt-br, per section 2).
4. **Android**: in `android/app/src/main/java/com/akfaiz/carryover/data/CoreMessages.kt` add, sorted by code like the rest of the map, `"topic.code" to CoreString.Text(R.string.core_topic_code, "Arg1", "Arg2")` (or `CoreString.Plural(R.plurals.core_topic_code, "Count", ...)`), the placeholder names in order of first appearance in the English. Then in each `strings.xml` (in the `core_*` block, sorted by name) add `core_topic_code` with `{{.Arg1}}` → `%1$s`, `{{.Arg2}}` → `%2$s`, plurals with the forms from section 3.
5. **Tests**: `go test ./internal/app/msg/ ./ui/i18n/` (`TestAndroidCatalog` checks CoreMessages.kt and the English resources against the catalog; `TestCoreMessagesAreInTheCatalog` the desktop's en.json), plus the Android `CatalogTest` for the other languages.

## 5. Finish

- Run the tests named above; on a mismatch, fix the catalog rather than the test.
- The English follows the README's plain style: short, concrete, no jargon (say "saves", "cloud", "this device").
- In the summary, list the keys/codes added or changed and the files touched, and flag any translation you are unsure of so the user can check it.
