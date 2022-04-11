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
	"io"
	"math"
	"runtime"
	"github.com/mlnoga/nightlight/internal/stats"
	colorful "github.com/lucasb-eyer/go-colorful"
)


//////////////////////////////////////////////////////////////////
// Complex, CPU-limited pixel operations. Parallelized across CPUs
//////////////////////////////////////////////////////////////////

// A pixel function. Operates in-place. For parallelization across CPUs.
type PixelFunction func(data []float32, params interface{}) 

// A three-channel pixel function. Data must be normalized to [0,1]. Operates in-place. For parallelization across CPUs.
type PixelFunction3Chan func(c0,c1,c2 []float32, params interface{}) 


// Apply given pixel function to the image. Uses thead parallelism across all available CPUs. Operates in-place. 
func (f* Image) ApplyPixelFunction(pf PixelFunction, args interface{}) {
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


// Apply given pixel function to given channel of the image. Uses thead parallelism across all available CPUs. Operates in-place. 
func (f* Image) ApplyPixelFunction1Chan(chanID int, pf PixelFunction, args interface{}) {
	l   :=len(f.Data)/3
	data:=f.Data[chanID*l:(chanID+1)*l]

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


// Apply given pixel function to all channels of the image. Uses thead parallelism across all available CPUs. Data must be normalized to [0,1]. Operates in-place. 
func (f* Image) ApplyPixelFunction3Chan(pf PixelFunction3Chan, args interface{}) {
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
		go func(c0,c1,c2 []float32) {
			pf(c0,c1,c2, args)
			<-sem
		}(data[lower:upper], data[lower+l:upper+l], data[lower+2*l:upper+2*l])
	}

	for i:=0; i<cap(sem); i++ {  // wait for goroutines to finish
		sem <- true
	}
}


type pfScaleOffsetArgs struct {
	Scale   float32
	Offset  float32
}

// Pixel function to apply a scale and an offset. 2nd parameter must be a pfScaleOffsetArgs. Operates in-place. 
func pfScaleOffset(data []float32, params interface{}) {
	scale, offset :=params.(pfScaleOffsetArgs).Scale, params.(pfScaleOffsetArgs).Offset
	for i, d:=range data {
		data[i]=d*scale+offset
	}
}

// Applies given scale factor and offset to image.  Operates in-place. 
func (f* Image) ApplyScaleOffset(scale, offset float32) {
	f.ApplyPixelFunction(pfScaleOffset, pfScaleOffsetArgs{scale, offset})
	f.Stats.UpdateCachedWith(scale, offset)
}

// Applies given scale factor and offset to image.  Operates in-place. 
func (f* Image) ApplyScaleOffsetToChannel(chanID int, scale, offset float32) {
	f.ApplyPixelFunction1Chan(chanID, pfScaleOffset, pfScaleOffsetArgs{scale, offset})
	f.Stats.Clear();
}

// Normalize image to [0..1] based on basic stats.  Operates in-place. 
func (f* Image) Normalize() {
	scale:=1.0/(f.Stats.Max()-f.Stats.Min())
	offset:=-f.Stats.Min()*scale
	f.ApplyScaleOffset(scale, offset)
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
func (f* Image) ApplyGamma(g float32) {
	f.ApplyPixelFunction(pfGamma, g)
	f.Stats.Clear()
}

// Apply gamma correction to image. Image must be normalized to [0,1] before. Operates in-place. 
func (f* Image) ApplyGammaToChannel(chanID int, g float32) {
	f.ApplyPixelFunction1Chan(chanID, pfGamma, g)
	f.Stats.Clear()
}

// Arguments for the RGB pixel function to adjust gamma for a range of intensities
type pfPartialGammaArgs struct {
	From   float32
	To     float32
	Factor float32
}

// Pixel function to apply partial gamma correction to values in given range. Data must be normalized to [0,1]. 2nd parameter must be a pfPartialGammaArgs. Operates in-place. 
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

// Apply gamma correction to image in given range. Image must be normalized to [0,1] before. Operates in-place. 
func (f* Image) ApplyPartialGamma(from, to, g float32) {
	f.ApplyPixelFunction(pfPartialGamma, pfPartialGammaArgs{from, to, g})
	f.Stats.Clear()
}

// Apply gamma correction to given channel of the image. Image must be normalized to [0,1] before. Operates in-place. 
func (f* Image) ApplyPartialGammaToChannel(chanID int, from, to, g float32) {
	f.ApplyPixelFunction1Chan(chanID, pfPartialGamma, pfPartialGammaArgs{from, to, g})
	f.Stats.Clear()
}


// Arguments for the RGB pixel function to adjust midtones
type pfMidtonesArgs struct {
	Mid    float32
	Black  float32
}

// Pixel function to apply midtones correction to given image slice. Data must be normalized to [0,1]. 
// 2nd parameter must be a pfMidtonesArgs. Operates in-place. 
func pfMidtones(data []float32, params interface{}) {
	mid, black:=params.(pfMidtonesArgs).Mid, params.(pfMidtonesArgs).Black
	clipLow   :=black*(mid-1.0) / ((2.0*mid -1.0)*black - mid)
	clipHigh  :=float32(1.0)
	scaler    :=1.0/(clipHigh-clipLow)
	for i, d:=range data {
		value:=d*(mid-1.0) / ((2.0*mid -1.0)*d - mid)
		if value<clipLow { 
			value=0 
	    } else if value>clipHigh {
	    	value=1
	    }
		data[i]=(value-clipLow)*scaler
	}

}

// Apply midtones correction to given image. Data must be normalized to [0,1]. Operates in-place. 
func (f* Image) ApplyMidtones(mid, black float32) {
	f.ApplyPixelFunction(pfMidtones, pfMidtonesArgs{mid, black})
	f.Stats.Clear()
}

// Apply midtones correction to given channel of given image. Data must be normalized to [0,1]. Operates in-place. 
func (f* Image) ApplyMidtonesToChannel(chanID int, mid, black float32) {
	f.ApplyPixelFunction1Chan(chanID, pfMidtones, pfMidtonesArgs{mid, black})
	f.Stats.Clear()
}


// Pixel function to convert a monochromic image to HSLuv Luminance. Data must be normalized to [0,1]. Operates in-place. 
func pfMonoToHSLuvLum(data []float32, params interface{}) {
	for i, d:=range data {
		_,_,lum:=colorful.LinearRgb(float64(d), float64(d), float64(d)).HSLuv()
		data[i]=float32(lum)
	}
}

// Converts a monochromic image to HSLuv Luminance. Data must be normalized to [0,1]. Operates in-place. 
func (f* Image) MonoToHSLuvLum() {
	f.ApplyPixelFunction(pfMonoToHSLuvLum, nil)
	f.Stats.Clear()
}


// Pixel function to convert a monochromic image to HSL Luminance. Data must be normalized to [0,1]. Operates in-place. 
func pfMonoToHSLLum(data []float32, params interface{}) {
	for i, d:=range data {
		_,_,lum:=colorful.LinearRgb(float64(d), float64(d), float64(d)).Hcl()
		data[i]=float32(lum)
	}
}

// Converts a monochromic image to HSL Luminance. Data must be normalized to [0,1]. Operates in-place. 
func (f* Image) MonoToHSLLum() {
	f.ApplyPixelFunction(pfMonoToHSLLum, nil)
	f.Stats.Clear()
}


// Pixel function to convert RGB to CIE HCL pixels. Operates in-place.
func pf3ChanRGBToHCL(rs,gs,bs []float32, params interface{}) {
	for i:=0; i<len(rs); i++ {
		r, g, b:=rs[i], gs[i], bs[i]

		col:=colorful.LinearRgb(float64(r),float64(g),float64(b))
		h,c,l:=col.Hcl()

		rs[i], gs[i], bs[i]=float32(h), float32(c), float32(l) 
	}
}

// Convert RGB to CIE HCL pixels. Operates in-place.
func (f* Image) RGBToHCL() {
	f.ApplyPixelFunction3Chan(pf3ChanRGBToHCL, nil)
	f.Stats.Clear()
}


// Pixel function to convert RGB to CIE HSL pixels. Operates in-place.
// https://en.wikipedia.org/wiki/Colorfulness#Saturation
func pf3ChanRGBToCIEHSL(rs,gs,bs []float32, params interface{}) {
	for i:=0; i<len(rs); i++ {
		r, g, b:=rs[i], gs[i], bs[i]

		col:=colorful.LinearRgb(float64(r),float64(g),float64(b))
		h,c,l:=col.Hcl()
		//s:=c/l
		s:=c/math.Sqrt(c*c+l*l)

		rs[i], gs[i], bs[i]=float32(h), float32(s), float32(l) 
	}
}

// Convert RGB to CIE HSL pixels. Operates in-place.
// https://en.wikipedia.org/wiki/Colorfulness#Saturation
func (f* Image) RGBToCIEHSL() {
	f.ApplyPixelFunction3Chan(pf3ChanRGBToCIEHSL, nil)
	f.Stats.Clear()
}



// Pixel function to convert CIE HSL to RGB pixels. Operates in-place.
// https://en.wikipedia.org/wiki/Colorfulness#Saturation
func pf3ChanCIEHSLToRGB(hs,ss,ls []float32, params interface{}) {
	for i:=0; i<len(hs); i++ {
		h, s, l:=hs[i], ss[i], ls[i]
		//c:=l*s
		c:=l*s/float32(math.Sqrt(float64(1-s*s)))

		col:=colorful.Hcl(float64(h), float64(c), float64(l)).Clamped()
		r,g,b:=col.LinearRgb()

		hs[i], ss[i], ls[i]=float32(r), float32(g), float32(b) 
	}
}

// Convert CIE HSL to RGB pixels. Operates in-place.
// https://en.wikipedia.org/wiki/Colorfulness#Saturation
func (f* Image) CIEHSLToRGB() {
	f.ApplyPixelFunction3Chan(pf3ChanCIEHSLToRGB, nil)
	f.Stats.Clear()
}


// Pixel function to convert RGB to xyY pixels. Operates in-place.
func pf3ChanToXyy(rs,gs,bs []float32, params interface{}) {
	for i:=0; i<len(rs); i++ {
		r, g, b:=rs[i], gs[i], bs[i]

		col:=colorful.LinearRgb(float64(r),float64(g),float64(b))
		x,y,Y:=col.Xyy()

		rs[i], gs[i], bs[i]=float32(x), float32(y), float32(Y) 
	}
}

// Convert RGB to xyY pixels. Operates in-place.
func (f* Image) ToXyy() {
	f.ApplyPixelFunction3Chan(pf3ChanToXyy, nil)
	f.Stats.Clear()
}


// Pixel function to convert Xyy to RGB pixels. Operates in-place.
func pf3ChanXyyToRGB(xs,ys,Ys []float32, params interface{}) {
	for i:=0; i<len(xs); i++ {
		x, y, Y:=xs[i], ys[i], Ys[i]

		col:=colorful.Xyy(float64(x), float64(y), float64(Y)).Clamped()
		r,g,b:=col.LinearRgb()

		xs[i], ys[i], Ys[i]=float32(r), float32(g), float32(b) 
	}
}

// Convert Xyy to RGB pixels. Operates in-place.
func (f* Image) XyyToRGB() {
	f.ApplyPixelFunction3Chan(pf3ChanXyyToRGB, nil)
	f.Stats.Clear()
}


// Pixel function to convert RGB to HSLuv pixels. Operates in-place.
// https://www.hsluv.org/
func pf3ChanRGBToHSLuv(rs,gs,bs []float32, params interface{}) {
	for i:=0; i<len(rs); i++ {
		r, g, b:=rs[i], gs[i], bs[i]

		col:=colorful.LinearRgb(float64(r),float64(g),float64(b))
		h,s,l:=col.HSLuv()

		rs[i], gs[i], bs[i]=float32(h), float32(s), float32(l) 
	}
}

// Convert RGB to HSLuv pixels. Operates in-place.
// https://www.hsluv.org/
func (f* Image) RGBToHSLuv() {
	f.ApplyPixelFunction3Chan(pf3ChanRGBToHSLuv, nil)
	f.Stats.Clear()
}


// Pixel function to convert HSLuv to RGB pixels. Operates in-place.
// https://www.hsluv.org/
func pf3ChanHSLuvToRGB(rs,gs,bs []float32, params interface{}) {
	for i:=0; i<len(rs); i++ {
		h, s, l:=rs[i], gs[i], bs[i]

		// col:=colorful.HSLuv(float64(h),float64(s),float64(l)).Clamped()
		// r,g,b:=col.LinearRgb()
		// rs[i], gs[i], bs[i]=float32(r), float32(g), float32(b)
		
		rs[i], gs[i], bs[i]=HSLuvToLinearRGB(h, s, l) 
	}
}

// default white, unfortunately defined as private in package colorful
var hSLuvD65 = [3]float64{0.95045592705167, 1.0, 1.089057750759878}

// convert HSLuv to linear RGB; faster; with color-preserving clamping
func HSLuvToLinearRGB(h, s, l float32) (r, g, b float32) {
	// HSLuv -> LuvLCh -> CIELUV -> CIEXYZ -> Linear RGB -> sRGB
	ll, u, v := colorful.LuvLChToLuv(colorful.HSLuvToLuvLCh(float64(h), float64(s), float64(l)))
	rr, gg, bb:=colorful.XyzToLinearRgb(colorful.LuvToXyzWhiteRef(ll, u, v, hSLuvD65))
	max:=math.Max(math.Max(rr,gg),bb) // color-preserving clamping, instead of default lightness-preserving
	if max>1 {
		rr/=max
		gg/=max
		bb/=max
	}
	return float32(rr), float32(gg), float32(bb)
}

// Convert HSLuv to RGB pixels. Operates in-place.
// https://www.hsluv.org/
func (f* Image) HSLuvToRGB() {
	f.ApplyPixelFunction3Chan(pf3ChanHSLuvToRGB, nil)
	f.Stats.Clear()
}





type pf3ChanChromaArgs struct {
	Gamma float32
	Threshold float32
}

// Pixel function to apply given gamma correction to color saturation (CIE HCL chroma), for luminances above the given threshold. 
// Data must be normalized to [0,1]. 2nd parameter must be a pf3ChanChromaArgs. Operates in-place. 
func pf3ChanChroma(hs,cs,ls []float32, params interface{}) {
	gamma, threshold:=params.(pf3ChanChromaArgs).Gamma, params.(pf3ChanChromaArgs).Threshold 
	gg:=float64(1.0/gamma)
	for i,l:=range ls {
		if l < threshold { continue }
		cs[i]=float32(math.Pow(float64(cs[i]), gg))
	}
}

//  Apply given gamma correction to color saturation (CIE HCL chroma), for luminances above the given threshold. 
//  Data must be normalized to [0,1]. Operates in-place. 
func (f* Image) AdjustChroma(gamma, threshold float32) {
	f.ApplyPixelFunction3Chan(pf3ChanChroma, pf3ChanChromaArgs{gamma, threshold})
	f.Stats.Clear()
}


type pf3ChanNeutralizeBackgroundArgs struct {
	Low float32
	High float32
}

// RGB pixel function to adjust CIE HCL chroma by multiplying with 0 for values below low, with 1 above high, and interpolating linearly in between. 
// Data must be HCL. 2nd parameter must be a pf3ChanNeutralizeBackgroundArgs. Operates in-place. 
func pf3ChanNeutralizeBackground(hs,cs,ls []float32, params interface{}) {
	low, high:=params.(pf3ChanNeutralizeBackgroundArgs).Low, params.(pf3ChanNeutralizeBackgroundArgs).Low
	scaler:=float32(0)
	if high>low { scaler=1.0/(high-low) }
	for i, l:=range ls {
		if l < low  {
			cs[i]=0
		} else if l<high {
			factor:=(l-low)*scaler
			cs[i]*=factor
		}
	}
}

// Adjust CIE HCL chroma by multiplying with 0 for values below low, with 1 above high, and interpolating linearly in between. 
// Data must be HCL. Operates in-place. 
func (f* Image) NeutralizeBackground(low, high float32) {
	f.ApplyPixelFunction3Chan(pf3ChanNeutralizeBackground, pf3ChanNeutralizeBackgroundArgs{low, high})
	f.Stats.Clear()
}


type pf3ChanChromaForHuesArgs struct {
	From   float32
	To     float32
	Factor float32
}

// RGB pixel function to adjust chroma for a given range of hues. Data must be HCL. 2nd parameter must be a pf3ChanChromaForHuesArgs
func pf3ChanChromaForHues(hs,cs,ls []float32, params interface{}) {
	from, to, factor:=params.(pf3ChanChromaForHuesArgs).From, params.(pf3ChanChromaForHuesArgs).To, params.(pf3ChanChromaForHuesArgs).Factor 
	for i, h:=range hs {
		if ((from<=to) && (h>from && h<to)) ||
		   ((from> to) && (h>from || h<to)) {  // if hue in given range (e.g. purples 295..30)
		   	c:=cs[i]
			c=float32(math.Max(0.0, math.Min(1.0, float64(c*factor))))  // scale chroma (e.g. zero out)
			cs[i]=c
		}
	}
}

// Selectively adjusts CIE HCL chroma for hues in given range by multiplying with given factor. Data must be HCL.
// Useful for desaturating purple stars
func (f* Image) AdjustChromaForHues(from, to, factor float32) {
	f.ApplyPixelFunction3Chan(pf3ChanChromaForHues, pf3ChanChromaForHuesArgs{from, to, factor})
	f.Stats.Clear()
}


// Arguments for the RGB pixel function to selectively rotate hues in a given range
type pf3ChanRotateColorsArgs struct {
	From   float32
	To     float32
	Offset float32
	LThres float32
}

// RGB pixel function to selectively rotate hues in a given range. Data must be HSLuv. 2nd parameter must be a pf3ChanRotateColorsArgs
func pf3ChanRotateColors(hs,ss,ls []float32, params interface{}) {
	from, to, offset, lthres:=params.(pf3ChanRotateColorsArgs).From, params.(pf3ChanRotateColorsArgs).To, params.(pf3ChanRotateColorsArgs).Offset, params.(pf3ChanRotateColorsArgs).LThres
	for i,l:=range ls {
		if l<lthres {
			continue
		}
		h:=hs[i]
		if ( from<=to  && (h>from && h<to)) ||
		   ((from> to) && (h>from || h<to)) {  // if hue in given range (e.g. greens 100..190)
			h+=offset                          // rotate by given amount (e.g yellow with -30)
			hs[i]=h
		}
	}
}

// Selectively rotate hues in a given range. Data must be HSLuv. 
// Useful to create Hubble palette images from narrowband data, by turning greens to yellows, before applying SCNR
func (f* Image) RotateColors(from, to, offset, lthres float32) {
	f.ApplyPixelFunction3Chan(pf3ChanRotateColors, pf3ChanRotateColorsArgs{from, to, offset, lthres})
	f.Stats.Clear()
}


// RGB pixel function for subtractive chroma noise reduction on the green color channel. Data must be HSLuv. 2nd parameter must be a float32
// Uses average neutral masking method with luminance protection
func pf3ChanSCNR(hs,ss,ls []float32, params interface{}) {
	factor:=params.(float32)
	for i:=0; i<len(hs); i++ {
		h,s,l:=hs[i], ss[i], ls[i]
		col  :=colorful.HSLuv(float64(h), float64(s), float64(l)).Clamped()
		r,g,b:=col.LinearRgb()

		correctedG:=0.5*(r+b)
		g2:=float32(math.Min(g, correctedG)) // average neutral SCNR
		weightedG:=factor*g2+(1-factor)*float32(g)

		// reassemble with luminance protection
		col     =colorful.LinearRgb(float64(r),float64(weightedG),float64(b))
		hnew,snew,_:=col.HSLuv()
		hs[i], ss[i]=float32(hnew), float32(snew)        
	}
}

// Apply subtractive chroma noise reduction to the green channel. Data must be normalized to [0,1]. 
// Uses average neutral masking method with luminance protection. Typically used to reduce green cast in narrowband immages when creating Hubble palette images
func (f* Image) SCNR(factor float32) {
	f.ApplyPixelFunction3Chan(pf3ChanSCNR, factor)
	f.Stats.Clear()
}



/////////////////////////////////////////////////////////
// Simple, I/O-limited pixel operations. Not parallelized
/////////////////////////////////////////////////////////

// Adjust image data to match the histogram peak of refStats.
// Assumes f.Stats are current; and updates them afterwards.
func (f *Image) MatchLocation(refLocation float32) {
	multiplier:=refLocation / f.Stats.Location()
	data:=f.Data
	for i, d:=range data {
		data[i]=d*multiplier
	}

	// optimization, so we don't have to recompute f.Stats=CalcExtendedStats(f.Data, f.Naxisn[0])
	f.Stats.UpdateCachedWith(multiplier, 0)
}

// Adjust image data to match the histogram shape of refStats.
// Assumes f.Stats are current; and updates them afterwards.
func (f *Image) MatchHistogram(refStats *stats.Stats) {
	multiplier:=refStats.Scale()    / f.Stats.Scale()
	offset    :=refStats.Location() - f.Stats.Location()*multiplier
	data:=f.Data
	for i, d:=range data {
		data[i]=d*multiplier + offset
	}

	// optimization, so we don't have to recompute f.Stats=CalcExtendedStats(f.Data, f.Naxisn[0])
	f.Stats.UpdateCachedWith(multiplier, offset)
}


// Offsets each color channel by a factor, clamping to  Operates in-place on image data normalized to [0,1]. 
func (f* Image) OffsetRGB(r, g, b float32) {
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
	f.Stats.Clear()
}


// Scales each color channel by a factor, clamping to  Operates in-place on image data normalized to [0,1]. 
func (f* Image) ScaleRGB(r, g, b float32) {
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
	f.Stats.Clear()
}


// Shift black point so a defined before value becomes the given after value. Operates in-place on image data normalized to [0,1]. 
func (f* Image) ShiftBlackToMove(before, after float32) {
    // Plug after and before into the transformation formula:
    //   after = (before - black) / (1 - black)
    // Then solving for black yields the following
    black:=(after-before)/(after-1)
    scale:=1/(1-black)
	data:=f.Data
	for i, d:=range data {
		data[i]=float32(math.Max(0, float64((d-black)*scale)))
	}
	f.Stats.Clear()
}

// Shift black point so a defined before value becomes the given after value. Operates in-place on image data normalized to [0,1]. 
func (f* Image) ShiftBlackToMoveChannel(chanID int, before, after float32) {
    // Plug after and before into the transformation formula:
    //   after = (before - black) / (1 - black)
    // Then solving for black yields the following
    black:=(after-before)/(after-1)
    scale:=1/(1-black)
    l:=len(f.Data)/3
	data:=f.Data[chanID*l:(chanID+1)*l]
	for i, d:=range data {
		data[i]=float32(math.Max(0, float64((d-black)*scale)))
	}
	f.Stats.Clear()
}


// Linearly transforms each color channel with multiplier alpha and offset beta, then clamps result to [0,1]
func (f* Image) ScaleOffsetClampRGB(alphaR, betaR, alphaG, betaG, alphaB, betaB float32) {
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
	f.Stats.Clear()
}

// Sets black point and white point of the image to clip the given percentage of pixels.
func (f* Image) SetBlackWhite(blackPerc, whitePerc float32, logWriter io.Writer) {
	data:=f.Data
	l:=len(data)

	// calculate min, max and histogram
	min, max:=float32(math.MaxFloat32), float32(-math.MaxFloat32)
	for _, d:=range data {
		if d<min { min=d }
		if d>max { max=d }
	}	
	hist:=make([]int32,65536)
	stats.Histogram(data, min, max, hist)

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
	if logWriter!=nil {
	    fmt.Fprintf(logWriter, "Black point is %.4g (%.4g%% clipped), white point %.4g (%.4g%%)\n",
    	            blackX, 100.0*float32(blackPixels)/float32(l), whiteX, 100.0*float32(whitePixels)/float32(l))
    }
	f.Stats.Clear()
}
