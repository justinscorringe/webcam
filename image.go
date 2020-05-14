package webcam

import (
	"fmt"
	"time"
	"bytes"
	"image"
	"image/jpeg"
	"io/ioutil"
	"github.com/pkg/errors"

)

var formats map[string]func([]byte, string, uint32, uint32)([]byte, error)
// TODO: When more formats are supported, split by ratio ie; 4:2:2 / 4:1:1
var packedYUV = []string{"YUYV", "YVYU", "UYVY", "VYUY"}
var planarYUV = []string{"YU12", "YV12", "NV12", "NV21"}
var rgb = []string{"RGB3", "BGR3"}
var rgba = []string{"RGB4", "BGR4"}

// Conversion of raw image formats to compressed jpegs
// Conversion is categorised by a string 4CC code for code readibility
func Compress(frame []byte, format string, width uint32, height uint32) ([]byte, string, error) {
	// Check we actually support this format
	if _, ok := formats[format]; !ok {
		if format == "JPEG" || format == "MJPG" {
			return frame, "already compressed", nil
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
	jpegFrame, err := encoder(frame, format, width, height)
	if err != nil {
		return nil, "error encoding", err
	}
	encoderMsg := fmt.Sprintf("Encoded image format %s of length %v and resolution %v x %v \n jpeg of length %v in %s", format, len(frame), width, height, len(jpegFrame), time.Since(start))
	return jpegFrame, encoderMsg, nil
}

// YUV 4:2:2 decoder. Supports YUYV, YVYU, UYVY, VYUY, YUNV.
func decodePackedYUV(frame []byte, f string, width uint32, height uint32) ([]byte, error) {
	
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
	// Compress to jpeg
	compressedImage, err := encodeJPEG(yuyv)
	if err != nil {
		return nil, err
	} 
	return compressedImage, nil
}
// YUV 4:2:0 decoder. Supports YU12, YV12, I420, NV12, NV21
func decodePlanarYUV(frame []byte, f string, width uint32, height uint32) ([]byte, error) {
	
	yuv := image.NewYCbCr(image.Rect(0, 0, int(width), int(height)), image.YCbCrSubsampleRatio420)
	// Copy luma plane
	for i := range yuv.Y {
		yuv.Y[i] = frame[i]
	}
	// Copy chroma planes in format specific order
	switch f {
	case "YU12":
		for i := range yuv.Cr {
			yuv.Cr[i] = frame[i+len(yuv.Y)]
		}
		for i := range yuv.Cb {
			yuv.Cb[i] = frame[i+len(yuv.Y)+len(yuv.Cr)]
		}
	case "YV12":
		for i := range yuv.Cr {
			yuv.Cr[i] = frame[i+len(yuv.Y)]
		}
		for i := range yuv.Cb {
			yuv.Cb[i] = frame[i+len(yuv.Y)+len(yuv.Cr)]
		}
	case "I420":
		for i := range yuv.Cr {
			yuv.Cr[i] = frame[i+len(yuv.Y)]
		}
		for i := range yuv.Cb {
			yuv.Cb[i] = frame[i+len(yuv.Y)+len(yuv.Cr)]
		}
	case "NV12":
		for i := range yuv.Cr {
			yuv.Cb[i] = frame[i+len(yuv.Y)]
			yuv.Cr[i] = frame[i+len(yuv.Y)+1]
		}
	case "NV21":
		for i := range yuv.Cr {
			yuv.Cr[i] = frame[i+len(yuv.Y)]
			yuv.Cb[i] = frame[i+len(yuv.Y)+1]
		}
	}
	// Compress to jpeg
	compressedImage, err := encodeJPEG(yuv)
	if err != nil {
		return nil, err
	} 
	return compressedImage, nil
}
// RGB decoder, it supports RGB3, BGR3.
func decodeRGB(frame []byte, f string, width uint32, height uint32) ([]byte, error) {
	
	rgb := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	rgbbuf := make([]uint8, 3*int(width)*int(height))
	for i := range frame {
		if i%3 == 0 {
			switch f {
			case "RGB3":
				rgbbuf[i] = frame[i]
				rgbbuf[i+1] = frame[i+1]
				rgbbuf[i+2] = frame[i+2]
			case "BGR3":
				rgbbuf[i] = frame[i+2]
				rgbbuf[i+1] = frame[i+1]
				rgbbuf[i+2] = frame[i]
			}
		}
	}
	rgb.Pix = rgbbuf
	rgb.Stride = 3 * int(width)
	compressedImage, err := encodeJPEG(rgb)
	if err != nil {
		return nil, err
	} 
	return compressedImage, nil
}
// This is our RGBA decoder, it supports RGB4 and BGR4.
func decodeRGBA(frame []byte, f string, width uint32, height uint32) ([]byte, error) {
	
	rgba := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	rgbabuf := make([]uint8, 4*int(width)*int(height))
	for i := range frame {
		if i%4 == 0 {
			switch f {
			case "RGB4":
				rgbabuf[i] = frame[i]
				rgbabuf[i+1] = frame[i+1]
				rgbabuf[i+2] = frame[i+2]
				rgbabuf[i+3] = frame[i+3]
			case "BGR4":
				rgbabuf[i] = frame[i+2]
				rgbabuf[i+1] = frame[i+1]
				rgbabuf[i+2] = frame[i]
				rgbabuf[i+3] = frame[i+3]
			}
		}
	}
	rgba.Pix = rgbabuf
	rgba.Stride = 4 * int(width)
	// Compress to jpeg
	compressedImage, err := encodeJPEG(rgba)
	if err != nil {
		return nil, err
	} 
	return compressedImage, nil
}
// Encodes our golang image.Image into a compressed JPEG byte array
func encodeJPEG(img image.Image) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := jpeg.Encode(buf, img, nil); err != nil {
		return nil, err
	}
	readBuf, _ := ioutil.ReadAll(buf)
	return readBuf, nil
}

// Declare our library of format types upon initialization
func init() {
	formats = make(map[string]func([]byte, string, uint32, uint32)([]byte, error))
	for _, format := range(packedYUV) {
		formats[format] = decodePackedYUV
	}
	for _, format := range(planarYUV) {
		formats[format] = decodePlanarYUV
	}
	for _, format := range(rgb) {
		formats[format] = decodeRGB
	}
	for _, format := range(rgba) {
		formats[format] = decodeRGBA
	}
}