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


// Color filter array type, from top to bottom and left to right
type CFAType int
const (
	CFATypeRGGB CFAType = iota
	CFATypeGRBG
	CFATypeGBRG
	CFATypeBGGR
)

// Textual names for color filter array types. Indexed by CFAType
var CFANames=[]string{"RGGB", "GRBG", "GBRG", "BGGR"}


func DebayerBilinearToRed(data []float32, width int32, cfaType CFAType) (rs []float32, adjWidth int32) {
	switch cfaType {
	case CFATypeRGGB:
		return DebayerBilinearRGGBToRed(data, width)
	}
	LogFatalf("Unimplemented CFA type for debayering: %d\n", cfaType)
	return nil, 0 // never reached
}

func DebayerBilinearRGGBToRed(data []float32, width int32) (rs []float32, adjWidth int32) {
	height   :=int32(len(data))/width
	adjWidth  =width  & ^1            // ignore last column and row in odd-sized images
	adjHeight:=height & ^1
	rs        =make([]float32,int(adjWidth)*int(adjHeight))

	// for all pixels in adjusted range
	for row:=int32(0); row<adjHeight; row+=2 {
		for col:=int32(0); col<adjWidth; col+=2 {
			srcOffset :=row*width   +col
			destOffset:=row*adjWidth+col

			// read relevant red values
			r:=data[srcOffset]
			rRight, rDown, rRD:=r, r, r
			if col+2<adjWidth {
				rRight=data[srcOffset+2]
	 			if row+2<adjHeight {
	 				rDown=data[srcOffset+  2*width]
	 				rRD  =data[srcOffset+2+2*width]
	 			}			
			} else if row+2<adjHeight {
	 				rDown=data[srcOffset+2*width]
			}

			// interpolate and write red values
			rs[destOffset           ]=      r
			rs[destOffset+1         ]=0.5 *(r+rRight)
 			rs[destOffset  +adjWidth]=0.5 *(r+rDown)
 			rs[destOffset+1+adjWidth]=0.25*(r+rRight+rDown+rRD)
		}
	}

	return rs, adjWidth
}

func DebayerBilinearRGGBToGreen(data []float32, width int32) (gs []float32, adjWidth int32) {
	height   :=int32(len(data))/width
	adjWidth  =width  & ^1            // ignore last column and row in odd-sized images
	adjHeight:=height & ^1
	gs        =make([]float32,int(adjWidth)*int(adjHeight))

	// for all pixels in adjusted range
	for row:=int32(0); row<adjHeight; row+=2 {
		for col:=int32(0); col<adjWidth; col+=2 {
			srcOffset :=row*width   +col
			destOffset:=row*adjWidth+col

			// read relevant green values
			g1:=data[srcOffset+1      ]
			g2:=data[srcOffset  +width]

			g1Left, g2Up:=g1, 0.5*(g1+g2)
			if col>=2 {
				g1Left=data[srcOffset-1      ]
	 			if row>=2 {
	 				g2Up=data[srcOffset-1-width]
	 			}			
			}
			g2Right, g1Down:=g2, 0.5*(g1+g2)
			if col<adjWidth-2 {
				g2Right=data[srcOffset+2+width]
	 			if row<adjHeight-2 {
	 				g1Down=data[srcOffset+1+2*width]
	 			}			
			}

			// interpolate and write green values
			gs[destOffset           ]=0.25*(g1+g2+g1Left+g2Up)
			gs[destOffset+1         ]=      g1
 			gs[destOffset  +adjWidth]=         g2
 			gs[destOffset+1+adjWidth]=0.25*(g1+g2+g2Right+g1Down)
		}
	}

	return gs, adjWidth
}

func DebayerBilinearRGGBToBlue(data []float32, width int32) (bs []float32, adjWidth int32) {
	height   :=int32(len(data))/width
	adjWidth  =width  & ^1            // ignore last column and row in odd-sized images
	adjHeight:=height & ^1
	bs        =make([]float32,int(adjWidth)*int(adjHeight))

	// for all pixels in adjusted range
	for row:=int32(0); row<adjHeight; row+=2 {
		for col:=int32(0); col<adjWidth; col+=2 {
			srcOffset :=row*width   +col
			destOffset:=row*adjWidth+col

			// read relevant blue values
			b:=data[srcOffset+1+width]
			bLeft, bUp, bLU:=b, b, b
			if col>=2 {
				bLeft=data[srcOffset-1+width]
	 			if row>=2 {
	 				bUp=data[srcOffset+1-width]
	 				bLU=data[srcOffset-1-width]
	 			}			
			} else if row>=2 {
	 				bUp=data[srcOffset+1-width]
			}

			// interpolate and write blue values
			bs[destOffset           ]=0.25*(b+bLeft+bUp+bLU)
			bs[destOffset+1         ]=0.5 *(b+bUp)
 			bs[destOffset  +adjWidth]=0.5 *(b+bLeft)
 			bs[destOffset+1+adjWidth]=      b
		}
	}

	return bs, adjWidth
}

