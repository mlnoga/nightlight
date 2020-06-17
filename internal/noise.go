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
	"math"
)

// Weights for noise estimation
var enWeights []float32 = []float32{
     1, -2,  1,
    -2,  4, -2,
     1, -2,  1,
}

// Estimate the level of gaussian noise on a natural image. Pure Go implementation
// From J. Immerkær, “Fast Noise Variance Estimation”, Computer Vision and Image Understanding, Vol. 64, No. 2, pp. 300-302, Sep. 1996.
func estimateNoisePureGo(data []float32, width int32) float32 {
	var enOffsets []int32 = []int32{
		-width-1, -width  , -width+1,
              -1,        0,        1,
         width-1,  width  ,  width+1, 
    }

    height:=int32(len(data))/width
    sum:=float32(0)
    for y:=int32(1); y<height-1; y++ {
        rowSum:=float32(0)
    	for x:=int32(1); x<width-1; x++ {
    		i:=y*width+x
	    	conv:=float32(0)
	    	for j,o:=range enOffsets {
				conv+=data[i+o]*enWeights[j]
	    	}
	    	rowSum+=float32(math.Abs(float64(conv)))
	    }
        sum+=rowSum
    }
    factor:=float32(math.Sqrt(0.5*math.Pi)) / (6 * float32(width-2) * float32(height - 2))
    return sum*factor
}
