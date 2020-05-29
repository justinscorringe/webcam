package webcam

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io/ioutil"
	"time"

	"github.com/disintegration/imaging"
	rgblib "github.com/pixiv/go-libjpeg/rgb"
	"github.com/pkg/errors"
)

var formats map[string]func([]byte, string, uint32, uint32) (image.Image, error)

// TODO: When more formats are supported, split by ratio ie; 4:2:2 / 4:1:1
var packedYUV = []string{"YUYV", "YVYU", "UYVY", "VYUY"}
var planarYUV = []string{"YU12", "YV12", "NV12", "NV21"}
var rgb = []string{"RGB3", "BGR3"}
var rgba = []string{"RGB4", "BGR4"}

// Conversion of raw image formats to compressed jpegs
// Conversion is categorised by a string 4CC code for code readibility
func Compress(frame []byte, format string, width uint32, height uint32, quality uint32, rotation string, rwidth int, rheight int) ([]byte, string, error) {
	// Check we actually support this format
	if _, ok := formats[format]; !ok {
		if format == "JPEG" || format == "MJPG" {
			return frame, fmt.Sprintf("hardware compressed %s of length %v; resolution %v x %v", format, len(frame), width, height), nil
		}
		return nil, "error encoding", fmt.Errorf("format %v is not supported by this encoder", format)
	}
	// Make sure the input values are sane
	if width <= 10 || height <= 10 || len(frame) <= 10 {
		return nil, "error encoding", errors.New("input error")
	}
	// Record time taken to encode image
	start := time.Now()
	// Encode our image
	encoder := formats[format]
	decodedImage, err := encoder(frame, format, width, height)
	if err != nil {
		return nil, "error encoding", err
	}
	// Rotate
	decodedImage = rotateImage(decodedImage, rotation)

	//Resize
	if rwidth != 0 {
		// If height is 0, aspect ratio will be maintained
		decodedImage = resizeImage(decodedImage, rwidth, rheight, quality)
	}

	// Compress to jpeg
	compressedImage, err := encodeJPEG(decodedImage, quality)
	if err != nil {
		return nil, "error compressing", err
	}
	encoderMsg := fmt.Sprintf("Encoded image format %s; length %v; resolution %v x %v; to jpeg of length %v in %s", format, len(frame), width, height, len(compressedImage), time.Since(start))
	return compressedImage, encoderMsg, nil
}

// YUV 4:2:2 decoder. Supports YUYV, YVYU, UYVY, VYUY, YUNV.
func decodePackedYUV(frame []byte, f string, width uint32, height uint32) (image.Image, error) {

	yuyv := image.NewYCbCr(image.Rect(0, 0, int(width), int(height)), image.YCbCrSubsampleRatio422)
	for i := range yuyv.Cb {
		ii := i * 4
		switch f {
		// Copy luma and chroma planes in format specific order
		case "YUYV":
			yuyv.Y[i*2] = frame[ii]
			yuyv.Y[i*2+1] = frame[ii+2]
			yuyv.Cb[i] = frame[ii+1]
			yuyv.Cr[i] = frame[ii+3]
		case "YVYU":
			yuyv.Y[i*2] = frame[ii]
			yuyv.Y[i*2+1] = frame[ii+2]
			yuyv.Cb[i] = frame[ii+3]
			yuyv.Cr[i] = frame[ii+1]
		case "YUNV":
			yuyv.Y[i*2] = frame[ii]
			yuyv.Y[i*2+1] = frame[ii+2]
			yuyv.Cb[i] = frame[ii+1]
			yuyv.Cr[i] = frame[ii+3]
		case "VYUY":
			yuyv.Y[i*2] = frame[ii+1]
			yuyv.Y[i*2+1] = frame[ii+3]
			yuyv.Cb[i] = frame[ii+2]
			yuyv.Cr[i] = frame[ii]
		case "UYVY":
			yuyv.Y[i*2] = frame[ii+1]
			yuyv.Y[i*2+1] = frame[ii+3]
			yuyv.Cb[i] = frame[ii]
			yuyv.Cr[i] = frame[ii+2]
		}
	}

	return yuyv, nil
}

