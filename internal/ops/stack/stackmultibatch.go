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


package stack

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"runtime"
	"runtime/debug"
	"sort"
	"github.com/pbnjay/memory"
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/ops"
)




type OpStackMultiBatch struct {
	Batch            *OpStackSingleBatch   `json:"batch"`
	Memory            int64           `json:"memory"`
	Save             *ops.OpSave          `json:"save"`
}

func NewOpStackMultiBatch(batch *OpStackSingleBatch, memory int64, save *ops.OpSave) (op *OpStackMultiBatch) {
	return &OpStackMultiBatch{
		Batch       : batch,
		Memory      : memory,
		Save        : save,
	}
}


func (op *OpStackMultiBatch) Apply(opLoadFiles []*ops.OpLoadFile, logWriter io.Writer) (fOut *fits.Image, err error) {
	if len(opLoadFiles)==0 {
		return nil, errors.New("No frames to batch process")
	}
	// Partition the loaders into optimal batches
	opLoadFilesPerm, numBatches, batchSize, maxThreads, err := op.partition(opLoadFiles, logWriter)
	if err!=nil { return nil, err }
	op.Batch.MaxThreads=maxThreads
	
	// Process each batch. The first batch sets the reference image 
	stack:=(*fits.Image)(nil)
	stackFrames:=int64(0)
	stackNoise:=float32(0)
	for b:=int64(0); b<numBatches; b++ {
		// Cut out relevant part of the overall input filenames
		batchStartOffset:= b   *batchSize
		batchEndOffset  :=(b+1)*batchSize
		if batchEndOffset>int64(len(opLoadFilesPerm)) { batchEndOffset=int64(len(opLoadFilesPerm)) }
		batchFrames     :=batchEndOffset-batchStartOffset
		opLoadFilesBatch:=opLoadFilesPerm[batchStartOffset:batchEndOffset]
		fmt.Fprintf(logWriter, "\nStarting batch %d of %d with %d frames...\n", b+1, numBatches, len(opLoadFilesBatch))

		// Stack the files in this batch
		if op.Batch==nil { return nil, errors.New("Missing batch parameters")}
		batch, err:=op.Batch.Apply(opLoadFilesBatch, logWriter)
		if err!=nil { return nil, err }

		// Update stack of stacks
		if numBatches>1 {
			stack=StackIncremental(stack, batch, float32(batchFrames))
			stackFrames+=batchFrames
			stackNoise +=batch.Stats.Noise()*float32(batchFrames)
		} else {
			stack=batch
		}

		// Free memory
		opLoadFilesBatch, batch=nil, nil
		debug.FreeOSMemory()
	}

	// Free more memory; primary frames already freed after stacking
	if op.Batch.PostProcess.Align.Reference    !=nil { op.Batch.PostProcess.Align.Reference   =nil }
	if op.Batch.PreProcess.Calibrate.DarkFrame !=nil { op.Batch.PreProcess.Calibrate.DarkFrame=nil }
	if op.Batch.PreProcess.Calibrate.FlatFrame !=nil { op.Batch.PreProcess.Calibrate.FlatFrame=nil }
	debug.FreeOSMemory()

	if numBatches>1 {
		// Finalize stack of stacks
		StackIncrementalFinalize(stack, float32(stackFrames))

		// Find stars in newly stacked image and report out on them
		stack, err=op.Batch.PreProcess.StarDetect.Apply(stack, logWriter)
		fmt.Fprintf(logWriter, "Overall stack: Stars %d HFR %.2f Exposure %gs %v\n", len(stack.Stars), stack.HFR, stack.Exposure, stack.Stats)

		avgNoise:=stackNoise/float32(stackFrames)
		expectedNoise:=avgNoise/float32(math.Sqrt(float64(numBatches)))
		fmt.Fprintf(logWriter, "Expected noise %.4g from stacking %d batches with average noise %.4g\n",
					expectedNoise, int(numBatches), avgNoise )
	}

	// Save and return
	stack, err=op.Save.Apply(stack, logWriter)
	if err!=nil { return nil, err }

	return stack, nil;
}


