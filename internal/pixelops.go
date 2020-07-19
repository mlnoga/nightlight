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


//////////////////////////////////////////////////////////////////
// Complex, CPU-limited pixel operations. Parallelized across CPUs
//////////////////////////////////////////////////////////////////

// A pixel function. Operates in-place. For parallelization across CPUs.
type PixelFunction func(data []float32, params interface{}) 

// An RGB pixel function. Data must be normalized to [0,1]. Operates in-place. For parallelization across CPUs.
type RGBPixelFunction func(rs,gs,bs []float32, params interface{}) 


// Apply given pixel function to the image. Uses thead parallelism across all available CPUs. Operates in-place. 
func (f* FITSImage) ApplyPixelFunction(pf PixelFunction, args interface{}) {
	data:=f.Data

	// split into 8*NumCPU() work packages, limit parallelism to NumCPUS()
	numBatches:=8*runtime.NumCPU()
	batchSize :=(len(data)+numBatches-1)/(numBatches)
	sem       :=make(chan bool, runtime.NumCPU())
	for lower:=0; lower<len(data); lower+=batchSize {
		upper:=lower+batchSize
		if upper>len(data) { upper=len(data) }

		sem <- true 
		go func(data []float32) {
			pf(data, args)
			<-sem
		}(data[lower:upper])
	}

	for i:=0; i<cap(sem); i++ {  // wait for goroutines to finish
		sem <- true
	}
}


// Apply given pixel function to the image. Uses thead parallelism across all available CPUs. Data must be normalized to [0,1]. Operates in-place. 
func (f* FITSImage) ApplyRGBPixelFunction(pf RGBPixelFunction, args interface{}) {
	data:=f.Data
	l   :=len(data)/3

	// split into 8*NumCPU() work packages, limit parallelism to NumCPUS()
	numBatches:=8*runtime.NumCPU()
	batchSize :=(l+numBatches-1)/(numBatches)
	sem       :=make(chan bool, runtime.NumCPU())
	for lower:=0; lower<l; lower+=batchSize {
		upper:=lower+batchSize
		if upper>l { upper=l }

		sem <- true 
		go func(r,g,b []float32) {
			pf(r,g,b, args)
			<-sem
		}(data[lower:upper], data[lower+l:upper+l], data[lower+2*l:upper+2*l])
	}

	for i:=0; i<cap(sem); i++ {  // wait for goroutines to finish
		sem <- true
	}
}


// Arguments for the RGB pixel function to adjust chroma for a range of hues
type pfScaleOffsetArgs struct {
	Scale   float32
	Offset  float32
}

// Pixel function to apply gamma correction. 2nd parameter must be a pfScaleOffsetArgs. Operates in-place. 
func pfScaleOffset(data []float32, params interface{}) {
	scale, offset :=params.(pfScaleOffsetArgs).Scale, params.(pfScaleOffsetArgs).Offset
	for i, d:=range data {
		data[i]=d*scale+offset
	}
}

// Applies given scale factor and offset to image.  Operates in-place. 
func (f* FITSImage) ScaleOffset(scale, offset float32) {
	f.ApplyPixelFunction(pfScaleOffset, pfScaleOffsetArgs{scale, offset})
}

// Normalize image to [0..1] based on basic stats.  Operates in-place. 
func (f* FITSImage) Normalize() {
	scale:=1.0/(f.Stats.Max-f.Stats.Min)
	offset:=-f.Stats.Min*scale
	f.ScaleOffset(scale, offset)
}


// Pixel function to apply gamma correction. Data must be normalized to [0,1]. 2nd parameter must be a float32. Operates in-place. 
func pfGamma(data []float32, params interface{}) {
	g :=params.(float32)
    gg:=float64(1.0/g)
	for i, d:=range data {
		data[i]=float32(math.Pow(float64(d), gg))
	}
}

// Apply gamma correction to image. Image must be normalized to [0,1] before. Operates in-place. 
func (f* FITSImage) ApplyGamma(g float32) {
	f.ApplyPixelFunction(pfGamma, g)
}

