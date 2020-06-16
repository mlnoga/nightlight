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
//	"fmt"
	"runtime"  // for NumCPU() on Median()
//	"sort"
	"sync"     // for wait group synchronization on Median()
)


// Generate bad pixel map. Pixels are considered bad if they deviate
// from a local Median filter by more than sigma times the standard
// deviation of the overall differences from the local Median filter.
// Returns an array of indices into the data.
func BadPixelMap(data []float32, width int32, mask []int32, sigmaLow, sigmaHigh float32) (bpm []int32, medianDiffStats *BasicStats) {
	tmp:=GetArrayOfFloat32FromPool(len(data))
//	MedianFilter(tmp, data, mask)
	MedianFilter3x3AVX2(tmp, data, width)

/*	tmp2:=GetArrayOfFloat32FromPool(len(data))
    MedianFilter3x3AVX2(tmp2, data, width)
	diffCount:=0
	for line:=int32(1); line<int32(len(data))/width-1; line++ {
		for col:=int32(1); col<width-1; col++ {
			index:=line*width+col
			if tmp[index]!=tmp2[index] {
				diffCount++
			}
		}
	}
	LogPrintf("diffCount %d\n", diffCount) */

	Subtract(tmp, data, tmp)

	medianDiffStats=CalcBasicStats(tmp)
	thresholdLow := - medianDiffStats.StdDev * sigmaLow
	thresholdHigh:=   medianDiffStats.StdDev * sigmaHigh
	// LogPrintf("Mediansub stats: %v  threslow: %.2f thresHigh: %.2f\n", stats, thresholdLow, thresholdHigh)

	bpm=[]int32{}
	for i, t:=range(tmp) {
		if t<thresholdLow || t>thresholdHigh {
			bpm=append(bpm, int32(i))
		}
	}

	PutArrayOfFloat32IntoPool(tmp)
	tmp=nil
	return bpm, medianDiffStats
}


// Applies an element-wise Median filter to the data with the local neighborhood defined by the mask,
// and stores the result in data
func MedianFilter(output, data []float32, mask []int32) {
	// Parallelize into as many goroutines as we have CPUs
	stepSize:=len(data)/runtime.NumCPU()
	var wg sync.WaitGroup
	wg.Add( (len(data)+stepSize-1)/stepSize )

	// Run the Median operation in parallel
	for step:=0; step<len(data); step+=stepSize {
		go func(start int) {
			defer wg.Done()
			end:=start+stepSize
			if end>len(data) { 
				end=len(data) 
			}
			buffer:=make([]float32, len(mask))
			for i:=start; i<end; i++ {
				output[i]=Median(data, int32(i), mask, buffer)
			}
		}(step)
	}

	wg.Wait() // and wait for all goroutines to finish
}

// assembly implementation in median3x3_avx.s
func MedianFilterLine3x3AVX2(output, data []float32, width int64)

func MedianFilter3x3AVX2(output, data []float32, width int32) {
	height:=len(data)/int(width)
	copy(output[:width], data[:width])                       // copy first row

	for line:=int(0); line<height-2; line++ {
		start, end:=line*int(width), (line+3)*int(width)

		output[start+int(width)]=data[start+int(width)]                // copy first column
		MedianFilterLine3x3AVX2(output[start:end], data[start:end], int64(width))
		output[start+2*int(width)-1]=data[start+2*int(width)-1]        // copy last column
	}
	copy(output[(height-1)*int(width):], data[(height-1)*int(width):]) // copy last row
}

// Applies an element-wise Median filter to the sparse data points provided by the indices,
// with the local neighborhood defined by the mask, and stores the result in data
func MedianFilterSparse(data []float32, indices[]int32, mask []int32) {
	//LogPrintf("applying sparse Median filter to %d indices with mask %v\n", len(indices), mask)
	buffer:=make([]float32, len(mask))
	for _,i := range(indices) {
		data[i]=Median(data, int32(i), mask, buffer)
	}
}


// Applies an element-wise Median filter to the sparse data points provided by the indices,
// with the local neighborhood defined by the mask, and stores the result in data
func ValidateMFS(data []float32, indices[]int32, mask []int32) {
	diffs:=0
	buffer:=make([]float32, len(mask))
	//LogPrintf("mask %v\n", mask)
	for _,i := range(indices) {
		value:=data[i]
		median :=Median(data, i, mask, buffer)
		delta:=value-median
		if delta< -408 || delta>408 {
			diffs++
		}
	}
//	LogPrintf("diffs %d\n", diffs)
}


// Applies an element-wise Median filter to the sparse data points provided by the indices,
// with the local neighborhood defined by the mask, and stores the result in data
func Median(data []float32, index int32, mask []int32, buffer []float32) float32 {
	// buffer:=make([]float32, len(mask))

	// gather the neighborhood of each indexed data point into an array
	num:=0		
	for _,o :=range(mask) {
		indexO:=index+o
		if indexO>=0 && indexO<int32(len(data)) {
			buffer[num]=data[indexO]
			num++
		}
	}

	if num==9 { 
		return MedianFloat32Slice9(buffer)
	} else {
		return QSelectMedianFloat32(buffer[:num])
	}
}


// Computes the element-wise difference of arrays a and b and stores in array c, that is, c[i]=a[i]-b[i]
func Subtract(c, a, b []float32) {
	for i,_ := range(c) {
		c[i]=a[i]-b[i]
	}
}

// Computes the element-wise division of arrays a and b, scaled with bMean and stores in array c, that is, c[i]=a[i]-b[i]
func Divide(c, a, b []float32, bMean float32) {
	for i,_ := range(c) {
		c[i]=a[i]*bMean/b[i]
	}
}
