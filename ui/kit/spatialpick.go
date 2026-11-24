package kit

import "math"

// The geometry of directional navigation, shared by every GUI toolkit.

// navRect is an object's place on the canvas.
type navRect struct{ X, Y, W, H float32 }

func (r navRect) cx() float32 { return r.X + r.W/2 }
func (r navRect) cy() float32 { return r.Y + r.H/2 }

func pickInDirection(from navRect, dir PadAction, cands []navRect) int {
	best, bestScore := -1, float32(math.MaxFloat32)
	for i, c := range cands {
		if c == from {
			continue
		}
		var gap, side float32 // distance along the direction, and across it
		switch dir {
		case PadDown:
			if c.cy() <= from.cy() || c.Y < from.Y+from.H-min(from.H, c.H)/2 {
				continue
			}
			gap, side = max(0, c.Y-(from.Y+from.H)), spanGap(from.X, from.W, c.X, c.W)
		case PadUp:
			if c.cy() >= from.cy() || c.Y+c.H > from.Y+min(from.H, c.H)/2 {
				continue
			}
			gap, side = max(0, from.Y-(c.Y+c.H)), spanGap(from.X, from.W, c.X, c.W)
		case PadRight:
			if c.cx() <= from.cx() || c.X < from.X+from.W-min(from.W, c.W)/2 {
				continue
			}
			gap, side = max(0, c.X-(from.X+from.W)), spanGap(from.Y, from.H, c.Y, c.H)
		case PadLeft:
			if c.cx() >= from.cx() || c.X+c.W > from.X+min(from.W, c.W)/2 {
				continue
			}
			gap, side = max(0, from.X-(c.X+c.W)), spanGap(from.Y, from.H, c.Y, c.H)
		default:
			return -1
		}
		// Among equally good ones, the one whose middle lines up best.
		align := float32(math.Abs(float64(c.cx() - from.cx())))
		if dir == PadLeft || dir == PadRight {
			align = float32(math.Abs(float64(c.cy() - from.cy())))
		}
		if s := gap + 2*side + align/100; s < bestScore {
			best, bestScore = i, s
		}
	}
	return best
}

// spanGap is the distance between two spans on one axis, 0 if they overlap.
func spanGap(a, aLen, b, bLen float32) float32 {
	switch {
	case b > a+aLen:
		return b - (a + aLen)
	case a > b+bLen:
		return a - (b + bLen)
	}
	return 0
}