// Arguments for the RGB pixel function to adjust chroma for a range of hues
type pfPartialGammaArgs struct {
	From   float32
	To     float32
	Factor float32
}

// Pixel function to apply gamma correction. Data must be normalized to [0,1]. 2nd parameter must be a float32. Operates in-place. 
func pfPartialGamma(data []float32, params interface{}) {
	from, to, g:=params.(pfPartialGammaArgs).From, params.(pfPartialGammaArgs).To, params.(pfPartialGammaArgs).Factor 
    gg:=float64(1.0/g)
	rescale2:=to-from
	rescale1:=1.0/rescale2
	for i, d:=range data {
		if d>from && d<to {
			dd     :=(d-from)*rescale1
			gammaDD:=float32(math.Pow(float64(dd), gg))
			data[i] =from + gammaDD*rescale2
		}
	}
}

// Apply gamma correction to image. Image must be normalized to [0,1] before. Operates in-place. 
func (f* FITSImage) ApplyPartialGamma(from, to, g float32) {
	f.ApplyPixelFunction(pfPartialGamma, pfPartialGammaArgs{from, to, g})
}


type rgbPFChromaArgs struct {
	Mul float32
	Add float32
	Threshold float32
}

// RGB pixel function to adjust CIE HCL chroma by multiplying with given factor and adding given offset. Data must be normalized to [0,1]. 2nd parameter must be a rgbPFChromaArgs. Operates in-place. 
func rgbPFChroma(rs,gs,bs []float32, params interface{}) {
	mul, add, threshold:=params.(rgbPFChromaArgs).Mul, params.(rgbPFChromaArgs).Add, params.(rgbPFChromaArgs).Threshold 
	for i:=0; i<len(rs); i++ {
		r, g, b:=rs[i], gs[i], bs[i]
		if 0.299*r +0.587*g +0.114*b < threshold { continue }

		col:=colorful.LinearRgb(float64(r),float64(g),float64(b))
		h,c,l:=col.Hcl()

		c=math.Max(0.0, math.Min(1.0, c*float64(mul)+float64(add)))

		col2:=colorful.Hcl(h, c, l).Clamped()
		rr,gg,bb:=col2.LinearRgb()
		rs[i], gs[i], bs[i]=float32(rr), float32(gg), float32(bb) 
	}
}

// Adjust CIE HCL chroma by multiplying with given factor and adding given offset. Data must be normalized to [0,1]. Operates in-place. 
// A perceptually linear way of boosting saturation.
func (f* FITSImage) AdjustChroma(mul, add, threshold float32) {
	f.ApplyRGBPixelFunction(rgbPFChroma, rgbPFChromaArgs{mul, add, threshold})
}


type rgbPFNeutralizeBackgroundArgs struct {
	Low float32
	High float32
}

// RGB pixel function to adjust CIE HCL chroma by multiplying with 0 for values below low, with 1 above high, and interpolating linearly in between. 
// Data must be normalized to [0,1]. 2nd parameter must be a rgbPFNeutralizeBackgroundArgs. Operates in-place. 
func rgbPFNeutralizeBackground(rs,gs,bs []float32, params interface{}) {
	low, high:=params.(rgbPFNeutralizeBackgroundArgs).Low, params.(rgbPFNeutralizeBackgroundArgs).Low
	scaler:=float32(0)
	if high>low { scaler=1.0/(high-low) }
	for i:=0; i<len(rs); i++ {
		r, g, b:=rs[i], gs[i], bs[i]
		val:=0.299*r +0.587*g +0.114*b
		if val >= high { continue }
		factor:=float32(0)
		if val>low {
			factor=(val-low)*scaler
		}

		col:=colorful.LinearRgb(float64(r),float64(g),float64(b))
		h,c,l:=col.Hcl()

		c*=float64(factor)

		col2:=colorful.Hcl(h, c, l).Clamped()
		rr,gg,bb:=col2.LinearRgb()
		rs[i], gs[i], bs[i]=float32(rr), float32(gg), float32(bb) 
	}
}

