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
	"github.com/pbnjay/memory"
	"math/rand"
	"runtime"
	"sort"
)


// Split input into required number of randomized batches, given the permissible amount of memory
func PrepareBatches(fileNames []string, stMemory int64, darkF, flatF *FITSImage) (numBatches, batchSize int64, ids []int, shuffledFileNames []string, imageLevelParallelism int32) {
	numFrames:=int64(len(fileNames))
	width, height:=int64(0), int64(0)
	if darkF!=nil {
		width, height=int64(darkF.Naxisn[0]), int64(darkF.Naxisn[1])
	}  else if flatF!=nil {
		width, height=int64(flatF.Naxisn[0]), int64(flatF.Naxisn[1])
	} else {
		LogPrintf("\nEstimating memory needs for %d images from %s:\n", numFrames, fileNames[0])
		first:=NewFITSImage()
		first.ReadFile(fileNames[0])
		PutArrayOfFloat32IntoPool(first.Data)
		first.Data=nil
		width, height=int64(first.Naxisn[0]), int64(first.Naxisn[1])
	}
	pixels:=width*height
	mPixels:=float32(width)*float32(height)*1e-6
	bytes:=pixels*4
	mib:=bytes/1024/1024
	LogPrintf("%d images of %dx%d pixels (%.1f MPixels), which each take %d MiB in-memory as floating point.\n", 
	           numFrames, width, height, mPixels, mib)

	availableFrames:=(int64(stMemory)*1024*1024)/bytes // rounding down
	imageLevelParallelism=int32(runtime.GOMAXPROCS(0))
	LogPrintf("CPU has %d threads. Physical memory is %d MiB, -stMemory is %d MiB, this fits %d frames.\n", imageLevelParallelism, memory.TotalMemory()/1024/1024, stMemory, availableFrames)

	// calculate batch sizes for preprocessing. Need to hold all frames, the light, the dark, and as many temp pictures as we have threads
	outer: 
	for ; imageLevelParallelism>=1; imageLevelParallelism-- {
		// batch size for preprocessing. Need to hold all frames, the light, the dark, and as many temp pictures as we have threads
		preBatchSize:=availableFrames - int64(imageLevelParallelism)
		if darkF!=nil { preBatchSize-- }
		if flatF!=nil { preBatchSize-- }
		if preBatchSize<2 { continue outer }

		batchSize=preBatchSize
		for {
			// correct for memory requirements of stacking: we also need temp storage for all batch stacks on top
			numBatches=(numFrames+batchSize-1)/batchSize
			newBatchSize:=preBatchSize-numBatches
			if newBatchSize<2 { continue outer }
			if newBatchSize==batchSize { break outer }
			batchSize=newBatchSize
		}
	}
	if imageLevelParallelism<1 || batchSize<2 { LogFatal("Cannot find a stacking execution path within the given memory constraints.") }
	// even out size of the last frame
	for ; (batchSize-1)*numBatches>=numFrames ; batchSize-- {}
	LogPrintf("Using %d batches of batch size %d with %d images in parallel.\n", numBatches, batchSize, imageLevelParallelism)

	perm:=make([]int, len(fileNames))
	for i,_:=range perm {
		perm[i]=i
	}
	if numBatches>1 {
		LogPrintf("Randomizing input files across batches...\n")
		perm=rand.Perm(len(fileNames))
		for i:=0; i<int(numBatches); i++ {
			from:=i*int(batchSize)
			to  :=(i+1)*int(batchSize)
			if to>len(perm) { to=len(perm) }
			sort.Ints(perm[from:to])
		}
		old:=fileNames
		fileNames:=make([]string, len(fileNames))
		for i,_:=range fileNames {
			fileNames[i]=old[perm[i]]
		}
	}
	return numBatches, batchSize, perm, fileNames, imageLevelParallelism
}