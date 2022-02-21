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

// +build !amd64

package median


// Applies 3x3 median filter to input data, assumed to be a 2D array with given line width, and stores results in output.
// Copies over the outermost rows and columns unchanged.
func MedianFilter3x3(output, data []float32, width int32) {
    medianFilter3x3PureGo(output, data,width)
}
