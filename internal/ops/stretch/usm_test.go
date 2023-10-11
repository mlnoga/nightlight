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

package stretch

import (
	//"fmt"
	"math"
	"testing"
)

type gaussianKernel1DTestCase struct {
	Sigma  float32
	Kernel []float32
}

func TestGaussianKernel1D(t *testing.T) {
	epsilon := 1e-5
	tcs := []gaussianKernel1DTestCase{
		{1.0, []float32{0.27901, 0.44198, 0.27901}},
		{2.0, []float32{0.028532, 0.067234, 0.124009, 0.179044, 0.20236, 0.179044, 0.124009, 0.067234, 0.028532}},
		{3.0, []float32{0.018816, 0.034474, 0.056577, 0.083173, 0.109523, 0.129188, 0.136498, 0.129188, 0.109523,
			0.083173, 0.056577, 0.034474, 0.018816}},
	}

	for _, tc := range tcs {
		sigma := tc.Sigma
		kernel := GaussianKernel1D(sigma)
		sum := float32(0)
		for i, k := range kernel {
			if math.Abs(float64(k-tc.Kernel[i])) > epsilon {
				t.Errorf("sigma=%f k[%d]=%f; want %f", sigma, i, k, tc.Kernel[i])
			}
			sum += k
		}
		if math.Abs(float64(sum-1)) > epsilon {
			t.Errorf("sigma=%f sum=%f; want 1", sigma, sum)
		}
	}
}

func TestGaussFilter2D(t *testing.T) {
	dims := []int{15, 31, 63}
	sigmas := []float32{1.0, 2.0, 3.0}
	epsilon := 1e-5

	for _, dim := range dims {
		for _, sigma := range sigmas {

			width, height := dim, dim
			sharp := make([]float32, width*height)
			peak := float32(9.99)
			sharp[width*(height/2)+width/2] = peak

			//fmt.Println("Sharp");
			//for y:=0; y<height; y++ {
			//	for x:=0; x<width; x++ {
			//		fmt.Printf(" %.2f", sharp[y*width+x])
			//	}
			//	fmt.Println();
			//}

			tmp := make([]float32, width*height)
			blur := make([]float32, width*height)
			kernel := GaussianKernel1D(sigma)
			kHalfSize := len(kernel) / 2
			// fmt.Printf("kernel size %d half size %d data %v\n", len(kernel), kHalfSize, kernel)

			GaussFilter2D(blur, tmp, sharp, width, sigma)

			//fmt.Println("Blurred");
			//for y:=0; y<height; y++ {
			//	for x:=0; x<width; x++ {
			//		fmt.Printf(" %.2f", blur[y*width+x])
			//	}
			//	fmt.Println();
			//}

			sum := float32(0)
			for y := 0; y < height/2-kHalfSize; y++ {
				for x := 0; x < width; x++ {
					if blur[y*width+x] != 0 {
						t.Errorf("sigma=%f b[%d*w+%d]=%f; want 0", sigma, y, x, blur[y*width+x])
					}
					sum += blur[y*width+x]
				}
			}
			for y := height/2 - kHalfSize; y < height/2+kHalfSize+1; y++ {
				for x := 0; x < width/2-kHalfSize-1; x++ {
					if blur[y*width+x] != 0 {
						t.Errorf("sigma=%f b[%d*w+%d]=%f; want 0", sigma, y, x, blur[y*width+x])
					}
					sum += blur[y*width+x]
				}
				for x := width/2 - kHalfSize; x < width/2+kHalfSize+1; x++ {
					if blur[y*width+x] <= 0 || blur[y*width+x] >= peak {
						t.Errorf("sigma=%f b[%d*w+%d]=%f; want >0 <%f", sigma, y, x, blur[y*width+x], peak)
					}
					sum += blur[y*width+x]
				}
				for x := width/2 + kHalfSize + 1; x < width; x++ {
					if blur[y*width+x] != 0 {
						t.Errorf("sigma=%f b[%d*w+%d]=%f; want 0", sigma, y, x, blur[y*width+x])
					}
					sum += blur[y*width+x]
				}
			}
			for y := height/2 + kHalfSize + 1; y < height; y++ {
				for x := 0; x < width; x++ {
					if blur[y*width+x] != 0 {
						t.Errorf("sigma=%f b[%d*w+%d]=%f; want 0", sigma, y, x, blur[y*width+x])
					}
					sum += blur[y*width+x]
				}
			}

			if math.Abs(float64(sum-peak)) > epsilon {
				t.Errorf("sigma=%f sum=%f; want %f", sigma, sum, peak)
			}

		}
	}
}

