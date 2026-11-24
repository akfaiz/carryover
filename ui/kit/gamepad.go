package kit

import "github.com/egoist/mygo/plugins/game/gamepad"

// stickThreshold is how far the left stick must be pushed to count as a
// direction; scrollDeadZone is how far the right stick must be pushed to
// scroll.
const (
	stickThreshold = 0.6
	scrollDeadZone = 0.25
)

// MygoPad reads the first connected gamepad through MyGo's gamepad package
// (XInput on Windows, evdev on Linux, GameController on macOS; Steam Input's
// virtual pad shows up as one of them). It may be called from any goroutine.
func MygoPad() (PadState, bool) {
	pads := gamepad.Pads()
	if len(pads) == 0 {
		return PadState{}, false
	}
	return padStateOf(pads[0].State()), true
}

func padStateOf(s gamepad.State) PadState {
	x, y := s.Axis(gamepad.LeftX), s.Axis(gamepad.LeftY)
	scroll := s.Axis(gamepad.RightY)
	if scroll > -scrollDeadZone && scroll < scrollDeadZone {
		scroll = 0
	}
	return PadState{
		Up:     s.Pressed(gamepad.Up) || y < -stickThreshold,
		Down:   s.Pressed(gamepad.Down) || y > stickThreshold,
		Left:   s.Pressed(gamepad.Left) || x < -stickThreshold,
		Right:  s.Pressed(gamepad.Right) || x > stickThreshold,
		Accept: s.Pressed(gamepad.South),
		Back:   s.Pressed(gamepad.East),
		X:      s.Pressed(gamepad.West),
		Y:      s.Pressed(gamepad.North),
		View:   s.Pressed(gamepad.Back),
		LB:     s.Pressed(gamepad.LeftShoulder),
		RB:     s.Pressed(gamepad.RightShoulder),
		Scroll: scroll,
	}
}
