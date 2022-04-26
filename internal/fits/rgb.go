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
	"github.com/mlnoga/nightlight/internal/stats"
)

// A RGB color in float32
type RGB struct {
	R float32
	G float32
	B float32
}


// Print RGB color as a human-readable string
func (rgb RGB) String() string {
	return fmt.Sprintf("RGB(%.2f%%, %.2f%%, %.2f%%)", rgb.R*100, rgb.G*100, rgb.B*100)
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
	hsl.Exposure+=lum.Exposure
}


// Set image black point iteratively. First match histogram scale and location among the channels.
// Then find the darkest block, and set it to the desired color; and find the average star color,
// and set it to the desired color  
func (f *Image) SetBlackWhitePoints(block int32, border, skipBright, skipDim float32, shadows, highlights RGB, logWriter io.Writer) error {
	// Estimate location (=histogram peak, background black point) per color channel
	statsR:=stats.NewStatsForChannel(f.Data, f.Naxisn[0], 0, 3)
	statsG:=stats.NewStatsForChannel(f.Data, f.Naxisn[0], 1, 3)
	statsB:=stats.NewStatsForChannel(f.Data, f.Naxisn[0], 2, 3)

	loc   :=RGB{ statsR.Location(), statsG.Location(), statsB.Location() }
	scaled:=RGB { loc.R+statsR.Scale()*3, loc.G+statsG.Scale()*3, loc.B+statsB.Scale()*3 }
	fmt.Fprintf(logWriter, "Location is %s and loc+3 sigma is %s\n", loc, scaled)

	f.setBlackWhitePoints(loc, scaled, shadows, highlights, logWriter)

	// 2nd pass
	//
	statsR=stats.NewStatsForChannel(f.Data, f.Naxisn[0], 0, 3)
	statsG=stats.NewStatsForChannel(f.Data, f.Naxisn[0], 1, 3)
	statsB=stats.NewStatsForChannel(f.Data, f.Naxisn[0], 2, 3)

	darkest:= f.findDarkestBlock(block, border)
	clip   := float32(0.9)
	stars  := f.meanStarIntensity(skipBright, skipDim, RGB{statsR.Max()*clip, statsG.Max()*clip, statsB.Max()*clip})
	fmt.Fprintf(logWriter, "Darkest block is %s and mean star color is %s\n", darkest, stars)

	f.setBlackWhitePoints(darkest, stars, shadows, highlights, logWriter)

	return nil
}


// Sets black point and white point once, maintaining brightness of the current shadows and current highlights,
// but adjusting the color tint towards the target
func (f *Image) setBlackWhitePoints(curShadows, curHighlights, targetShadows, targetHighlights RGB, logWriter io.Writer) {
	// Pick average shadow color as new shadow (avoid degenerated colors)
	newShadow:=(curShadows.R+curShadows.G+curShadows.B)/3
	newShadows:=RGB { targetShadows.R*newShadow, targetShadows.G*newShadow, targetShadows.B*newShadow }
	//locNewR, locNewG, locNewB := float32(1.0), float32(1.03), float32(1.01)

	// Pick average higlight color as new highlight color (avoid degenerated colors)
    newHighlight:= (curHighlights.R+curHighlights.G+curHighlights.B)/3 
    newHighlights:=RGB { targetHighlights.R*newHighlight, targetHighlights.G*newHighlight, targetHighlights.B*newHighlight }
	// starNewR, starNewG, starNewB := float32(0.902), float32(0.99), float32(0.955)

	// Calculate multiplicative correction factors
	alphaR:=(newHighlights.R - newShadows.R) / (curHighlights.R - curShadows.R)
	alphaG:=(newHighlights.G - newShadows.G) / (curHighlights.G - curShadows.G)
	alphaB:=(newHighlights.B - newShadows.B) / (curHighlights.B - curShadows.B)

	// Calculate additive correction factors 
	betaR:=newShadows.R - alphaR * curShadows.R
	betaG:=newShadows.G - alphaG * curShadows.G
	betaB:=newShadows.B - alphaB * curShadows.B

	// Apply the correction factors
	fmt.Fprintf(logWriter, "r=%.3f*r %+.1f%%, g=%.1f*g %+.3f%%, b=%.3f*b %+.1f%%\n", alphaR, betaR*100, alphaG, betaG*100, alphaB, betaB*100)
	f.ScaleOffsetClampRGB(alphaR, betaR, alphaG, betaG, alphaB, betaB)
}


