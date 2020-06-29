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
	colorful "github.com/lucasb-eyer/go-colorful"
	"math"
	"runtime"
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
		Stars :ref.Stars,
		HFR   :ref.HFR,
	}

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
	return min, mult
}

// Combine L, R, G, B images into one multi-channel image.
// All images must have the same dimensions, or undefined results occur. 
func CombineLRGB(chans []*FITSImage) FITSImage {
	pixelsOrig:=chans[0].Pixels
	pixelsComb:=pixelsOrig*int32(len(chans)-1)          // combining 4 channels into 3
	rgb:=FITSImage{
		Header:NewFITSHeader(),
		Bitpix:-32,
		Bzero :0,
		Naxisn:make([]int32, len(chans[0].Naxisn)+1),
		Pixels:pixelsComb,
		Data  :make([]float32,int(pixelsComb)),
		Exposure: chans[0].Exposure+chans[1].Exposure+chans[2].Exposure+chans[3].Exposure,
		Stars :chans[0].Stars,
		HFR   :chans[0].HFR,
	}

	copy(rgb.Naxisn, chans[0].Naxisn)
	rgb.Naxisn[len(chans[0].Naxisn)]=int32(len(chans)-1) // combining 4 channels into 3

      lMin,   lMult:=float32(0), float32(1) //getCommonNormalizationFactors(chans[0:1])
	rgbMin, rgbMult:=float32(0), float32(1) //getCommonNormalizationFactors(chans[1: ])

	// parallelize work across CPU cores
	sem         :=make(chan bool, runtime.NumCPU())
	workPackages:=8*runtime.NumCPU()
	for i:=0; i<workPackages; i++ {
		sem <- true 

		go func(i int) {
			defer func() { <-sem }()
			// each work package is one block of the original picture
			start:=len(chans[0].Data)* i   /workPackages
			end  :=len(chans[0].Data)*(i+1)/workPackages
			if end>len(chans[0].Data) {
				end=len(chans[0].Data)
			}

			ls:=chans[0].Data[start:end]
			rs:=chans[1].Data[start:end]
			gs:=chans[2].Data[start:end]
			bs:=chans[3].Data[start:end]

			rOut:=rgb.Data[start+0*int(pixelsOrig):end+0*int(pixelsOrig)]
			gOut:=rgb.Data[start+1*int(pixelsOrig):end+1*int(pixelsOrig)]
			bOut:=rgb.Data[start+2*int(pixelsOrig):end+2*int(pixelsOrig)]

			combineLRGBFragment(ls, rs, gs, bs, lMin, lMult, rgbMin, rgbMult, rOut, gOut, bOut)
		}(i)
	}
	for i:=0; i<cap(sem); i++ {  // wait for goroutines to finish
		sem <- true
	}

	return rgb
} 

// Single worker thread for LRGB color combination
func combineLRGBFragment(ls, rs, gs, bs []float32, lMin, lMult, rgbMin, rgbMult float32, rOut, gOut, bOut []float32) {
	// for each pixel, combine channels
	for id, _ :=range ls {
		// calculate l, r, g, b in [0,1]
		l:=(ls[id]-  lMin) *   lMult 
		r:=(rs[id]-rgbMin) * rgbMult
		g:=(gs[id]-rgbMin) * rgbMult
		b:=(bs[id]-rgbMin) * rgbMult

		// convert RGB to LAB xyY color space
		col:=colorful.LinearRgb(float64(r),float64(g),float64(b))
		x,y,_:=col.Xyy() // ignore the Y channel, which is luminance from RGB

		// substitute given L from channel 0 for calculated luminance from RGB
		newCol:=colorful.Xyy(x,y,float64(l)).Clamped()

		// convert back into RGB color space and store
		rr, gg, bb:=newCol.LinearRgb()
		rOut[id]= float32(rr)
		gOut[id]= float32(gg)
		bOut[id]= float32(bb)
	}
}

