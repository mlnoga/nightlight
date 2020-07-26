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


// Check if coordinate is within [0, size-1], and if not, reflect out of bounds coordinates back into the value range
func reflect(size, x int) int {
    if(x < 0) {
      return -x - 1;
    }
    if(x >= size) {
      return 2*size - x - 1;
    }
    return x;
}


// Returns the definite integral of the gaussian function with midpoint mu and standard deviation sigma for input x
func GaussianDefiniteIntegral(mu, sigma, x float32) float32 {
    return 0.5 * (1 + float32(math.Erf(   float64((x-mu)/(sqrt2 * sigma)) )) )
}

// Generates a 1D gaussian kernel for the given sigma. Based on symbolic integration via error function
func GaussianKernel1D(sigma float32) (kernel []float32) {
    mu          :=float32(0)

    // Find minimal kernel width for which the area under the curve left of the kernel is below the acceptable error
    acceptOut   :=float32(0.01)
    radius      :=0
    for {
        val:=GaussianDefiniteIntegral(mu, sigma, float32(-0.5)-float32(radius))
        if val < acceptOut { 
            radius--
            break 
        }
        radius++ 
    }
    width       :=2*radius+1
    kernel       =make([]float32, width)

    // Calculate left half of the kernel via symbolic integration
    sum         :=float32(0)
    lower       :=GaussianDefiniteIntegral(mu, sigma, float32(-0.5)-float32(radius)             )
    for i:=0; i<=radius; i++ {
        upper   :=GaussianDefiniteIntegral(mu, sigma, float32(-0.5)-float32(radius)+float32(i+1))
        delta   :=upper - lower
        kernel[i]=delta
        sum     +=delta
        lower    =upper
    }

    // Mirror right half of the kernel to avoid numeric instability
    for i:=1; i<=radius; i++ {
        value             := kernel[radius - i]
        kernel[radius + i] = value
        sum               += value
    }

    // Normalize the sum of the kernel to 1, for dealing with the truncated part of the distribution.
    factor:=1.0/sum
    for i,_:=range(kernel) { kernel[i]*=factor }
    return kernel
}


// Convolve the given 2D image provided by data and with with the given convolution kernel along the x axis, and store the result in res
func Convolve1DX(res, data []float32, width int, kernel []float32) {
    height:=len(data)/width    
    k := len(kernel) / 2
    for y:=0; y<height; y++ {
        for x:=0; x<width; x++ {
            sum := float32(0.0)
            for i := -k; i <=k; i++ {
                x1 := reflect(width, x+i)
                sum+= data[y*width+x1]*kernel[i+k]
            }
            res[y*width+x] = sum
        }
    }
}

// Convolve the given 2D image provided by data and with with the given convolution kernel along the y axis, and store the result in res
func Convolve1DY(res, data []float32, width int, kernel []float32) {
    height:=len(data)/width    
    k := len(kernel) / 2
    for y:=0; y<height; y++ {
        for x:=0; x<width; x++ {
            sum := float32(0.0)
            for i := -k; i <=k; i++ {
                y1 := reflect(height, y+i)
                sum+= data[y1*width+x]*kernel[i+k]
            }
            res[y*width+x] = sum
        }
    }
}

// Generate a convolution kernel for a 2D gauss filter of given standard deviation, and applies it to the 2D image given by data and width.
// Overwrites tmp and returns the result in res. 
func GaussFilter2D(res, tmp, data[] float32, width int, sigma float32) {
    kernel:=GaussianKernel1D(sigma)
    Convolve1DX(tmp, data, width, kernel)    
    Convolve1DY(res, tmp,  width, kernel)
}


// Applies unsharp mask to 2D image given bz data and width, using provided radius for Gauss filter and gain for combination.
// Results are clipped to min..max. Pixels below the threshold are left unchanged. Overwrites tmp, and returns the result in res
func ApplyUnsharpMask(res, data, blurred []float32, gain float32, min, max, absThreshold float32) {
    for i, d:=range data {
        if d<absThreshold {
            res[i]=d
        } else {
            r:=d + (d-blurred[i])*gain
            if r<min { r=min }
            if r>max { r=max }
            res[i]=r
        }
    }    
}


// Applies unsharp mask to 2D image given bz data and width, using provided radius for Gauss filter and gain for combination.
// Results are clipped to min..max. Pixels below the threshold are left unchanged. Returns results in a newly allocated array
func UnsharpMask(data []float32, width int, sigma float32, gain float32, min, max, absThreshold float32) []float32 {
    tmp:=make([]float32, len(data))
    blurred:=make([]float32, len(data))
    GaussFilter2D(blurred, tmp, data, width, sigma)
    ApplyUnsharpMask(tmp, data, blurred, gain, min, max, absThreshold)
    return tmp
}
