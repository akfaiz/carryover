package kit

import "github.com/egoist/mygo/ui"

// Icons are 24x24 outline SVGs drawn in the current text color: draw one with
// ui.Icon(c, kit.IconX).FontSize(kit.IconMD). They stand in for the glyphs and
// emoji the pages used before.

// outline wraps shapes in a 24x24 stroked icon.
func outline(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	// IconPlay is a play triangle: the game itself, no emulator.
	IconPlay = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M7 4.5v15l12-7.5z"/></svg>`))
	// IconDownload is an arrow into a tray: a game installed by another launcher.
	IconDownload = outline(`<path d="M12 4v11M7 11l5 5 5-5M5 20h14"/>`)
	// IconUpload is an arrow out of a tray.
	IconUpload = outline(`<path d="M12 16V5M7 9l5-5 5 5M5 20h14"/>`)
	// IconStorage is a stack of drives.
	IconStorage = outline(`<rect x="3" y="4" width="18" height="6" rx="1"/><rect x="3" y="14" width="18" height="6" rx="1"/><path d="M7 7h.01M7 17h.01"/>`)
	// IconFolder is a folder.
	IconFolder = outline(`<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>`)
	// IconFolderOpen is an open folder.
	IconFolderOpen = outline(`<path d="M3 18V6a1 1 0 0 1 1-1h5l2 2h8a1 1 0 0 1 1 1v2"/><path d="M3 18l2.5-8h16L19 18z"/>`)
	// IconComputer is a monitor on a stand: an emulator.
	IconComputer = outline(`<rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/>`)

	// IconLibrary is a grid of four tiles.
	IconLibrary = outline(`<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/>`)
	// IconGamepad is a game controller.
	IconGamepad = outline(`<path d="M6 11h4M8 9v4M15 12h.01M18 10h.01"/><path d="M17.3 5H6.7a4 4 0 0 0-3.9 3.2l-1.5 7A3 3 0 0 0 4.2 19c.9 0 1.6-.4 2.2-1.1L8 16h8l1.6 1.9c.6.7 1.3 1.1 2.2 1.1a3 3 0 0 0 2.9-3.8l-1.5-7A4 4 0 0 0 17.3 5z"/>`)
	// IconCloud is a cloud.
	IconCloud = outline(`<path d="M17.5 19H9a7 7 0 1 1 6.7-9h1.8a4.5 4.5 0 1 1 0 9z"/>`)
	// IconTrash is a bin.
	IconTrash = outline(`<path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14M10 11v5M14 11v5"/>`)
	// IconSettings is a set of sliders.
	IconSettings = outline(`<path d="M4 6h8M18 6h2M4 12h2M12 12h8M4 18h10M20 18h0"/><circle cx="15" cy="6" r="2"/><circle cx="9" cy="12" r="2"/><circle cx="17" cy="18" r="2"/>`)
	// IconArrowLeft points left: back.
	IconArrowLeft = outline(`<path d="M19 12H5M11 6l-6 6 6 6"/>`)
	// IconRefresh is a circular arrow: sync, reload.
	IconRefresh = outline(`<path d="M20 12a8 8 0 1 1-2.3-5.7M20 4v5h-5"/>`)
	// IconPencil is a pencil: edit.
	IconPencil = outline(`<path d="M4 20h4L19 9l-4-4L4 16zM13.5 6.5l4 4"/>`)
	// IconMore is three dots: more actions.
	IconMore = outline(`<path d="M5 12h.01M12 12h.01M19 12h.01" stroke-width="3"/>`)
	// IconPlus is a plus: add.
	IconPlus = outline(`<path d="M12 5v14M5 12h14"/>`)
	// IconCheck is a tick.
	IconCheck = outline(`<path d="M5 12.5l4.5 4.5L19 7.5"/>`)
	// IconAlert is a warning triangle.
	IconAlert = outline(`<path d="M12 4l9 16H3zM12 10v4M12 17h.01"/>`)
	// IconInfo is an "i" in a circle: information.
	IconInfo = outline(`<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>`)
	// IconX is a cross: close, remove, failed.
	IconX = outline(`<path d="M6 6l12 12M18 6L6 18"/>`)
	// IconChevronDown points down: a drop-down, an expander.
	IconChevronDown = outline(`<path d="M6 9l6 6 6-6"/>`)
	// IconChevronUp points up: fold an expanded section.
	IconChevronUp = outline(`<path d="M6 15l6-6 6 6"/>`)
	// IconChevronRight points right: a collapsed section.
	IconChevronRight = outline(`<path d="M9 6l6 6-6 6"/>`)
	// IconPin is a pin: keep a backup.
	IconPin = outline(`<path d="M9 4h6l-1 6 3 3H7l3-3zM12 13v7"/>`)
	// IconSearch is a magnifier.
	IconSearch = outline(`<circle cx="11" cy="11" r="6"/><path d="M20 20l-4.5-4.5"/>`)
	// IconRestore is a clock with a back arrow: restore a backup.
	IconRestore = outline(`<path d="M4 12a8 8 0 1 0 2.3-5.7M4 4v5h5M12 8v4l3 2"/>`)
	// IconExit is a door with an arrow: quit.
	IconExit = outline(`<path d="M10 5H5v14h5M15 8l4 4-4 4M19 12H9"/>`)
	// IconExternal is an arrow out of a box: opens elsewhere.
	IconExternal = outline(`<path d="M14 4h6v6M20 4l-9 9M18 14v5H5V6h5"/>`)
	// IconDot is a filled dot: something is running.
	IconDot = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><circle cx="12" cy="12" r="5"/></svg>`))
)