// Adjust CIE HCL chroma by multiplying with 0 for values below low, with 1 above high, and interpolating linearly in between. 
// Data must be normalized to [0,1]. Operates in-place. 
func (f* FITSImage) NeutralizeBackground(low, high float32) {
	f.ApplyRGBPixelFunction(rgbPFNeutralizeBackground, rgbPFNeutralizeBackgroundArgs{low, high})
}


// Arguments for the RGB pixel function to adjust chroma for a range of hues
type rgbPFChromaForHuesArgs struct {
	From   float32
	To     float32
	Factor float32
}

// RGB pixel function to adjust chroma for a given range of hues. Data must be normalized to [0,1]. 2nd parameter must be a rgbPFChromaForHuesArgs
func rgbPFChromaForHues(rs,gs,bs []float32, params interface{}) {
	from, to, factor:=params.(rgbPFChromaForHuesArgs).From, params.(rgbPFChromaForHuesArgs).To, params.(rgbPFChromaForHuesArgs).Factor 
	for i:=0; i<len(rs); i++ {
		r, g, b:=rs[i], gs[i], bs[i]

		col:=colorful.LinearRgb(float64(r),float64(g),float64(b))
		h,c,l:=col.Hcl()                                       // remember original luminance
		if ((from<=to) && (h>float64(from) && h<float64(to))) ||
		   ((from> to) && (h>float64(from) || h<float64(to))) {  // if hue in given range (e.g. purples 295..30)
			c=math.Max(0.0, math.Min(1.0, c*float64(factor)))  // scale chroma (e.g. zero out)
			col=colorful.Hcl(h, c, l).Clamped()
			rr,gg,bb:=col.LinearRgb()
			rs[i], gs[i], bs[i]=float32(rr), float32(gg), float32(bb)
		}
	}
}

// Selectively adjusts CIE HCL chroma for hues in given range by multiplying with given factor. Data must be normalized to [0,1] before.
// Useful for desaturating purple stars
func (f* FITSImage) AdjustChromaForHues(from, to, factor float32) {
	f.ApplyRGBPixelFunction(rgbPFChromaForHues, rgbPFChromaForHuesArgs{from, to, factor})
}


// Arguments for the RGB pixel function to selectively rotate hues in a given range
type rgbPFRotateColorsArgs struct {
	From   float32
	To     float32
	Offset float32
}

// RGB pixel function to selectively rotate hues in a given range. Data must be normalized to [0,1]. 2nd parameter must be a rgbPFRotateColorsArgs
func rgbPFRotateColors(rs,gs,bs []float32, params interface{}) {
	from, to, offset:=params.(rgbPFRotateColorsArgs).From, params.(rgbPFRotateColorsArgs).To, params.(rgbPFRotateColorsArgs).Offset
	for i:=0; i<len(rs); i++ {
		r, g, b:=rs[i], gs[i], bs[i]

		col:=colorful.LinearRgb(float64(r),float64(g),float64(b))
		h,c,l:=col.Hcl()                                       // remember original luminance
		if ( from<=to  && (h>float64(from) && h<float64(to))) ||
		   ((from> to) && (h>float64(from) || h<float64(to))) {  // if hue in given range (e.g. greens 100..190)
			h+=float64(offset)                                 // rotate by given amount (e.g yellow with -30)
			col=colorful.Hcl(h, c, l).Clamped()
			rr,gg,bb:=col.LinearRgb()
			rs[i], gs[i], bs[i]=float32(rr), float32(gg), float32(bb)
		}
	}
}

// Selectively rotate hues in a given range. Data must be normalized to [0,1]. 
// Useful to create Hubble palette images from narrowband data, by turning greens to yellows, before applying SCNR
func (f* FITSImage) RotateColors(from, to, offset float32) {
	f.ApplyRGBPixelFunction(rgbPFRotateColors, rgbPFRotateColorsArgs{from, to, offset})
}


