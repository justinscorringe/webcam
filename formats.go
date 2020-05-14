package webcam

import "fmt"

// Represents image format code used by V4L2 subsystem.
type PixelFormat uint32
// Struct that describes frame size supported by a webcam
// For fixed sizes min and max values will be the same and
// step value will be equal to '0'
type FrameSize struct {
	MinWidth  uint32
	MaxWidth  uint32
	StepWidth uint32

	MinHeight  uint32
	MaxHeight  uint32
	StepHeight uint32
}

// Returns string representation of frame size, e.g.
// 1280x720 for fixed-size frames and
// [320-640;160]x[240-480;160] for stepwise-sized frames
func (s FrameSize) GetString() string {
	if s.StepWidth == 0 && s.StepHeight == 0 {
		return fmt.Sprintf("%dx%d", s.MaxWidth, s.MaxHeight)
	} else {
		return fmt.Sprintf("[%d-%d;%d]x[%d-%d;%d]", s.MinWidth, s.MaxWidth, s.StepWidth, s.MinHeight, s.MaxHeight, s.StepHeight)
	}
}

// Functions allow the conversion of PixelFormats to and from human readable 4CC strings
// ie; "YUYV" to 0x55595659 and vice versa
func EncodeFormat(value string) PixelFormat{

	var a byte = ' '
	var b byte = ' '
	var c byte = ' '
	var d byte = ' '
	{
		length := len(value)

		if 1 <= length  {
			a = byte(value[0])
		}
		if 2 <= length  {
			b = byte(value[1])
		}
		if 3 <= length  {
			c = byte(value[2])
		}
		if 4 <= length  {
			d = byte(value[3])
		}
	}
	var code uint32

	code =  uint32(a)        |
	       (uint32(b) <<  8) |
	       (uint32(c) << 16) |
	       (uint32(d) << 24)

	return PixelFormat(code)
}
func DecodeFormat(format PixelFormat) string {
	a := byte((uint32(format)      ) & 0xff)
	b := byte((uint32(format) >>  8) & 0xff)
	c := byte((uint32(format) >> 16) & 0xff)
	d := byte((uint32(format) >> 24) & 0xff)

	return fmt.Sprintf("%c%c%c%c", a, b, c, d)
}