// Package kit holds Carryover's reusable Fyne building blocks: the palette and
// theme, cards, badges, buttons, rows and layouts, the toaster, the image
// cache, progress and tile widgets, and helpers for work on the UI thread.
//
// It is pure Fyne. It knows nothing about games, services or the core, and it
// contains no user-visible text of its own: callers pass every label in, already
// translated, so the translation catalog stays in package ui. Being under
// internal/, it can export freely without becoming part of Carryover's public
// API.
package kit
