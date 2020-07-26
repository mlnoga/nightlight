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
	"testing"
	"math"
)

type gaussianKernel1DTestCase struct {
	Sigma   float32
	Kernel  []float32 
}

func TestGaussianKernel1D(t *testing.T) {
	epsilon:=1e-5
	tcs:=[]gaussianKernel1DTestCase{
		gaussianKernel1DTestCase{1.0, []float32{0.27901, 0.44198, 0.27901}},
		gaussianKernel1DTestCase{2.0, []float32{0.028532, 0.067234, 0.124009, 0.179044, 0.20236, 0.179044, 0.124009, 0.067234, 0.028532}},
		gaussianKernel1DTestCase{3.0, []float32{0.018816, 0.034474, 0.056577, 0.083173, 0.109523, 0.129188, 0.136498, 0.129188, 0.109523, 
		                                        0.083173, 0.056577, 0.034474, 0.018816}},
	}

	for _,tc:=range tcs {
		sigma :=tc.Sigma
		kernel:=GaussianKernel1D(sigma)
		sum   :=float32(0)
		for i,k :=range(kernel) { 
			if math.Abs(float64(k-tc.Kernel[i]))>epsilon { t.Errorf("sigma=%f k[%d]=%f; want %f", sigma, i, k, tc.Kernel[i]) }
			sum+=k 
		}
		if math.Abs(float64(sum-1))>epsilon { t.Errorf("sigma=%f sum=%f; want 1", sigma, sum) }
	}
}

