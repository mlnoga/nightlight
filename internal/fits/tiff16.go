// Copyright (C) 2020 Markus L. Noga
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package fits

import (
	"bufio"
	"image"
	"image/color"
	"io"
	"math"
	"os"

	"github.com/mlnoga/nightlight/internal/stats"
	"golang.org/x/image/tiff"
)

// Write a FITS image to 16-bit TIFF, using the given min, max and gamma.
func (f *Image) WriteTIFF16ToFile(fileName string, min, max, gamma float32) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	return f.WriteTIFF16(writer, min, max, gamma)
}

// Write a FITS image to 16-bit TIFF, using the given min, max and gamma.
func (f *Image) WriteTIFF16(writer io.Writer, min, max, gamma float32) error {
	// convert pixels into Golang Image
	width, height := int(f.Naxisn[0]), int(f.Naxisn[1])
	size := width * height
	img := image.NewRGBA64(image.Rectangle{image.Point{0, 0}, image.Point{width, height}})
	scale := 1.0 / (max - min)
	gammaInv := float64(1.0 / gamma)
	for y := 0; y < height; y++ {
		yoffset := y * width
		for x := 0; x < width; x++ {
			r := f.Data[yoffset+x]
			g := f.Data[yoffset+x+size]
			b := f.Data[yoffset+x+size*2]
			r = (r - min) * scale
			g = (g - min) * scale
			b = (b - min) * scale
			// replace NaNs with zeros for export, else JPG output breaks
			if math.IsNaN(float64(r)) || r < 0 {
				r = 0
			}
			if math.IsNaN(float64(g)) || g < 0 {
				g = 0
			}
			if math.IsNaN(float64(b)) || b < 0 {
				b = 0
			}
			if r > 1 {
				r = 1
			}
			if g > 1 {
				g = 1
			}
			if b > 1 {
				b = 1
			}
			if gammaInv != 1.0 {
				r = float32(math.Pow(float64(r), gammaInv))
				g = float32(math.Pow(float64(g), gammaInv))
				b = float32(math.Pow(float64(b), gammaInv))
			}
			c := color.RGBA64{uint16(r * 65535), uint16(g * 65535), uint16(b * 65535), 65535}
			img.SetRGBA64(x, y, c)
		}
	}

	return tiff.Encode(writer, img, &tiff.Options{Compression: tiff.Uncompressed, Predictor: false})
}

// Write a grayscale FITS image to 16-bit TIFF, using the given min, max and gamma.
func (f *Image) WriteMonoTIFF16ToFile(fileName string, min, max, gamma float32) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	return f.WriteMonoTIFF16(writer, min, max, gamma)
}

// Write a grayscale FITS image to 16-bit TIFF, using the given min, max and gamma.
func (f *Image) WriteMonoTIFF16(writer io.Writer, min, max, gamma float32) error {
	// convert pixels into Golang Image
	width, height := int(f.Naxisn[0]), int(f.Naxisn[1])
	img := image.NewGray16(image.Rectangle{image.Point{0, 0}, image.Point{width, height}})
	scale := 1 / (max - min)
	gammaInv := float64(1.0 / gamma)
	for y := 0; y < height; y++ {
		yoffset := y * width
		for x := 0; x < width; x++ {
			gray := f.Data[yoffset+x]
			gray = (gray - min) * scale
			// replace NaNs with zeros for export, else TIFF output breaks
			if math.IsNaN(float64(gray)) || gray < 0 {
				gray = 0
			}
			if gray > 1 {
				gray = 1
			}
			if gammaInv != 1.0 {
				gray = float32(math.Pow(float64(gray), gammaInv))
			}
			c := color.Gray16{uint16(gray * 65535)}
			img.SetGray16(x, y, c)
		}
	}

	return tiff.Encode(writer, img, &tiff.Options{Compression: tiff.Uncompressed, Predictor: false})
}

// Read a color or grayscale TIFF image into a FITS image.
func (f *Image) ReadTIFF(fileName string) error {
	// open file and create buffered reader
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)

	// decode TIFF file into golang image
	t, err := tiff.Decode(reader)
	if err != nil {
		return err
	}

	// determine width, height, color depth and number of color channels
	width, height := t.Bounds().Dx(), t.Bounds().Dy()
	bitpix, channels := colorModelToBitpixAndChannels(t.ColorModel())

	// set FITS metadata
	f.Bitpix = bitpix
	f.Naxisn = make([]int32, 3)
	f.Naxisn[0] = int32(width)
	f.Naxisn[1] = int32(height)
	f.Naxisn[2] = int32(channels)
	if channels == 1 {
		f.Naxisn = f.Naxisn[:2]
	}
	f.Pixels = int32(width) * int32(height) * channels
	f.Bzero, f.Bscale = 0, 1

	// allocate FITS image bitmap
	f.Data = make([]float32, f.Pixels)
	sizeThird := int(f.Pixels / 3)
	sizeTwoThirds := int(f.Pixels * 2)

	// keep running stats
	min, max, sum := float32(math.MaxFloat32), float32(-math.MaxFloat32), float64(0)

	// read and convert pixels
	if channels == 1 {
		// greyscale case
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				// convert pixel
				c := t.At(x, y).(color.Gray16)
				gray := float32(c.Y)
				f.Data[y*width+x] = gray

				// update stats
				if gray < min {
					min = gray
				}
				if gray > max {
					max = gray
				}
				sum += float64(gray)
			}
		}
	} else {
		// RGB case
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				// convert pixel
				c := t.At(x, y).(color.RGBA64)
				r, g, b := float32(c.R), float32(c.G), float32(c.B)
				f.Data[y*width+x] = r
				f.Data[y*width+x+sizeThird] = g
				f.Data[y*width+x+sizeTwoThirds] = b

				// convert r, g, b to greyscale value
				gray := 0.2126*r + 0.7152*g + 0.0722*b

				// update stats
				if gray < min {
					min = gray
				}
				if gray > max {
					max = gray
				}
				sum += float64(gray)
			}
		}
	}

	// finalize stats
	mean := float32(sum / float64(width) / float64(height))
	f.Stats = stats.NewStatsWithMMM(f.Data, f.Naxisn[0], min, max, mean)

	return nil
}

func colorModelToBitpixAndChannels(m color.Model) (bitpix, channels int32) {
	switch m {
	case color.RGBAModel:
		return 8, 3
	case color.RGBA64Model:
		return 16, 3
	case color.NRGBAModel:
		return 8, 3
	case color.NRGBA64Model:
		return 16, 3
	case color.AlphaModel:
		return 8, 1
	case color.Alpha16Model:
		return 16, 1
	case color.GrayModel:
		return 8, 1
	case color.Gray16Model:
		return 16, 1
	default:
		return 0, 0
	}
}
