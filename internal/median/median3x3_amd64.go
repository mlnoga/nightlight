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

// +build amd64

package median

import (
    "github.com/klauspost/cpuid"
)


// Applies 3x3 median filter to input data, assumed to be a 2D array with given line width, and stores results in output.
// Copies over the outermost rows and columns unchanged.
func MedianFilter3x3(output, data []float32, width int32) {
    if cpuid.CPU.AVX2() {
      medianFilter3x3AVX2(output, data, width)
    } else {
      medianFilter3x3PureGo(output, data,width)
    }
}


// Applies 3x3 median filter to input data, assumed to be a 2D array with given line width, and stores results in output.
// Copies over the outermost rows and columns unchanged. AVX2 implementation
func medianFilter3x3AVX2(output, data []float32, width int32) {
	height:=len(data)/int(width)
	copy(output[:width], data[:width])                       // copy first row

	for line:=int(0); line<height-2; line++ {
		start, end:=line*int(width), (line+3)*int(width)

		output[start+int(width)]=data[start+int(width)]                // copy first column
		medianFilterLine3x3AVX2(output[start:end], data[start:end], int64(width))
		output[start+2*int(width)-1]=data[start+2*int(width)-1]        // copy last column
	}
	copy(output[(height-1)*int(width):], data[(height-1)*int(width):]) // copy last row
}


// Input data is three lines of given width. Applies a 3x3 median filter to these.
// Stores results in the middle row of the output, which must have the same shape as the input.
// Does not touch first and last column. AVX2 implementation
func medianFilterLine3x3AVX2(output, data []float32, width int64)

