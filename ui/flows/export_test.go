package flows

import "github.com/akfaiz/carryover/ui/env"

// ImportRow is a game of the import dialog, for tests that give it games.
type ImportRow = importRow

// NewImportRow returns a game of the import dialog.
func NewImportRow(name, gameID, route string) ImportRow {
	return importRow{name: name, gameID: gameID, route: route}
}

// ShowImportRows opens the import dialog with the given games.
func ShowImportRows(u env.Env, rows []ImportRow) {
	showImport(u, importText{title: "Import test", intro: "Games.", reading: "Reading…", empty: "None."},
		func() []importRow { return rows })
}