// Set image black point so histogram peaks match the given new peak value,
// and median star colors are of a neutral tone
func (f *FITSImage) SetBlackWhitePoints(newBlack float32) error {
	// Estimate location (=histogram peak, background black point) per color channel
	l:=len(f.Data)/3
	statsR,err:=CalcExtendedStats(f.Data[   :  l], f.Naxisn[0])
	if err!=nil {return err}
	statsG,err:=CalcExtendedStats(f.Data[l  :2*l], f.Naxisn[0])
	if err!=nil {return err}
	statsB,err:=CalcExtendedStats(f.Data[2*l:   ], f.Naxisn[0])
	if err!=nil {return err}
	locR, locG, locB:=statsR.Location, statsG.Location, statsB.Location
	LogPrintf("RGB histogram peaks are located at (%.2f%%, %.2f%%, %.2f%%)\n", locR*100, locG*100, locB*100)

	// Estimate median star color
	starR:=medianStarIntensity(f.Data[   :  l], f.Naxisn[0], f.Stars)
	starG:=medianStarIntensity(f.Data[l  :2*l], f.Naxisn[0], f.Stars)
	starB:=medianStarIntensity(f.Data[2*l:   ], f.Naxisn[0], f.Stars)
	LogPrintf("Median star color is (%.2f%%, %.2f%%, %.2f%%) from %d stars with avg HFR %.2g\n", starR*100, starG*100, starB*100, len(f.Stars), f.HFR)

	// Crank up the luminance to calculate assumed current white
	col:=colorful.LinearRgb(float64(starR), float64(starG), float64(starB))
	x,y,_:=col.Xyy()                        // ignore the Y channel, which is luminance from RGB
	newCol:=colorful.Xyy(x,y,1.0).Clamped() // maximal luminance
	rr, gg, bb:=newCol.LinearRgb()          // convert back into RGB color space
	whiteR, whiteG, whiteB:=float32(rr), float32(gg), float32(bb)
	//LogPrintf("AssuWhite r=%g g=%g b=%g\n", whiteR, whiteG, whiteB)

	// Calculate transformation factors
	alphaR:=(1.0-newBlack)/(whiteR-locR)
	alphaG:=(1.0-newBlack)/(whiteG-locG)
	alphaB:=(1.0-newBlack)/(whiteB-locB)
	betaR := newBlack-alphaR*locR
	betaG := newBlack-alphaG*locG
	betaB := newBlack-alphaB*locB
	//LogPrintf("alphaR %g betaR=%g\n", alphaR, betaR)
	//LogPrintf("alphaG %g betaG=%g\n", alphaG, betaG)
	//LogPrintf("alphaB %g betaB=%g\n", alphaB, betaB)

	//LogPrintf("newBlack r=%g g=%g b=%g\n", alphaR*locR  +betaR, alphaG*locG  +betaG, alphaB*locB  +betaB)
	//LogPrintf("newWhite r=%g g=%g b=%g\n", alphaR*whiteR+betaR, alphaG*whiteG+betaG, alphaB*whiteB+betaB)

 	// Apply transformation
	LogPrintf("Moving peak to (%.2f%%, %.2f%%, %.2f%%) and making (%.2f%%, %.2f%%, %.2f%%) white...\n", 
	           newBlack*100, newBlack*100, newBlack*100, whiteR*100, whiteG*100, whiteB*100)
	f.ScaleOffsetClampRGB(alphaR, betaR, alphaG, betaG, alphaB, betaB)
	return nil
}

// Returns median intensity value for the stars in the given monochrome image
func medianStarIntensity(data []float32, width int32, stars []Star) float32 {
	height:=int32(len(data))/width
	// Gather together channel values for all stars
	gathered:=make([]float32,len(data))
	numGathered:=0
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
							gathered[numGathered]=d
							numGathered++
						}
					}
				}
			}
		}
	}

	median:=QSelectMedianFloat32(gathered[:numGathered])
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

