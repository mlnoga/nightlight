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

	return tiff.Encode(writer, img, &tiff.Options{Compression: tiff.Deflate, Predictor: true})
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

	return tiff.Encode(writer, img, &tiff.Options{Compression: tiff.Deflate, Predictor: true})
}
