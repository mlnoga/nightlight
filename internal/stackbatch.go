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
	"fmt"
	"io"
	"math"
	"runtime/debug"
	"github.com/mlnoga/nightlight/internal/fits"
)


type OpStackSingleBatch struct {
	PreProcess      *OpPreProcess      `json:"preProcess"`
	SelectReference *OpSelectReference `json:"selectReference"`
	PostProcess     *OpPostProcess     `json:"postProcess"`
	Stack           *OpStack           `json:"stack"`
	StarDetect      *OpStarDetect      `json:"starDetect"` 
	Save            *OpSave            `json:"save"`
	MaxThreads       int64             `json:"-"`
}
var _ OperatorJoinFiles = (*OpStackSingleBatch)(nil) // Compile time assertion: type implements the interface


func NewOpStackSingleBatch(opPreProc *OpPreProcess, opSelectReference *OpSelectReference, 
	                       opPostProc *OpPostProcess, opStack *OpStack, opStarDetect *OpStarDetect, 
	                       opSave *OpSave) *OpStackSingleBatch {
	return &OpStackSingleBatch{
		PreProcess:      opPreProc, 
		SelectReference: opSelectReference,
		PostProcess:     opPostProc,
		Stack:           opStack, 
		StarDetect:      opStarDetect,
		Save:            opSave, 
		MaxThreads:      0,
	}
}


// Stack a given batch of files, using the reference provided, or selecting a reference frame if nil.
// Returns the stack for the batch, and updates reference frame internally
func (op *OpStackSingleBatch) Apply(opLoadFiles []*OpLoadFile, logWriter io.Writer) (fOut *fits.Image, err error) {
	// Preprocess light frames (subtract dark, divide flat, remove bad pixels, detect stars and HFR)
	fmt.Fprintf(logWriter, "\nPreprocessing %d frames...\n", len(opLoadFiles))

	opParallelPre:=NewOpParallel(op.PreProcess, op.MaxThreads)
	lights, err:=opParallelPre.ApplyToFiles(opLoadFiles, logWriter)
	if err!=nil { return nil, err }
	lights=RemoveNils(lights) // Remove nils from lights, in case of read errors
	debug.FreeOSMemory()					

	avgNoise:=float32(0)
	for _,l:=range lights {
		avgNoise+=l.Stats.Noise
	}
	avgNoise/=float32(len(lights))
	fmt.Fprintf(logWriter, "Average input frame noise is %.4g\n", avgNoise)

	// Select reference frame, unless one was provided from prior batches
	if op.SelectReference.Frame==nil {
		lights, err=op.SelectReference.ApplyToFITS(lights, logWriter)
		if err!=nil { return nil, err }
		op.PostProcess.Align.Reference    =op.SelectReference.Frame
	    op.PostProcess.Normalize.Reference=op.SelectReference.Frame
	}

	// Post-process all light frames (align, normalize)
	fmt.Fprintf(logWriter, "\nPostprocessing %d frames...\n", len(lights))

	opParallelPost:=NewOpParallel(op.PostProcess, op.MaxThreads)
	lights, err=opParallelPost.ApplyToFITS(lights, logWriter)
	if err!=nil { return nil, err }
	lights=RemoveNils(lights) // Remove nils from lights, in case of alignment errors
	debug.FreeOSMemory()					

	// Tell the stacker the location of the reference frame. Used to fill in NaN pixels when stacking
	op.Stack.RefFrameLoc=0
	if op.PostProcess.Align.Reference!=nil && op.PostProcess.Align.Reference.Stats!=nil {
		op.Stack.RefFrameLoc=op.PostProcess.Align.Reference.Stats.Location
	}
	numFrames:=len(lights)

	// Perform the stack
	fmt.Fprintf(logWriter, "\nStacking %d frames...\n", len(lights))
	fOut, err=op.Stack.Apply(lights, logWriter)
	if err!=nil { return nil, err }

	// Free memory
	lights=nil
	debug.FreeOSMemory()


	// Find stars in the newly stacked batch and report out on them
	fOut, err=op.StarDetect.Apply(fOut, logWriter)
	fmt.Fprintf(logWriter, "Batch %d stack: Stars %d HFR %.2f Exposure %gs %v\n", 0, len(fOut.Stars), fOut.HFR, fOut.Exposure, fOut.Stats)
	// FIXME: Batch ID

	expectedNoise:=avgNoise/float32(math.Sqrt(float64(numFrames)))
	fmt.Fprintf(logWriter, "Batch %d expected noise %.4g from stacking %d frames with average noise %.4g\n",
				0, expectedNoise, numFrames, avgNoise )
	// FIXME: Batch ID

	// Save batch interim results, if desired
	if fOut, err=op.Save.Apply(fOut, logWriter); err!=nil { return nil, err}

	return fOut, nil
}


// Remove nils from an array of fits.Images, editing the underlying array in place
func RemoveNils(lights []*fits.Image) ([]*fits.Image) {
	o:=0
	for i:=0; i<len(lights); i+=1 {
		if lights[i]!=nil {
			lights[o]=lights[i]
			o+=1
		}
	}
	for i:=o; i<len(lights); i++ {
		lights[i]=nil
	}
	return lights[:o]	
}

