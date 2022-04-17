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


package qsort

import (
	"testing"
	"github.com/valyala/fastrand"
)


func TestMedian(t *testing.T) {
	rng:=fastrand.RNG{}
	for i:=1; i<1000; i++ {
		// prepare array of given length with a random permutation of 1..n
		arr:=make([]float32, i)
		for j:=0; j<len(arr); j++ {
			arr[j]=float32(j+1)
		}
		for j:=0; j<len(arr); j++ {
			k:=rng.Uint32n(uint32(len(arr)))
			arr[j], arr[k] = arr[k], arr[j]
		}

		// calculate expected result
		var expect float32
		if (i&1)!=0 {
			expect=float32((i+1)/2)
		} else {
			expect=0.5*(float32(i/2) + float32(i/2+1))
		}

		// calculate actual result and compare
		res:=QSelectMedianFloat32(arr)
		if res!=expect {
			t.Logf("median(1..%d) got %f expect %f\n", i ,res, expect)
			t.Fail()
		}
	}
}