func TestUnsharpMask(t *testing.T) {
	dims := []int{15}
	sigmas := []float32{1.0, 2.0, 3.0}
	epsilon := 1e-5

	for _, dim := range dims {
		for _, sigma := range sigmas {

			width, height := dim, dim
			sharp := make([]float32, width*height)
			back := float32(10.0)
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					sharp[y*width+x] = back
				}
			}
			peak := float32(15)
			max := float32(20)
			sharp[width*(height/2)+width/2] = peak
			// expectedSum := float32(width)*float32(height)*back + peak - back

			//fmt.Println("Sharp");
			//for y:=0; y<height; y++ {
			//	for x:=0; x<width; x++ {
			//		fmt.Printf(" %.2f", sharp[y*width+x])
			//	}
			//	fmt.Println();
			//}

			blur := UnsharpMask(sharp, width, sigma, 1.0, 0, max, 0)
			kernel := GaussianKernel1D(sigma)
			kHalfSize := len(kernel) / 2

			//fmt.Printf("Unsharp masked sigma %.2f\n", sigma);
			//for y:=0; y<height; y++ {
			//	for x:=0; x<width; x++ {
			//		fmt.Printf(" %.2f", blur[y*width+x])
			//	}
			//	fmt.Println();
			//}

			sum := float32(0)
			for y := 0; y < height/2-kHalfSize; y++ {
				for x := 0; x < width; x++ {
					if math.Abs(float64(blur[y*width+x]-back)) > epsilon {
						t.Errorf("sigma=%f b[%d*w+%d]=%f; want %f", sigma, y, x, blur[y*width+x], back)
					}
					sum += blur[y*width+x]
				}
			}
			for y := height/2 - kHalfSize; y < height/2+kHalfSize+1; y++ {
				for x := 0; x < width/2-kHalfSize-1; x++ {
					if math.Abs(float64(blur[y*width+x]-back)) > epsilon {
						t.Errorf("sigma=%f b[%d*w+%d]=%f; want %f", sigma, y, x, blur[y*width+x], back)
					}
					sum += blur[y*width+x]
				}
				for x := width/2 - kHalfSize; x < width/2+kHalfSize+1; x++ {

					if y == width/2 && x == width/2 {
						if blur[y*width+x] <= peak || blur[y*width+x] > max {
							t.Errorf("sigma=%f b[%d*w+%d]=%f; want >%f <%f", sigma, y, x, blur[y*width+x], peak, max)
						}
					} else {
						if blur[y*width+x] <= 0 || blur[y*width+x] > max {
							t.Errorf("sigma=%f b[%d*w+%d]=%f; want >0 <%f", sigma, y, x, blur[y*width+x], peak)
						}
					}
					sum += blur[y*width+x]
				}
				for x := width/2 + kHalfSize + 1; x < width; x++ {
					if math.Abs(float64(blur[y*width+x]-back)) > epsilon {
						t.Errorf("sigma=%f b[%d*w+%d]=%f; want %f", sigma, y, x, blur[y*width+x], back)
					}
					sum += blur[y*width+x]
				}
			}
			for y := height/2 + kHalfSize + 1; y < height; y++ {
				for x := 0; x < width; x++ {
					if math.Abs(float64(blur[y*width+x]-back)) > epsilon {
						t.Errorf("sigma=%f b[%d*w+%d]=%f; want %f", sigma, y, x, blur[y*width+x], back)
					}
					sum += blur[y*width+x]
				}
			}

			//if math.Abs(float64(sum-expectedSum)) > epsilon {
			//		t.Errorf("sigma=%f sum=%f; want %f", sigma, sum, expectedSum)
			//}
		}
	}
}
