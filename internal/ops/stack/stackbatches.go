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
	"math/rand"
	"runtime"
	"runtime/debug"
	"sort"
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/ops"
)


type OpStackBatches struct {
	ops.OpBase
	PerBatch         *ops.OpSequence   `json:"perBatch"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpStackBatchesDefault() })} // register the operator for JSON decoding

func NewOpStackBatchesDefault() *OpStackBatches { return NewOpStackBatches(ops.NewOpSequence()) }

func NewOpStackBatches(perBatch *ops.OpSequence) (op *OpStackBatches) {
	return &OpStackBatches{
		OpBase      : ops.OpBase{Type:"stackBatches"},
		PerBatch    : perBatch,
	}
}

func (op *OpStackBatches) MakePromises(ins []ops.Promise, c *ops.Context) (outs []ops.Promise, err error) {
	if len(ins)==0 { return nil, errors.New("No frames to batch process") }
	out:=func() (fOut *fits.Image, err error) {
		return op.Apply(ins, c)
	}
	return []ops.Promise{out}, nil
}

func (op *OpStackBatches) Apply(ins []ops.Promise, c *ops.Context) (fOut *fits.Image, err error) {
	// Partition the loaders into optimal batches
	insPerm, numBatches, batchSize, maxThreads, err := op.partition(ins, c)
	if err!=nil { return nil, err }
	c.MaxThreads=int(maxThreads)

	// Process each batch. The first batch sets the reference image 
	stack:=(*fits.Image)(nil)
	stackFrames:=int64(0)
	for b:=int64(0); b<numBatches; b++ {
		// Cut out relevant part of the overall input filenames
		batchStartOffset:= b   *batchSize
		batchEndOffset  :=(b+1)*batchSize
		if batchEndOffset>int64(len(insPerm)) { batchEndOffset=int64(len(insPerm)) }
		batchFrames     :=batchEndOffset-batchStartOffset
		insBatch:=insPerm[batchStartOffset:batchEndOffset]
		fmt.Fprintf(c.Log, "\nStarting batch %d of %d with %d frames...\n", b+1, numBatches, len(insBatch))

		// Stack the files in this batch
		if op.PerBatch==nil { return nil, errors.New("Missing batch parameters")}
		batchPromises, err:=op.PerBatch.MakePromises(insBatch, c)
		if err!=nil { return nil, err }
		if len(batchPromises)!=1 { return nil, errors.New("stacking returned more than one promise")}
		batch, err:=batchPromises[0]()      // materialize the result
		if err!=nil { return nil, err }

		// Update stack of stacks
		if numBatches>1 {
			stack=StackIncremental(stack, batch, float32(batchFrames))
			stackFrames+=batchFrames
		} else {
			stack=batch
		}

		// Free memory
		batch=nil
		debug.FreeOSMemory()
	}

	// Free more memory; primary frames already freed after stacking
	c.DarkFrame, c.FlatFrame = nil, nil
	debug.FreeOSMemory()

	if numBatches>1 {
		// Finalize stack of stacks
		StackIncrementalFinalize(stack, float32(stackFrames))
	}

	return stack, nil;
}


func (op *OpStackBatches) partition(ins []ops.Promise, c *ops.Context) (insPerm []ops.Promise, 
	                                                                    numBatches, batchSize, maxThreads int64, err error) {
	numFrames:=int64(len(ins))
	width, height:=int64(0), int64(0)
	if c.DarkFrame!=nil {
		width, height=int64(c.DarkFrame.Naxisn[0]), int64(c.DarkFrame.Naxisn[1])
	}  else if c.FlatFrame!=nil {
		width, height=int64(c.FlatFrame.Naxisn[0]), int64(c.FlatFrame.Naxisn[1])
	} else if len(ins)>0 {
		first,err:=ins[0]()
		if err!=nil { return nil, 0,0,0, err }
		fmt.Fprintf(c.Log, "\nEstimating memory needs for %d images from %s:\n", numFrames, first.FileName)
		width, height=int64(first.Naxisn[0]), int64(first.Naxisn[1])
	} else {
		return nil, 0,0,0, errors.New("No input files to prepare batches")
	}
	pixels:=width*height
	mPixels:=float32(width)*float32(height)*1e-6
	bytes:=pixels*4
	mib:=bytes/1024/1024
	fmt.Fprintf(c.Log, "%d images of %dx%d pixels (%.1f MPixels), which each take %d MiB in-memory as floating point.\n", 
	            numFrames, width, height, mPixels, mib)

	availableFrames:=(int64(c.StackMemoryMB)*1024*1024)/bytes // rounding down
	maxThreads=int64(runtime.GOMAXPROCS(0))
	fmt.Fprintf(c.Log, "CPU has %d threads. Physical memory is %d MiB, -op.Memory is %d MiB, this fits %d frames.\n", 
		        maxThreads, c.MemoryMB, c.StackMemoryMB, availableFrames)

	// Calculate batch sizes for preprocessing
	for ; maxThreads>=1; maxThreads-- {
		// Besides the lights in the current batch, we need one temp frame per thread,
		// the optional dark and flat, the reference frame from batch 0 (if >1 batches), 
		// and the stack of stacks (if >1 bacthes) 
		batchSize=availableFrames - int64(maxThreads)
		if c.DarkFrame!=nil { batchSize-- }  // FIXME may not be loaded yet...
		if c.FlatFrame!=nil { batchSize-- }  // FIXME may not be loaded yet...
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
	fmt.Fprintf(c.Log, "Using %d random batches of size %d with %d images in parallel.\n", numBatches, batchSize, maxThreads)

	insPerm=ins
	if numBatches>1 {
		perm:=make([]int, len(ins))
		for i,_:=range perm {
			perm[i]=i
		}
		fmt.Fprintf(c.Log, "Randomizing input files into batches...\n")
		perm=rand.Perm(len(ins))
		for i:=0; i<int(numBatches); i++ {
			from:=i*int(batchSize)
			to  :=(i+1)*int(batchSize)
			if to>len(perm) { to=len(perm) }
			sort.Ints(perm[from:to])
		}
		insPerm=make([]ops.Promise, len(ins))
		for i,_:=range ins {
			insPerm[i]=ins[perm[i]]
		}
	}
	return insPerm, numBatches, batchSize, maxThreads, nil
}



