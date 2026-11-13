//go:build cgo

package kit

import "github.com/go-gl/glfw/v3.4/glfw"

// stickThreshold is how far the left stick must be pushed to count as a
// direction.
const stickThreshold = 0.6

// glfwPad reads the first connected gamepad through GLFW, which Fyne already
// runs: Steam Input presents its virtual controller to the app as one. It
// must be called on the main thread (see Nav.startPad).
func glfwPad() (PadState, bool) {
	for j := glfw.Joystick1; j <= glfw.JoystickLast; j++ {
		if !j.Present() || !j.IsGamepad() {
			continue
		}
		gs := j.GetGamepadState()
		if gs == nil {
			continue
		}
		pressed := func(b glfw.GamepadButton) bool { return gs.Buttons[b] == glfw.Press }
		x, y := gs.Axes[glfw.AxisLeftX], gs.Axes[glfw.AxisLeftY]
		return PadState{
			Up:     pressed(glfw.ButtonDpadUp) || y < -stickThreshold,
			Down:   pressed(glfw.ButtonDpadDown) || y > stickThreshold,
			Left:   pressed(glfw.ButtonDpadLeft) || x < -stickThreshold,
			Right:  pressed(glfw.ButtonDpadRight) || x > stickThreshold,
			Accept: pressed(glfw.ButtonA),
			Back:   pressed(glfw.ButtonB),
		}, true
	}
	return PadState{}, false
}
