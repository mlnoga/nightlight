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
	"errors"
	"math"
)


func DebayerBilinear(data []float32, width int32, debayer, cfa string) (res []float32, adjWidth int32, err error) {
	// Translate color filter array type into offsets
	// Pattern: RGRGRGRG
	//          GBGBGBGB
	//          RGRGRGRG
	//          GBGBGBGB
	var xOffset, yOffset int32
	switch cfa {
	case "RGGB","rggb": xOffset, yOffset=0,0
	case "GRBG","grbg": xOffset, yOffset=1,0
	case "GBRG","gbrg": xOffset, yOffset=0,1
	case "BGGR","bggr": xOffset, yOffset=1,1
	default: return nil, 0, errors.New("Unknown CFA value "+cfa)
	}

	// Select color channel and debayer
	switch(debayer) {
	case "R","r": 
		res, adjWidth=DebayerBilinearRGGBToRed(data, width, xOffset, yOffset)
		return res, adjWidth, nil
	case "G","g": 
		res, adjWidth=DebayerBilinearRGGBToGreen(data, width, xOffset, yOffset)
		return res, adjWidth, nil
	case "B","b":
		res, adjWidth=DebayerBilinearRGGBToBlue(data, width, xOffset, yOffset)
		return res, adjWidth, nil
	default:      
		return nil, 0, errors.New("Unknown debayering value " + debayer)
	}
}


func DebayerBilinearRGGBToRed(data []float32, width, xOffset, yOffset int32) (rs []float32, adjWidth int32) {
	height   :=int32(len(data))/width
	adjWidth  =(width-xOffset)  & ^1            // ignore last column and row in odd-sized images
	adjHeight:=(height-yOffset) & ^1
	rs        =make([]float32,int(adjWidth)*int(adjHeight))

	/* for row:=int32(0); row<height; row++ {
		LogPrintf("%2d:", row)
		for col:=int32(0); col<width; col++ {
			LogPrintf(" %4.f", data[row*width+col])
		}
		LogPrintln("")
	}

	LogPrintf("\nxOffset %d yOffset %d\n", xOffset, yOffset)
	for row:=int32(0); row<adjHeight; row++ {
		LogPrintf("%2d:", row)
		for col:=int32(0); col<adjWidth; col++ {
			LogPrintf(" %4.f", data[(row+yOffset)*width+col+xOffset])
		}
		LogPrintln("")
	} */

	// for all pixels in adjusted range
	for row:=int32(0); row<adjHeight; row+=2 {
		for col:=int32(0); col<adjWidth; col+=2 {
			srcOffset :=(row+yOffset)*   width +(col+xOffset)
			destOffset:=(row        )*adjWidth +(col        )

			// read relevant red values
			r:=data[srcOffset]
			rRight, rDown, rRD:=r, r, r
			if col+xOffset<width-2 {
				rRight=data[srcOffset+2]
	 			if row+yOffset<height-2 {
	 				rDown=data[srcOffset+  2*width]
	 				rRD  =data[srcOffset+2+2*width]
	 			}			
			} else if row+yOffset<height-2 {
	 				rDown=data[srcOffset+2*width]
			}
			//LogPrintf("r%d c%d: r %.f right %.f down %.f RD %.f\n", row, col, r, rRight, rDown, rRD)

			// interpolate and write red values
			rs[destOffset           ]=      r
			rs[destOffset+1         ]=0.5 *(r+rRight)
 			rs[destOffset  +adjWidth]=0.5 *(r+rDown)
 			rs[destOffset+1+adjWidth]=0.25*(r+rRight+rDown+rRD)
		}
	}

	/* LogPrintf("\nResult %dx%d\n", adjWidth, adjHeight)
	for row:=int32(0); row<adjHeight; row++ {
		LogPrintf("%2d:", row)
		for col:=int32(0); col<adjWidth; col++ {
			LogPrintf(" %4.f", rs[row*adjWidth+col])
		}
		LogPrintln("")
	} */

	return rs, adjWidth
}

const sqrt2 float32 = float32(math.Sqrt2)