// finds mean color of the darkest block in a given color image
func (f *Image) findDarkestBlock(blockSize int32, border float32) RGB {
  width:=f.Naxisn[0]
  height:=f.Naxisn[1]
  channelSize:=width*height

  xBlockFirst:=( int32(float32(width)*border) / blockSize ) * blockSize
  xBlockLast:= ( ( width - xBlockFirst ) / blockSize ) * blockSize

  yBlockFirst:=( int32(float32(height)*border) / blockSize ) * blockSize
  yBlockLast:= ( ( height - yBlockFirst ) / blockSize ) * blockSize
  invBlockPixels:=1.0/float32(blockSize*blockSize)

  rMin, gMin, bMin, lMin := 
    float32(math.MaxFloat32),
    float32(math.MaxFloat32),
    float32(math.MaxFloat32),
    float32(math.MaxFloat32)

  // for all blocks
  for yBlock:=yBlockFirst; yBlock<yBlockLast; yBlock+=blockSize {
    yBlockEnd:=yBlock+blockSize 
    for xBlock:=xBlockFirst; xBlock<xBlockLast; xBlock+=blockSize {
      xBlockEnd:=xBlock+blockSize
      
      // sum red channel in block
      r:=float32(0)
      for y:=yBlock; y<yBlockEnd; y++ {
        rowSum:=float32(0)
        for x:=xBlock; x<xBlockEnd; x++ {
          rowSum+=f.Data[x+width*y]
        }
        r+=rowSum
      }
      r*=invBlockPixels
   
     // sum green channel
     g:=float32(0)
      for y:=yBlock; y<yBlockEnd; y++ {
        rowSum:=float32(0)
        for x:=xBlock; x<xBlockEnd; x++ {
          rowSum+=f.Data[x+width*y+channelSize]
        }
        g+=rowSum
      }
      g*=invBlockPixels

     // sum blue channel
     b:=float32(0)
      for y:=yBlock; y<yBlockEnd; y++ {
        rowSum:=float32(0)
        for x:=xBlock; x<xBlockEnd; x++ {
          rowSum+=f.Data[x+width*y+2*channelSize]
        }
        b+=rowSum
      }
      b*=invBlockPixels

       // estimate luminance and keep darkest
       l := (r + g + b) / 3 // 0.2126 * r + 0.7152 * g + 0.0722 * b
       if l<lMin {
         rMin, gMin, bMin, lMin = r, g, b, l
       }
     }
  }

  return RGB{rMin, gMin, bMin}
}


// Returns mean color for the stars in the given RGB image
func (f *Image) meanStarIntensity(skipBright, skipDim float32, clip RGB) RGB {
	if len(f.Stars)==0 { return RGB{0, 0, 0} }

	sStart:=             int(float32(len(f.Stars))*skipBright)
	sEnd  :=len(f.Stars)-int(float32(len(f.Stars))*skipDim)
	if sStart>=sEnd { return RGB{0, 0, 0} }

	width:=f.Naxisn[0]
	height:=f.Naxisn[1]
	channelSize:=width*height

	totalR, totalG, totalB, totalPixels:=float32(0), float32(0), float32(0), int32(0)

	// For each star
	for _, s:=range f.Stars[sStart:sEnd] { 
		starX,starY:=s.Index%width, s.Index/width
		hfr:=s.HFR*0.75
		hfrR:=int32(hfr+0.5)
		hfrSq:=(hfr+0.01)*(hfr+0.01)

		starR, starG, starB, starPixels:=float32(0), float32(0), float32(0), int32(0)

		// For all pixels in this star
		for offY:=-hfrR; offY<=hfrR; offY++ {
			y:=starY+offY
			if y>=0 && y<height {
				for offX:=-hfrR; offX<=hfrR; offX++ {
					x:=starX+offX
					if x>=0 && x<width {
						distSq:=float32(offX*offX+offY*offY)
						if distSq<=hfrSq { 
							// check for color clipping
							r := f.Data[y*width+x]
							g := f.Data[y*width+x + channelSize]
							b := f.Data[y*width+x + channelSize*2]
							if r<clip.R && g<clip.G && b<clip.B {
								// accumulate pixel values for the star
								starR+=r
								starG+=g
								starB+=b
								starPixels++
							}
						}
					}
				}
			}
		}

		// accumulate total pixel values
		totalR+=starR
		totalG+=starG
		totalB+=starB
		totalPixels+=starPixels
	}

	// normalize and return totals
	norm:=1.0/float32(totalPixels)
	return RGB{totalR*norm, totalG*norm, totalB*norm}
}
