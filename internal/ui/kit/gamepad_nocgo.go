//go:build !cgo

package kit

// glfwPad reports no gamepad: GLFW (and so Fyne's desktop driver) needs cgo,
// so this only builds for type-checking without it.
func glfwPad() (PadState, bool) { return PadState{}, false }