func (op *OpStackMultiBatch) partition(opLoadFiles []*ops.OpLoadFile, logWriter io.Writer) (opLoadFilesPerm []*ops.OpLoadFile, 
	                                                                               numBatches, batchSize, maxThreads int64, err error) {
	numFrames:=int64(len(opLoadFiles))
	width, height:=int64(0), int64(0)
	if op.Batch.PreProcess.Calibrate.DarkFrame!=nil {
		width, height=int64(op.Batch.PreProcess.Calibrate.DarkFrame.Naxisn[0]), int64(op.Batch.PreProcess.Calibrate.DarkFrame.Naxisn[1])
	}  else if op.Batch.PreProcess.Calibrate.FlatFrame!=nil {
		width, height=int64(op.Batch.PreProcess.Calibrate.FlatFrame.Naxisn[0]), int64(op.Batch.PreProcess.Calibrate.FlatFrame.Naxisn[1])
	} else if len(opLoadFiles)>0 {
		fmt.Fprintf(logWriter, "\nEstimating memory needs for %d images from %s:\n", numFrames, opLoadFiles[0].FileName)
		first,err:=opLoadFiles[0].Apply(logWriter)
		if err!=nil { return nil, 0,0,0, err }
		width, height=int64(first.Naxisn[0]), int64(first.Naxisn[1])
	} else {
		return nil, 0,0,0, errors.New("No input files to prepare batches")
	}
	pixels:=width*height
	mPixels:=float32(width)*float32(height)*1e-6
	bytes:=pixels*4
	mib:=bytes/1024/1024
	fmt.Fprintf(logWriter, "%d images of %dx%d pixels (%.1f MPixels), which each take %d MiB in-memory as floating point.\n", 
	            numFrames, width, height, mPixels, mib)

	availableFrames:=(int64(op.Memory)*1024*1024)/bytes // rounding down
	maxThreads=int64(runtime.GOMAXPROCS(0))
	fmt.Fprintf(logWriter, "CPU has %d threads. Physical memory is %d MiB, -op.Memory is %d MiB, this fits %d frames.\n", 
		        maxThreads, memory.TotalMemory()/1024/1024, op.Memory, availableFrames)

	// Calculate batch sizes for preprocessing
	for ; maxThreads>=1; maxThreads-- {
		// Besides the lights in the current batch, we need one temp frame per thread,
		// the optional dark and flat, the reference frame from batch 0 (if >1 batches), 
		// and the stack of stacks (if >1 bacthes) 
		batchSize=availableFrames - int64(maxThreads)
		if op.Batch.PreProcess.Calibrate.DarkFrame!=nil { batchSize-- }
		if op.Batch.PreProcess.Calibrate.FlatFrame!=nil { batchSize-- }
		if batchSize<2 { continue }

		// correct for multi-batch memory requirements 
		numBatches=(numFrames+batchSize-1)/batchSize
		if numBatches>1 {
			batchSize-=2	// reference frame from batch 0, and stack of stacks
		}
		if batchSize<2 { continue }
		if batchSize<int64(maxThreads) { continue }
		break
	}
	if maxThreads<1 || batchSize<2 { return nil, 0,0,0, errors.New("Cannot find a stacking execution path within the given memory constraints.") }
	// even out size of the last frame
	for ; (batchSize-1)*numBatches>=numFrames ; batchSize-- {}
	fmt.Fprintf(logWriter, "Using %d batches of batch size %d with %d images in parallel.\n", numBatches, batchSize, maxThreads)

	opLoadFilesPerm=opLoadFiles
	if numBatches>1 {
		perm:=make([]int, len(opLoadFiles))
		for i,_:=range perm {
			perm[i]=i
		}
		fmt.Fprintf(logWriter, "Randomizing input files across batches...\n")
		perm=rand.Perm(len(opLoadFiles))
		for i:=0; i<int(numBatches); i++ {
			from:=i*int(batchSize)
			to  :=(i+1)*int(batchSize)
			if to>len(perm) { to=len(perm) }
			sort.Ints(perm[from:to])
		}
		opLoadFilesPerm:=make([]*ops.OpLoadFile, len(opLoadFiles))
		for i,_:=range opLoadFiles {
			opLoadFilesPerm[i]=opLoadFiles[perm[i]]
		}
	}
	return opLoadFilesPerm, numBatches, batchSize, maxThreads, nil
}



