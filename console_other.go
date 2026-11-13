//go:build !windows

package main

func attachConsole() {}

// showFatal does nothing: the startup error is already on stderr.
func showFatal(string) {}
