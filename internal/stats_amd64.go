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
    "github.com/klauspost/cpuid"
)

// Calculate minimum, mean and maximum of given data
func calcMinMeanMax(data []float32) (min, mean, max float32) {
    if cpuid.CPU.AVX2() {
        return calcMinMeanMaxAVX2(data)
    }
    return calcMinMeanMaxPureGo(data)
}

// Calculate minimum, mean and maximum of given data. AVX2 implementation
func calcMinMeanMaxAVX2(data []float32) (min, mean, max float32)


// Calculate variance of given data from provided mean. 
func calcVariance(data []float32, mean float32) (result float64) {
    if cpuid.CPU.AVX2() {
		return calcVarianceAVX2(data, mean)
    }
	return calcVariancePureGo(data, mean)
}

// Calculate variance of given data from provided mean. AVX2 implementation
func calcVarianceAVX2(data []float32, mean float32) (result float64)