func DebayerBilinearRGGBToGreen(data []float32, width, xOffset, yOffset int32) (gs []float32, adjWidth int32) {
	height   :=int32(len(data))/width
	adjWidth  =(width-xOffset)  & ^1            // ignore last column and row in odd-sized images
	adjHeight:=(height-yOffset) & ^1
	gs        =make([]float32,int(adjWidth)*int(adjHeight))

	/* for row:=int32(0); row<height; row++ {
		LogPrintf("%2d:", row)
		for col:=int32(0); col<width; col++ {
			LogPrintf(" %4.f", data[row*width+col])
		}
		LogPrintln("")
	}

	LogPrintf("\nxOffset %d yOffset %d\n", xOffset, yOffset)
	for row:=int32(0); row<adjHeight; row++ {
		LogPrintf("%2d:", row)
		for col:=int32(0); col<adjWidth; col++ {
			LogPrintf(" %4.f", data[(row+yOffset)*width+col+xOffset])
		}
		LogPrintln("")
	} */

	// for all pixels in adjusted range
	for row:=int32(0); row<adjHeight; row+=2 {
		for col:=int32(0); col<adjWidth; col+=2 {
			srcOffset :=(row+yOffset)*   width +(col+xOffset)
			destOffset:=(row        )*adjWidth +(col        )

			// read relevant green values
			g1:=data[srcOffset+1      ]
			g2:=data[srcOffset  +width]

			g1Left:=(2.0  *g1 + sqrt2*g2)*(1.0/(2.0+sqrt2))
			g2Up  :=(sqrt2*g1 + 2.0  *g2)*(1.0/(2.0+sqrt2))
			if col+xOffset>0 {
				g1Left=data[srcOffset-1]
			}
 			if row+yOffset>0 {
 				g2Up=data[srcOffset-width]
 			}			

			g2Right:=(2.0  *g1 + sqrt2*g2)*(1.0/(2.0+sqrt2))
			g1Down :=(sqrt2*g1 + 2.0  *g2)*(1.0/(2.0+sqrt2))
			if col+xOffset<width-2 {
				g2Right=data[srcOffset+2+width]
			}
 			if row+yOffset<height-2 {
 				g1Down=data[srcOffset+1+2*width]
 			}			

			// LogPrintf("r%d c%d: g1 %.f g2 %.f g1Left %.f g1Down %.f g2Up %.f g2Right %.f \n", row, col, g1, g2, g1Left, g1Down, g2Up, g2Right)

			// interpolate and write green values
			gs[destOffset           ]=0.25*(g1+g2+g1Left+g2Up)
			gs[destOffset+1         ]=      g1
 			gs[destOffset  +adjWidth]=         g2
 			gs[destOffset+1+adjWidth]=0.25*(g1+g2+g2Right+g1Down)
		}
	}

	/* LogPrintf("\nResult %dx%d\n", adjWidth, adjHeight)
	for row:=int32(0); row<adjHeight; row++ {
		LogPrintf("%2d:", row)
		for col:=int32(0); col<adjWidth; col++ {
			LogPrintf(" %4.f", gs[row*adjWidth+col])
		}
		LogPrintln("")
	} */

	return gs, adjWidth
}

func DebayerBilinearRGGBToBlue(data []float32, width, xOffset, yOffset int32) (bs []float32, adjWidth int32) {
	height   :=int32(len(data))/width
	adjWidth  =(width-xOffset)  & ^1            // ignore last column and row in odd-sized images
	adjHeight:=(height-yOffset) & ^1
	bs        =make([]float32,int(adjWidth)*int(adjHeight))

	/* for row:=int32(0); row<height; row++ {
		LogPrintf("%2d:", row)
		for col:=int32(0); col<width; col++ {
			LogPrintf(" %4.f", data[row*width+col])
		}
		LogPrintln("")
	}

	LogPrintf("\nxOffset %d yOffset %d\n", xOffset, yOffset)
	for row:=int32(0); row<adjHeight; row++ {
		LogPrintf("%2d:", row)
		for col:=int32(0); col<adjWidth; col++ {
			LogPrintf(" %4.f", data[(row+yOffset)*width+col+xOffset])
		}
		LogPrintln("")
	} */

	// for all pixels in adjusted range
	for row:=int32(0); row<adjHeight; row+=2 {
		for col:=int32(0); col<adjWidth; col+=2 {
			srcOffset :=(row+yOffset)*   width +(col+xOffset)
			destOffset:=(row        )*adjWidth +(col        )

			// read relevant blue values
			b:=data[srcOffset+1+width]
			bLeft, bUp, bLU:=b, b, b
			if col+xOffset>0 {
				bLeft=data[srcOffset-1+width]
	 			if row+yOffset>0 {
	 				bUp=data[srcOffset+1-width]
	 				bLU=data[srcOffset-1-width]
	 			}			
			} else if row+yOffset>0 {
	 				bUp=data[srcOffset+1-width]
			}
			// LogPrintf("r%d c%d: b %.f left %.f up %.f LU %.f\n", row, col, b, bLeft, bUp, bLU)

			// interpolate and write blue values
			bs[destOffset           ]=0.25*(b+bLeft+bUp+bLU)
			bs[destOffset+1         ]=0.5 *(b+bUp)
 			bs[destOffset  +adjWidth]=0.5 *(b+bLeft)
 			bs[destOffset+1+adjWidth]=      b
		}
	}

	/* LogPrintf("\nResult %dx%d\n", adjWidth, adjHeight)
	for row:=int32(0); row<adjHeight; row++ {
		LogPrintf("%2d:", row)
		for col:=int32(0); col<adjWidth; col++ {
			LogPrintf(" %4.f", bs[row*adjWidth+col])
		}
		LogPrintln("")
	} */

	return bs, adjWidth
}

