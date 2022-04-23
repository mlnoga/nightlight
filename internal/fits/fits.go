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
	"fmt"
	"math"
	"strings"
	"github.com/mlnoga/nightlight/internal/stats"
	"github.com/mlnoga/nightlight/internal/star"
)

// A FITS image. 
// Spec here:   https://fits.gsfc.nasa.gov/standard40/fits_standard40aa-le.pdf
// Primer here: https://fits.gsfc.nasa.gov/fits_primer.html
type Image struct {
	ID       int         // Sequential ID number, for log output. Counted upwards from 0 for light frames. By convention, dark is -1 and flat is -2
	FileName string      // Original file name, if any, for log output.

	Header Header 	     // The header with all keys, values, comments, history entries etc.
	Bitpix int32         // Bits per pixel value from the header. Positive values are integral, negative floating.
	Bzero  float32 		 // Zero offset. True pixel value is Bzero + Bscale * Data[i]. 
	Bscale float32 		 // Value scaler. True pixel value is Bzero + Bscale * Data[i]. 
						 // Helps implement unsigned values with signed data types.
	Naxisn []int32 		 // Axis dimensions. Most quickly varying dimension first (i.e. X,Y)
	Pixels int32 		 // Number of pixels in the image. Product of Naxisn[]

	Data   []float32     // The image data

	Exposure float32     // Image exposure in seconds

	Stats  *stats.Stats   // Basic image statistics: min, mean, max
	MedianDiffStats *stats.Stats // Local median difference stats, for bad pixel detection, star detection
	 
	Stars  []star.Star        // Star detections
	HFR    float32       // Half-flux radius of the star detections

	Trans    star.Transform2D // Transformation to reference frame
	Residual float32     // Residual error from the above transformation 
}

// Creates a FITS image initialized with empty header
func NewImage() *Image {
	return &Image{
		Header:  NewHeader(),
		Bscale:  1,
	}
}

// Creates a FITS image from given naxisn. Data is not copied, allocated if nil. naxisn is deep copied
func NewImageFromNaxisn(naxisn []int32, data []float32) *Image {
	numPixels:=int32(1)
	for _,naxis:=range(naxisn) {
		numPixels*=naxis
	}
	if data==nil {
		data=make([]float32, numPixels)
	}
	return &Image{
		ID:       0,
		FileName: "",
		Header:   NewHeader(),
		Bitpix:   -32,
		Bzero:    0,
		Bscale:   1,
		Naxisn:   append([]int32(nil), naxisn...), // clone slice
		Pixels:   numPixels,
		Data:     data,
		Exposure: 0,
		Stats:    stats.NewStats(data, naxisn[0]),
		MedianDiffStats: nil,
		Stars:    nil,
		HFR:      0,
		Trans:    star.IdentityTransform2D(),
		Residual: 0,
	}
}


// Creates a FITS image from given image. New data array will be allocated
func NewImageFromImage(img *Image) *Image {
	data:=make([]float32, img.Pixels)
	return &Image{
		ID:       img.ID,
		FileName: img.FileName,
		Header:   img.Header,
		Bitpix:   img.Bitpix,
		Bzero:    img.Bzero,
		Bscale:   img.Bscale,
		Naxisn:   append([]int32(nil), img.Naxisn...), // clone slice
		Pixels:   img.Pixels,
		Data:     data,
		Exposure: img.Exposure,
		Stats:    stats.NewStats(data, img.Naxisn[0]),
		MedianDiffStats: nil,
		Stars:    img.Stars,
		HFR:      img.HFR,
		Trans:    star.IdentityTransform2D(),
		Residual: 0,
	}
}


// FITS header data
type Header struct {
	Bools    map[string]bool
	Ints     map[string]int32
	Floats   map[string]float32
	Strings  map[string]string
	Dates    map[string]string
	Comments []string
	History  []string
	End      bool
	Length   int32
}

// Creates a FITS header initialized with empty maps and arrays
func NewHeader() Header {
	return Header{
		Bools:   make(map[string]bool), 
		Ints:    make(map[string]int32),
		Floats:  make(map[string]float32),
		Strings: make(map[string]string),
		Dates:   make(map[string]string),
		Comments:make([]string,0),
		History: make([]string,0),
		End:     false,
	}
}

const fitsBlockSize int      = 2880       // Block size of FITS header and data units
const HeaderLineSize int =   80       // Line size of a FITS header


func (f *Image) DimensionsToString() string {
	b:=strings.Builder{}
	for i,naxis:=range(f.Naxisn) {
		if i>0 { 
			fmt.Fprintf(&b, "x%d", naxis)
		} else {
			fmt.Fprintf(&b, "%d", naxis)
		}
	} 
	return b.String()
}


// Apply NxN binning to source image and return new resulting image
func NewImageBinNxN(src *Image, n int32) *Image {
	// calculate binned image size
	binnedPixels:=int32(1)
	binnedNaxisn:=make([]int32, len(src.Naxisn))
	for i,originalN:=range(src.Naxisn) {
		binnedN:=originalN/n
		binnedNaxisn[i]=binnedN
		binnedPixels*=binnedN
	}

	binned:=NewImageFromNaxisn(binnedNaxisn, nil)
	binned.ID, binned.FileName, binned.Exposure = src.ID, src.FileName, src.Exposure

	// calculate binned image pixel values
	// FIXME: pretty inefficient?
	normalizer:=1.0/float32(n*n)
	for y:=int32(0); y<binnedNaxisn[1]; y++ {
		for x:=int32(0); x<binnedNaxisn[0]; x++ {
			sum:=float32(0)
			for yoff:=int32(0); yoff<n; yoff++ {
				for xoff:=int32(0); xoff<n; xoff++ {
					origPos:=(y*n+yoff)*src.Naxisn[0] + (x*n+xoff)
					sum+=src.Data[origPos]
				}
			}
			avg:=sum*normalizer
			binnedPos:=y*binned.Naxisn[0] + x
			binned.Data[binnedPos]=avg
		}		
	}

	return binned
}


// Fill a circle of given radius on the FITS image
func (f* Image) FillCircle(xc,yc,r,color float32) {
	for y:=-r; y<=r; y+=0.5 {
		for x:=-r; x<=r; x+=0.5 {
			distSq:=y*y+x*x
			if distSq<=r*r+1e-6 {
				index:=int32(xc+x) + int32(yc+y)*(f.Naxisn[0])
				if index>=0 && index<int32(len(f.Data)) {
					f.Data[index]=color
				}
			}
		}
	}
}


// Show stars detected on the source image as circles in a new resulting image
func NewImageFromStars(src *Image, hfrMultiple float32) *Image {
	res:=NewImageFromImage(src)
	for _,s:=range(src.Stars) {
		radius:=s.HFR*hfrMultiple
		res.FillCircle(s.X, s.Y, radius, s.Mass/(radius*radius*float32(math.Pi)) )
	}
	return res
}



// Equal tells whether a and b contain the same elements.
// A nil argument is equivalent to an empty slice.
func EqualInt32Slice(a, b []int32) bool {
    if len(a) != len(b) {
        return false
    }
    for i, v := range a {
        if v != b[i] {
            return false
        }
    }
    return true
}


