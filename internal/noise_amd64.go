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

package internal

import (
    "math"
    "github.com/klauspost/cpuid"
)

// Estimate the level of gaussian noise on a natural image.
// From J. Immerkær, “Fast Noise Variance Estimation”, Computer Vision and Image Understanding, Vol. 64, No. 2, pp. 300-302, Sep. 1996
func EstimateNoise(data []float32, width int32) float32 {
    if cpuid.CPU.AVX2() {
        return estimateNoiseAVX2(data, width)
    }
    return estimateNoisePureGo(data,width)
}

// Estimate the level of gaussian noise on a natural image. AVX2 implementation for full image
func estimateNoiseAVX2(data []float32, width int32) float32 {
    height:=len(data)/int(width)

    sum:=float32(0)
    for line:=int(0); line<height-2; line++ {
        start, end:=line*int(width), (line+3)*int(width)
        noise:=estimateNoiseLineAVX2(data[start:end], int64(width))
        sum+=noise
    }
    factor:=float32(math.Sqrt(0.5*math.Pi)) / (6 * float32(width-2) * float32(height - 2))
    return sum*factor
}

// AVX2 assembly implementation of noise estimation for a triplet of lines of data
func estimateNoiseLineAVX2(data []float32, width int64) float32