// RGB pixel function for subtractive chroma noise reduction on the green color channel. Data must be normalized to [0,1]. 2nd parameter must be a float32
// Uses average neutral masking method with luminance protection
func rgbPFSCNR(rs,gs,bs []float32, params interface{}) {
	factor:=params.(float32)
	for i:=0; i<len(rs); i++ {
		r, g, b:=rs[i], gs[i], bs[i]

		col:=colorful.LinearRgb(float64(r),float64(g),float64(b))
		_,_,l:=col.Hcl()         // remember original luminance

		correctedG:=0.5*(r+b)
		g2:=float32(math.Min(float64(g), float64(correctedG))) // average neutral SCNR
		weightedG:=factor*g2+(1-factor)*g

		// reassemble with luminance protection
		col   =colorful.LinearRgb(float64(r),float64(weightedG),float64(b))
		h,c,_:=col.Hcl()         
		col   =colorful.Hcl(h, c, l).Clamped()
		rr,gg,bb:=col.LinearRgb()
		rs[i], gs[i], bs[i]=float32(rr), float32(gg), float32(bb) 
	}
}

// Apply subtractive chroma noise reduction to the green channel. Data must be normalized to [0,1]. 
// Uses average neutral masking method with luminance protection. Typically used to reduce green cast in narrowband immages when creating Hubble palette images
func (f* FITSImage) SCNR(factor float32) {
	f.ApplyRGBPixelFunction(rgbPFSCNR, factor)
}


// RGB pixel function to replace the current luminance with factor * max(R,G,B) + (1-factor) * luminance
func rgbPFLumChannelMax(rs,gs,bs []float32, params interface{}) {
	factor:=params.(float32)
	for i:=0; i<len(rs); i++ {
		r, g, b:=rs[i], gs[i], bs[i]

		col:=colorful.LinearRgb(float64(r),float64(g),float64(b))
		h,c,l:=col.Hcl()         // remember original luminance

		channelMax:=math.Max(math.Max(float64(r), float64(g)), float64(b))
		correctedL:=channelMax*float64(factor) + l*(1-float64(factor))

		// reassemble with luminance protection
		col   =colorful.Hcl(h, c, correctedL).Clamped()
		rr,gg,bb:=col.LinearRgb()
		rs[i], gs[i], bs[i]=float32(rr), float32(gg), float32(bb) 
	}
}

// replace the current luminance with factor * max(R,G,B) + (1-factor) * luminance. Data must be normalized to [0,1]. 
func (f* FITSImage) LumChannelMax(factor float32) {
	f.ApplyRGBPixelFunction(rgbPFLumChannelMax, factor)
}



/////////////////////////////////////////////////////////
// Simple, I/O-limited pixel operations. Not parallelized
/////////////////////////////////////////////////////////

// Adjust image data to match the histogram shape of refStats.
// Assumes f.Stats are current; and updates them afterwards.
func (f *FITSImage) MatchHistogram(refStats *BasicStats) {
	multiplier:=refStats.Scale    / f.Stats.Scale
	offset    :=refStats.Location - f.Stats.Location*multiplier
	data:=f.Data
	for i, d:=range data {
		data[i]=d*multiplier + offset
	}

	// optimization, so we don't have to recompute f.Stats=CalcExtendedStats(f.Data, f.Naxisn[0])
	f.Stats.Min     =f.Stats.Min     *multiplier+offset
	f.Stats.Max     =f.Stats.Max     *multiplier+offset
	f.Stats.Mean    =f.Stats.Mean    *multiplier+offset
	f.Stats.StdDev  =f.Stats.Mean    *multiplier
	f.Stats.Location=f.Stats.Location*multiplier+offset
	f.Stats.Scale   =f.Stats.Scale   *multiplier
}


