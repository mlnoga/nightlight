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
	"sort"
)


// Split input into required number of randomized batches, given the permissible amount of memory
func PrepareBatches(fileNames []string, stMemory int64) (numBatches, batchSize int64, ids []int, shuffledFileNames []string) {
	LogPrintf("\nEstimating memory needs for %d images from %s:\n", len(fileNames), fileNames[0])
	first:=NewFITSImage()
	first.ReadFile(fileNames[0])
	PutArrayOfFloat32IntoPool(first.Data)
	first.Data=nil
	LogPrintf("Image has %dx%d pixels (%.1f MPixels) and takes %d MB in-memory as floating point.\n", 
	           first.Naxisn[0], first.Naxisn[1], float32(first.Pixels)/1024/1024, int(first.Pixels)*4/1024/1024)

	LogPrintf("Physical memory is %d MB, -stMemory limit for stacking is %d MB.\n", memory.TotalMemory()/1024/1024, stMemory)

	maxBatchSize:=stMemory*1024*1024/4/int64(first.Pixels)
	maxBatchMBs :=int(maxBatchSize)*int(first.Pixels)*4/1024/1024
	LogPrintf("Maximum batch size is %d images which take %d MB for stacking.\n", maxBatchSize, maxBatchMBs )

	numBatches =(int64(len(fileNames))+maxBatchSize-1)/maxBatchSize
	batchSize  =(int64(len(fileNames))+numBatches  -1)/numBatches
	batchMBs  :=int(batchSize)*int(first.Pixels)*4/1024/1024
	LogPrintf("Using %d batch(es) of %d images each, which needs %d MB\n", numBatches, batchSize, batchMBs)

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
	return numBatches, batchSize, perm, fileNames
}