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


package median

import (
	"math"
	"github.com/mlnoga/nightlight/internal/qsort"
)

// Applies 3x3 median filter to input data, assumed to be a 2D array with given line width, and stores results in output.
// Copies over the outermost rows and columns unchanged. Pure go implementation
func medianFilter3x3PureGo(output, data []float32, width int32) {
	height:=len(data)/int(width)
	copy(output[:width], data[:width])                       // copy first row

	for line:=int(0); line<height-2; line++ {
		start, end:=line*int(width), (line+3)*int(width)

		output[start+int(width)]=data[start+int(width)]                // copy first column
		MedianFilterLine3x3PureGo(output[start:end], data[start:end], width)
		output[start+2*int(width)-1]=data[start+2*int(width)-1]        // copy last column
	}
	copy(output[(height-1)*int(width):], data[(height-1)*int(width):]) // copy last row
}


// Input data is three lines of given width. Applies a 3x3 median filter to these.
// Stores results in the middle row of the output, which must have the same shape as the input.
// Does not touch first and last column
func MedianFilterLine3x3PureGo(output, data []float32, width int32) {
	var gathered=[]float32{0,0,0,0,0,0,0,0,0}

	for i:=width+1; i<2*width-1; i++ {
		ioff:=i-width-1
		j:=0
		gathered[j]=data[ioff]
		ioff++
		j++
		gathered[j]=data[ioff]
		ioff++
		j++
		gathered[j]=data[ioff]
		ioff+=width-2
		j++
		gathered[j]=data[ioff]
		ioff++
		j++
		gathered[j]=data[ioff]
		ioff++
		j++
		gathered[j]=data[ioff]
		ioff+=width-2
		j++
		gathered[j]=data[ioff]
		ioff++
		j++
		gathered[j]=data[ioff]
		ioff++
		j++
		gathered[j]=data[ioff]
		output[i]=MedianFloat32Slice9(gathered)
	}	
}


// Calculates the median of a float32 slice of length nine
// Modifies the elements in place
// From https://stackoverflow.com/questions/45453537/optimal-9-element-sorting-network-that-reduces-to-an-optimal-median-of-9-network
// See also http://ndevilla.free.fr/median/median/src/optmed.c for other sizes
// Array must not contain IEEE NaN
func MedianFloat32Slice9(a []float32) float32 {       // 30x min/max
    // function swap(i,j) {var tmp = MIN(a[i],a[j]); a[j] = MAX(a[i],a[j]); a[i] = tmp;}
    // function min(i,j) {a[i] = MIN(a[i],a[j]);}
    // function max(i,j) {a[j] = MAX(a[i],a[j]);}

    if a[0]>a[1] { a[0], a[1] = a[1], a[0]}  // swap(a,0,1)
    if a[3]>a[4] { a[3], a[4] = a[4], a[3]}  // swap(a,3,4)
    if a[6]>a[7] { a[6], a[7] = a[7], a[6]}  // swap(a,6,7)
    if a[1]>a[2] { a[1], a[2] = a[2], a[1]}  // swap(a,1,2)
    if a[4]>a[5] { a[4], a[5] = a[5], a[4]}  // swap(a,4,5)
    if a[7]>a[8] { a[7], a[8] = a[8], a[7]}  // swap(a,7,8)
    if a[0]>a[1] { a[0], a[1] = a[1], a[0]}  // swap(a,0,1)
    if a[3]>a[4] { a[3], a[4] = a[4], a[3]}  // swap(a,3,4)
    if a[6]>a[7] { a[6], a[7] = a[7], a[6]}  // swap(a,6,7)
    if a[0]>a[3] { a[3]       = a[0]      }  // max (a,0,3)
    if a[3]>a[6] { a[6]       = a[3]      }  // max (a,3,6)
    if a[1]>a[4] { a[1], a[4] = a[4], a[1]}  // swap(a,1,4)
    if a[4]>a[7] { a[4]       = a[7]      }  // min (a,4,7)
    if a[1]>a[4] { a[4]       = a[1]      }  // max (a,1,4)
    if a[5]>a[8] { a[5]       = a[8]      }  // min (a,5,8)
    if a[2]>a[5] { a[2]       = a[5]      }  // min (a,2,5)
    if a[2]>a[4] { a[2], a[4] = a[4], a[2]}  // swap(a,2,4)
    if a[4]>a[6] { a[4]       = a[6]      }  // min (a,4,6)
    if a[2]>a[4] { a[4]       = a[2]      }  // max (a,2,4)
    return a[4]
}

// Calculates the median of a float32 slice
// Modifies the elements in place
// Array must not contain IEEE NaN
func MedianFloat32(a []float32) float32 {
	if len(a)==0 { return float32(math.NaN()) }
	if len(a)==9 { return MedianFloat32Slice9(a) }
	return qsort.QSelectMedianFloat32(a)
}