// YUV 4:2:0 decoder. Supports YU12, YV12, I420, NV12, NV21
func decodePlanarYUV(frame []byte, f string, width uint32, height uint32) (image.Image, error) {

	yuv := image.NewYCbCr(image.Rect(0, 0, int(width), int(height)), image.YCbCrSubsampleRatio420)
	// Copy luma plane
	for i := range yuv.Y {
		yuv.Y[i] = frame[i]
	}
	// Copy chroma planes in format specific order
	switch f {
	case "YU12":
		for i := range yuv.Cr {
			yuv.Cb[i] = frame[i+len(yuv.Y)]
		}
		for i := range yuv.Cb {
			yuv.Cr[i] = frame[i+len(yuv.Y)+len(yuv.Cr)]
		}
	case "YV12":
		for i := range yuv.Cb {
			yuv.Cr[i] = frame[i+len(yuv.Y)]
		}
		for i := range yuv.Cr {
			yuv.Cb[i] = frame[i+len(yuv.Y)+len(yuv.Cr)]
		}
	case "I420":
		for i := range yuv.Cb {
			yuv.Cb[i] = frame[i+len(yuv.Y)]
		}
		for i := range yuv.Cr {
			yuv.Cr[i] = frame[i+len(yuv.Y)+len(yuv.Cr)]
		}
	case "NV12":
		for i := range yuv.Cr {
			ii := 2 * i
			yuv.Cb[i] = frame[len(yuv.Y)+ii]
			yuv.Cr[i] = frame[len(yuv.Y)+ii+1]
		}
	case "NV21":
		for i := range yuv.Cr {
			ii := 2 * i
			yuv.Cb[i] = frame[len(yuv.Y)+ii+1]
			yuv.Cr[i] = frame[len(yuv.Y)+ii]
		}
	}
	return yuv, nil
}

// RGB decoder, it supports RGB3, BGR3.
func decodeRGB(frame []byte, f string, width uint32, height uint32) (image.Image, error) {

	rgb := rgblib.NewImage(image.Rect(0, 0, int(width), int(height)))
	for i := range frame {
		if i%3 == 0 {
			switch f {
			case "RGB3":
				rgb.Pix[i] = frame[i]
				rgb.Pix[i+1] = frame[i+1]
				rgb.Pix[i+2] = frame[i+2]
			case "BGR3":
				rgb.Pix[i] = frame[i+2]
				rgb.Pix[i+1] = frame[i+1]
				rgb.Pix[i+2] = frame[i]
			}
		}
	}
	return rgb, nil
}

// This is our RGBA decoder, it supports RGB4 and BGR4.
func decodeRGBA(frame []byte, f string, width uint32, height uint32) (image.Image, error) {

	rgba := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	rgbabuf := make([]uint8, 4*int(width)*int(height))
	for i := range frame {
		if i%4 == 0 {
			switch f {
			case "RGB4":
				rgbabuf[i] = frame[i+2]
				rgbabuf[i+1] = frame[i+1]
				rgbabuf[i+2] = frame[i]
				rgbabuf[i+3] = frame[i+3]
			case "BGR4":
				rgbabuf[i] = frame[i]
				rgbabuf[i+1] = frame[i+1]
				rgbabuf[i+2] = frame[i+2]
				rgbabuf[i+3] = frame[i+3]
			}
		}
	}
	rgba.Pix = rgbabuf
	rgba.Stride = 4 * int(width)
	return rgba, nil
}

// Rotates the image based on int argument (90, 180, 270)
func rotateImage(img image.Image, rotation string) image.Image {

	switch rotation {
	case "90", "90CW", "90cw", "270ccw", "270CCW":
		img = imaging.Rotate90(img)
	case "180", "180cw", "180CW", "180ccw", "180CCW":
		img = imaging.Rotate180(img)
	case "270", "270CW", "270cw", "90ccw", "90CCW":
		img = imaging.Rotate270(img)
	default:
	}
	return img
}

// Resize the image to a given width and height, quality discerning resize technique
func resizeImage(img image.Image, width int, height int, quality uint32) image.Image {
	if quality >= 75 {
		img = imaging.Resize(img, width, height, imaging.Lanczos)
	} else if quality >= 50 {
		img = imaging.Resize(img, width, height, imaging.CatmullRom)
	} else {
		img = imaging.Resize(img, width, height, imaging.Box)
	}
	return img
}

// Encodes our golang image.Image into a compressed JPEG byte array
func encodeJPEG(img image.Image, quality uint32) ([]byte, error) {
	buf := &bytes.Buffer{}
	compression := jpeg.Options{Quality: int(quality)}
	if err := jpeg.Encode(buf, img, &compression); err != nil {
		return nil, err
	}
	readBuf, _ := ioutil.ReadAll(buf)
	return readBuf, nil
}

// Interface to check if format is supported
func CompressionAvailable(format string) bool {
	if _, ok := formats[format]; ok {
		return true
	}
	return false
}

// Declare our library of format types upon initialization
func init() {
	formats = make(map[string]func([]byte, string, uint32, uint32) (image.Image, error))
	for _, format := range packedYUV {
		formats[format] = decodePackedYUV
	}
	for _, format := range planarYUV {
		formats[format] = decodePlanarYUV
	}
	for _, format := range rgb {
		formats[format] = decodeRGB
	}
	for _, format := range rgba {
		formats[format] = decodeRGBA
	}
}
