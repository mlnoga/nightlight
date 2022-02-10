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
func BadPixelMap(data []float32, width int32, sigmaLow, sigmaHigh float32) (bpm []int32, medianDiffStats *BasicStats) {
	tmp:=make([]float32,len(data))
	MedianFilter3x3(tmp, data, width)
	Subtract(tmp, data, tmp)

	medianDiffStats=CalcBasicStats(tmp)
	thresholdLow := - medianDiffStats.StdDev * sigmaLow
	thresholdHigh:=   medianDiffStats.StdDev * sigmaHigh
	// LogPrintf("Mediansub stats: %v  threslow: %.2f thresHigh: %.2f\n", stats, thresholdLow, thresholdHigh)

	bpm=make([]int32,len(data)/100)[:0] // []int32{}
	for i, t:=range(tmp) {
		if t<thresholdLow || t>thresholdHigh {
			bpm=append(bpm, int32(i))
		}
	}

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

	return MedianFloat32(buffer)
}


// Computes the element-wise difference of arrays a and b and stores in array c, that is, c[i]=a[i]-b[i]
func Subtract(c, a, b []float32) {
	for i,_ := range(c) {
		c[i]=a[i]-b[i]
	}
}

// Computes the element-wise division of arrays a and b, scaled with bMean and stores in array c, that is, c[i]=a[i]*bMax/b[i]
func Divide(c, a, b []float32, bMax float32) {
	for i,_ := range(c) {
		c[i]=a[i]*bMax/b[i]
	}
}