// Offsets each color channel by a factor, clamping to  Operates in-place on image data normalized to [0,1]. 
func (f* FITSImage) OffsetRGB(r, g, b float32) {
	l:=len(f.Data)/3
	data:=f.Data
	for i, d:=range data[   :  l] {
		data[i    ]=d+r
	}
	for i, d:=range data[  l:2*l] {
		data[i+  l]=d+g
	}
	for i, d:=range data[2*l:   ] {
		data[i+2*l]=d+b
	}
}


// Scales each color channel by a factor, clamping to  Operates in-place on image data normalized to [0,1]. 
func (f* FITSImage) ScaleRGB(r, g, b float32) {
	l:=len(f.Data)/3
	data:=f.Data
	for i, d:=range data[   :  l] {
		data[i    ]=float32(math.Min(1, float64(d*r)))
	}
	for i, d:=range data[  l:2*l] {
		data[i+  l]=float32(math.Min(1, float64(d*g)))
	}
	for i, d:=range data[2*l:   ] {
		data[i+2*l]=float32(math.Min(1, float64(d*b)))
	}
}


// Shift black point so a defined before value becomes the given after value. Operates in-place on image data normalized to [0,1]. 
func (f* FITSImage) ShiftBlackToMove(before, after float32) {
    // Plug after and before into the transformation formula:
    //   after = (before - black) / (1 - black)
    // Then solving for black yields the following
    black:=(after-before)/(after-1)
    scale:=1/(1-black)
	data:=f.Data
	for i, d:=range data {
		data[i]=float32(math.Max(0, float64((d-black)*scale)))
	}
}

// Linearly transforms each color channel with multiplier alpha and offset beta, then clamps result to [0,1]
func (f* FITSImage) ScaleOffsetClampRGB(alphaR, betaR, alphaG, betaG, alphaB, betaB float32) {
	l:=len(f.Data)/3
	data:=f.Data
	for i, r:=range data[   :  l] {
		data[i    ]=float32(math.Max(math.Min(1, float64(alphaR*r+betaR)),0))
	}
	for i, g:=range data[  l:2*l] {
		data[i+  l]=float32(math.Max(math.Min(1, float64(alphaG*g+betaG)),0))
	}
	for i, b:=range data[2*l:   ] {
		data[i+2*l]=float32(math.Max(math.Min(1, float64(alphaB*b+betaB)),0))
	}
}

// Sets black point and white point of the image to clip the given percentage of pixels.
func (f* FITSImage) SetBlackWhite(blackPerc, whitePerc float32) {
	data:=f.Data
	l:=len(data)

	// calculate min, max and histogram
	min, max:=float32(math.MaxFloat32), float32(-math.MaxFloat32)
	for _, d:=range data {
		if d<min { min=d }
		if d>max { max=d }
	}	
	hist:=make([]int32,65536)
	Histogram(data, min, max, hist)

	// calculate black level
	blackPixels, blackIndex:=int32(0), int32(0)
	for i:=0; i<l; i++ {
		h:=hist[i]
		if (blackPixels+h)>int32(blackPerc*0.01*float32(l)) {
			blackIndex=int32(i)
			break
		}
		blackPixels+=h
	}
	blackX:=min+(float32(blackIndex)+0.5)*(max-min)/float32(len(hist)-1)

	// calculate white level
	whitePixels, whiteIndex:=int32(0), int32(0)
	for i:=len(hist)-1; i>=0; i-- {
		h:=hist[i]
		if (whitePixels+h)>int32(whitePerc*0.01*float32(l)) {
			whiteIndex=int32(i)
			break
		}
		whitePixels+=h
	}
	whiteX:=min+(float32(whiteIndex)+0.5)*(max-min)/float32(len(hist)-1)
	hist=nil

	// apply black and white point correction
	for i,d:=range data {
		d=(d-blackX)/(whiteX-blackX)
		d=float32(math.Min(math.Max(0, float64(d)), 1))
		data[i]=d
	}
    LogPrintf("Black point is %.4g (%.4g%% clipped), white point %.4g (%.4g%%)\n",
                blackX, 100.0*float32(blackPixels)/float32(l), whiteX, 100.0*float32(whitePixels)/float32(l))
}
