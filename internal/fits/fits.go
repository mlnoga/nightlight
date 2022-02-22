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
	"io"
	"fmt"
	"math"
	"strings"
	"github.com/mlnoga/nightlight/internal/qsort"
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

// Combine single color images into one multi-channel image.
// All images must have the same dimensions, or undefined results occur. 
func NewRGBFromChannels(chans []*Image, ref *Image, logWriter io.Writer) *Image {
	naxisn:=make([]int32, len(chans[0].Naxisn)+1)
	copy(naxisn, chans[0].Naxisn)
	naxisn[len(chans[0].Naxisn)]=int32(len(chans))

	rgb:=NewImageFromNaxisn(naxisn, nil)
	rgb.Exposure=chans[0].Exposure+chans[1].Exposure+chans[2].Exposure
	if ref!=nil { rgb.Stars, rgb.HFR=ref.Stars, ref.HFR }

	pixelsOrig:=chans[0].Pixels
	min, mult:=getCommonNormalizationFactors(chans)
	fmt.Fprintf(logWriter, "common normalization factors min=%f mult=%f\n", min, mult)
	for id, ch:=range chans {
		dest:=rgb.Data[int32(id)*pixelsOrig : (int32(id)+1)*pixelsOrig]
		for j,val:=range ch.Data {
			dest[j]=(val-min)*mult
		}
	}
	return rgb
} 

// calculate common normalization factors to [0..1] across all channels
func getCommonNormalizationFactors(chans []*Image) (min, mult float32) {
	min =chans[0].Stats.Min()
	max:=chans[0].Stats.Max()
	for _, ch :=range chans[1:] {
		if ch.Stats.Min()<min {
			min=ch.Stats.Min()
		}
		if ch.Stats.Max()>max {
			max=ch.Stats.Max()
		}
	}
	mult=1 / (max - min)
	return min, mult
}


// Applies luminance to existing 3-channel image with luminance in 3rd channel, all channels in [0,1]. 
// All images must have the same dimensions, or undefined results occur. 
func (hsl *Image) ApplyLuminanceToCIExyY(lum *Image) {
	l:=len(hsl.Data)/3
	dest:=hsl.Data[2*l:]
	copy(dest, lum.Data)
}


// Set image black point so histogram peaks match the rightmost channel peak,
// and median star colors are of a neutral tone. 
func (f *Image) SetBlackWhitePoints(logWriter io.Writer) error {
	// Estimate location (=histogram peak, background black point) per color channel
	locR:=stats.NewStatsForChannel(f.Data, f.Naxisn[0], 0, 3).Location()
	locG:=stats.NewStatsForChannel(f.Data, f.Naxisn[0], 1, 3).Location()
	locB:=stats.NewStatsForChannel(f.Data, f.Naxisn[0], 2, 3).Location()

	// Pick rightmost histogram peak as new background peak location (do not clip blacks)
	locNew:=locR
	if locG>locNew { locNew=locG }
	if locB>locNew { locNew=locB }

	// Estimate median star color
	l:=len(f.Data)/3
	starR:=medianStarIntensity(f.Data[   :  l], f.Naxisn[0], f.Stars)
	starG:=medianStarIntensity(f.Data[l  :2*l], f.Naxisn[0], f.Stars)
	starB:=medianStarIntensity(f.Data[2*l:   ], f.Naxisn[0], f.Stars)
	fmt.Fprintf(logWriter, "Background peak (%.2f%%, %.2f%%, %.2f%%) and median star color (%.2f%%, %.2f%%, %.2f%%)\n", 
	  	      locR*100, locG*100, locB*100, starR*100, starG*100, starB*100)

	// Pick left most star color as new star color location (do not clip stars)
	starNew:=starR
	if starG<starNew { starNew=starG }
	if starB<starNew { starNew=starB }

	// Calculate multiplicative correction factors
	alphaR:=(locNew-starNew)/(locR-starR)
	alphaG:=(locNew-starNew)/(locG-starG)
	alphaB:=(locNew-starNew)/(locB-starB)

	// Calculate additive correction factors 
	betaR:=starNew - alphaR*starR
	betaG:=starNew - alphaG*starG
	betaB:=starNew - alphaB*starB

	fmt.Fprintf(logWriter, "r=%.3f*r %+.3f, g=%.3f*g %+.3f, b=%.3f*b %+.3f\n", alphaR, betaR, alphaG, betaG, alphaB, betaB)
	f.ScaleOffsetClampRGB(alphaR, betaR, alphaG, betaG, alphaB, betaB)
	return nil
}

// Returns median intensity value for the stars in the given monochrome image
func medianStarIntensity(data []float32, width int32, stars []star.Star) float32 {
	if len(stars)==0 { return 0 }

	height:=int32(len(data))/width
	// Gather together channel values for all stars
	gathered:=make([]float32,0,len(data))
	for _, s:=range stars {
		starX,starY:=s.Index%width, s.Index/width
		hfrR:=int32(s.HFR+0.5)
		hfrSq:=(s.HFR+0.01)*(s.HFR+0.01)
		for offY:=-hfrR; offY<=hfrR; offY++ {
			y:=starY+offY
			if y>=0 && y<height {
				for offX:=-hfrR; offX<=hfrR; offX++ {
					x:=starX+offX
					if x>=0 && x<width {
						distSq:=float32(offX*offX+offY*offY)
						if distSq<=hfrSq { 
							d:=data[y*width+x]
							gathered=append(gathered, d)
						}
					}
				}
			}
		}
	}

	median:=qsort.QSelectMedianFloat32(gathered)
	gathered=nil
	return median
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


