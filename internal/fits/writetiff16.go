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

// Write a FITS image to 16-bit TIFF. Image must be normalized to [0,1]
func (f *Image) WriteTIFF16ToFile(fileName string) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	return f.WriteTIFF16(writer)
}

// Write a FITS image to 16-bit TIFF. Image must be normalized to [0,1]
func (f *Image) WriteTIFF16(writer io.Writer) error {
	// convert pixels into Golang Image
	width, height := int(f.Naxisn[0]), int(f.Naxisn[1])
	size := width * height
	img := image.NewRGBA64(image.Rectangle{image.Point{0, 0}, image.Point{width, height}})
	for y := 0; y < height; y++ {
		yoffset := y * width
		for x := 0; x < width; x++ {
			r := f.Data[yoffset+x]
			g := f.Data[yoffset+x+size]
			b := f.Data[yoffset+x+size*2]
			// replace NaNs with zeros for export, else JPG output breaks
			if math.IsNaN(float64(r)) {
				r = 0
			}
			if math.IsNaN(float64(g)) {
				g = 0
			}
			if math.IsNaN(float64(b)) {
				b = 0
			}
			c := color.RGBA64{uint16(r * 65535.0), uint16(g * 65535.0), uint16(b * 65535.0), 65535}
			img.SetRGBA64(x, y, c)
		}
	}

	return tiff.Encode(writer, img, &tiff.Options{Compression: tiff.Deflate, Predictor: true})
}

// Write a grayscale FITS image to 16-bit TIFF. Image must be normalized to [0,1]
func (f *Image) WriteMonoTIFF16ToFile(fileName string) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	return f.WriteMonoTIFF16(writer)
}

// Write a grayscale FITS image to 16-bit TIFF. Image must be normalized to [0,1]
func (f *Image) WriteMonoTIFF16(writer io.Writer) error {
	// convert pixels into Golang Image
	width, height := int(f.Naxisn[0]), int(f.Naxisn[1])
	img := image.NewGray16(image.Rectangle{image.Point{0, 0}, image.Point{width, height}})
	for y := 0; y < height; y++ {
		yoffset := y * width
		for x := 0; x < width; x++ {
			gray := f.Data[yoffset+x]
			// replace NaNs with zeros for export, else TIFF output breaks
			if math.IsNaN(float64(gray)) {
				gray = 0
			}
			c := color.Gray16{uint16(gray * 65535.0)}
			img.SetGray16(x, y, c)
		}
	}

	return tiff.Encode(writer, img, &tiff.Options{Compression: tiff.Deflate, Predictor: true})
}
