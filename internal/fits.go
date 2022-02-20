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


package internal

import (
	"fmt"
	"math"
	"strings"
)

// A FITS image. 
// Spec here:   https://fits.gsfc.nasa.gov/standard40/fits_standard40aa-le.pdf
// Primer here: https://fits.gsfc.nasa.gov/fits_primer.html
type FITSImage struct {
	ID       int         // Sequential ID number, for log output. Counted upwards from 0 for light frames. By convention, dark is -1 and flat is -2
	FileName string      // Original file name, if any, for log output.

	Header FITSHeader 	 // The header with all keys, values, comments, history entries etc.
	Bitpix int32         // Bits per pixel value from the header. Positive values are integral, negative floating.
	Bzero  float32 		 // Zero offset. True pixel value is Bzero + Data[i]. 
						 // Helps implement unsigned values with signed data types.
	Naxisn []int32 		 // Axis dimensions. Most quickly varying dimension first (i.e. X,Y)
	Pixels int32 		 // Number of pixels in the image. Product of Naxisn[]

	Data   []float32     // The image data

	Exposure float32     // Image exposure in seconds

	Stats  *BasicStats   // Basic image statistics: min, mean, max
	MedianDiffStats *BasicStats // Local median difference stats, for bad pixel detection, star detection
	 
	Stars  []Star        // Star detections
	HFR    float32       // Half-flux radius of the star detections

	Trans    Transform2D // Transformation to reference frame
	Residual float32     // Residual error from the above transformation 
}

// Creates a FITS image initialized with empty header
func NewFITSImage() FITSImage {
	return FITSImage{
		Header:  NewFITSHeader(),
	}
}

// FITS header data
type FITSHeader struct {
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
func NewFITSHeader() FITSHeader {
	return FITSHeader{
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
const fitsHeaderLineSize int =   80       // Line size of a FITS header


func (f *FITSImage) DimensionsToString() string {
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
func CombineRGB(chans []*FITSImage, ref *FITSImage) FITSImage {
	pixelsOrig:=chans[0].Pixels
	pixelsComb:=pixelsOrig*int32(len(chans))
	rgb:=FITSImage{
		Header:NewFITSHeader(),
		Bitpix:-32,
		Bzero :0,
		Naxisn:make([]int32, len(chans[0].Naxisn)+1),
		Pixels:pixelsComb,
		Data  :make([]float32,int(pixelsComb)),
		Exposure: chans[0].Exposure+chans[1].Exposure+chans[2].Exposure,
		Stars :[]Star{},
		HFR   :0,
	}
	if ref!=nil { rgb.Stars, rgb.HFR=ref.Stars, ref.HFR }

	copy(rgb.Naxisn, chans[0].Naxisn)
	rgb.Naxisn[len(chans[0].Naxisn)]=int32(len(chans))

	min, mult:=getCommonNormalizationFactors(chans)
	for id, ch:=range chans {
		dest:=rgb.Data[int32(id)*pixelsOrig : (int32(id)+1)*pixelsOrig]
		for j,val:=range ch.Data {
			dest[j]=(val-min)*mult
		}
	}

	return rgb
} 

// calculate common normalization factors to [0..1] across all channels
func getCommonNormalizationFactors(chans []*FITSImage) (min, mult float32) {
	min =chans[0].Stats.Min
	max:=chans[0].Stats.Max
	for _, ch :=range chans[1:] {
		if ch.Stats.Min<min {
			min=ch.Stats.Min
		}
		if ch.Stats.Max>max {
			max=ch.Stats.Max
		}
	}
	mult=1 / (max - min)
	LogPrintf("common normalization factors min=%f mult=%f\n", min, mult)
	return min, mult
}


// Applies luminance to existing 3-channel image with luminance in 3rd channel, all channels in [0,1]. 
// All images must have the same dimensions, or undefined results occur. 
func (hsl *FITSImage) ApplyLuminanceToCIExyY(lum *FITSImage) {
	l:=len(hsl.Data)/3
	dest:=hsl.Data[2*l:]
	copy(dest, lum.Data)
}


// Set image black point so histogram peaks match the rightmost channel peak,
// and median star colors are of a neutral tone. 
func (f *FITSImage) SetBlackWhitePoints() error {
	// Estimate location (=histogram peak, background black point) per color channel
	l:=len(f.Data)/3
	statsR,err:=CalcExtendedStats(f.Data[   :  l], f.Naxisn[0])
	if err!=nil {return err}
	statsG,err:=CalcExtendedStats(f.Data[l  :2*l], f.Naxisn[0])
	if err!=nil {return err}
	statsB,err:=CalcExtendedStats(f.Data[2*l:   ], f.Naxisn[0])
	if err!=nil {return err}
	locR, locG, locB:=statsR.Location, statsG.Location, statsB.Location

	// Pick rightmost histogram peak as new background peak location (do not clip blacks)
	locNew:=locR
	if locG>locNew { locNew=locG }
	if locB>locNew { locNew=locB }

	// Estimate median star color
	starR:=medianStarIntensity(f.Data[   :  l], f.Naxisn[0], f.Stars)
	starG:=medianStarIntensity(f.Data[l  :2*l], f.Naxisn[0], f.Stars)
	starB:=medianStarIntensity(f.Data[2*l:   ], f.Naxisn[0], f.Stars)
	LogPrintf("Background peak (%.2f%%, %.2f%%, %.2f%%) and median star color (%.2f%%, %.2f%%, %.2f%%)\n", 
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

	LogPrintf("r=%.3f*r %+.3f, g=%.3f*g %+.3f, b=%.3f*b %+.3f\n", alphaR, betaR, alphaG, betaG, alphaB, betaB)
	f.ScaleOffsetClampRGB(alphaR, betaR, alphaG, betaG, alphaB, betaB)
	return nil
}

// Returns median intensity value for the stars in the given monochrome image
func medianStarIntensity(data []float32, width int32, stars []Star) float32 {
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

	median:=QSelectMedianFloat32(gathered)
	gathered=nil
	return median
}

// Apply NxN binning to source image and return new resulting image
func BinNxN(src *FITSImage, n int32) FITSImage {
	// calculate binned image size
	binnedPixels:=int32(1)
	binnedNaxisn:=make([]int32, len(src.Naxisn))
	for i,originalN:=range(src.Naxisn) {
		binnedN:=originalN/n
		binnedNaxisn[i]=binnedN
		binnedPixels*=binnedN
	}

	// created binned image header
	binned:=FITSImage{
		Header:NewFITSHeader(),
		Bitpix:-32,
		Bzero :0,
		Naxisn:binnedNaxisn,
		Pixels:binnedPixels,
		Data  :make([]float32,int(binnedPixels)),
		Exposure: src.Exposure,
		ID    :src.ID,
	}

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
func (f* FITSImage) FillCircle(xc,yc,r,color float32) {
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
func ShowStars(src *FITSImage, hfrMultiple float32) FITSImage {
	// created new image header
	res:=FITSImage{
		Header:NewFITSHeader(),
		Bitpix:-32,
		Bzero :0,
		Naxisn:src.Naxisn,
		Pixels:src.Pixels,
		Data  :make([]float32,int(src.Pixels)),
	}

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


