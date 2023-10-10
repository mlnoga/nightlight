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
	"image/jpeg"
	"io"
	"math"
	"os"
)

// Write a FITS image to JPG. Image must be normalized to [0,1]
func (f *Image) WriteJPGToFile(fileName string, quality int) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	return f.WriteJPG(writer, quality)
}

// Write a FITS image to JPG. Image must be normalized to [0,1]
func (f *Image) WriteJPG(writer io.Writer, quality int) error {
	// convert pixels into Golang Image
	width, height := int(f.Naxisn[0]), int(f.Naxisn[1])
	size := width * height
	img := image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{width, height}})
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
			c := color.RGBA{uint8(r * 255.0), uint8(g * 255.0), uint8(b * 255.0), 255}
			img.SetRGBA(x, y, c)
		}
	}

	return jpeg.Encode(writer, img, &jpeg.Options{Quality: quality})
}

// Write a grayscale FITS image to JPG. Image must be normalized to [0,1]
func (f *Image) WriteMonoJPGToFile(fileName string, quality int) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	return f.WriteMonoJPG(writer, quality)
}

// Write a grayscale FITS image to JPG. Image must be normalized to [0,1]
func (f *Image) WriteMonoJPG(writer io.Writer, quality int) error {
	// convert pixels into Golang Image
	width, height := int(f.Naxisn[0]), int(f.Naxisn[1])
	img := image.NewGray(image.Rectangle{image.Point{0, 0}, image.Point{width, height}})
	for y := 0; y < height; y++ {
		yoffset := y * width
		for x := 0; x < width; x++ {
			gray := f.Data[yoffset+x]
			// replace NaNs with zeros for export, else JPG output breaks
			if math.IsNaN(float64(gray)) {
				gray = 0
			}
			c := color.Gray{uint8(gray * 255.0)}
			img.SetGray(x, y, c)
		}
	}

	return jpeg.Encode(writer, img, &jpeg.Options{Quality: quality})
}
