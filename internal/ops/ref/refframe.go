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


package ref

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/qsort"
	"github.com/mlnoga/nightlight/internal/ops"
	"github.com/mlnoga/nightlight/internal/ops/pre"
)



// Reference frame selection mode
type RefSelMode int
const (
	RFMStarsOverHFR RefSelMode = iota // Pick frame with highest ratio of stars over HFR (for lights)
	RFMMedianLoc                      // Pick frame with median location (for multiplicative correction when integrating master flats)
	RFMFileName                       // Load from given filename
	RFMFileID                         // Load from given frame ID
	RFMLRGB                           // (L)RGB mode. Uses luminance if present (id=3), else the RGB frame with the best stars/HFR ratio
)

type OpSelectReference struct {
	ops.OpBase
	Mode            RefSelMode         `json:"mode"`
	FileName        string             `json:"fileName"`
	FileID          int                `json:"fileID"`
	StarDetect     *pre.OpStarDetect   `json:"starDetect"`
	mutex           sync.Mutex         `json:"-"`
	materialized    []*fits.Image      `json:"-"` 
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpSelectReferenceDefault() })} // register the operator for JSON decoding

func NewOpSelectReferenceDefault() *OpSelectReference { return NewOpSelectReference(RFMStarsOverHFR, "", 0, pre.NewOpStarDetectDefault() )}

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpSelectReference(mode RefSelMode, fileName string, fileID int, opStarDetect *pre.OpStarDetect) *OpSelectReference {
	op:=OpSelectReference{
		OpBase    : ops.OpBase{Type:"selectRef"},
		Mode:       mode,
		FileName:   fileName,
		FileID:     fileID,
		StarDetect: opStarDetect,
	}
	return &op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpSelectReference) UnmarshalJSON(data []byte) error {
	type defaults OpSelectReference
	def:=defaults( *NewOpSelectReferenceDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpSelectReference(def)
	return nil
}

// Selects a reference for all given input promises using the specified mode.
// This creates separate output promises for each input promise.
// The first of them to acquire the reference mutex evaluates all images
func (op *OpSelectReference) MakePromises(ins []ops.Promise, c *ops.Context) (outs []ops.Promise, err error) {
	if len(ins)==0 { return nil, errors.New(fmt.Sprintf("%s operator needs inputs", op.Type)) }

	outs=make([]ops.Promise, len(ins))
	for i,_:=range(ins) {
		outs[i]=op.applySingle(i, ins, c)
	}
	return outs, nil
}


func (op *OpSelectReference) applySingle(i int, ins []ops.Promise, c *ops.Context) ops.Promise {
	return func() (f *fits.Image, err error) {
		op.mutex.Lock()                 // lock so a single thread accesses the reference frame
		if c.RefFrameError!=nil {       // if reference frame detection failed in a prior thread
			op.mutex.Unlock()           // return immediately with the same error
			return nil, errors.New("same error") 
		}
		if c.RefFrame!=nil {            // if a reference frame already exists
			op.mutex.Unlock()           // unlock immediately to allow ...
			if op.materialized==nil || op.materialized[i]==nil {
				return ins[i]()         // ... materializations to be parallelized by the caller
			} else {
				mat:=op.materialized[i]
				op.materialized[i]=nil  // remove reference to free memory
				return mat, nil
			}
		}
		defer op.mutex.Unlock()		    // else release lock later when reference frame is computed

		// if reference image is given in a file, load it and detect stars w/o materializing all input promises
		if op.Mode==RFMFileName {
			if op.FileName=="" { return ins[i]() }

			var promises []ops.Promise
			promises, c.RefFrameError=ops.NewOpLoad(-3, op.FileName).MakePromises(nil, c)
			if c.RefFrameError!=nil { return nil, c.RefFrameError }
			if len(promises)!=1 {
				c.RefFrameError=errors.New("load operator did not create exactly one promise") 
				return nil, c.RefFrameError 
			}
		
			promises, c.RefFrameError=op.StarDetect.MakePromises([]ops.Promise{promises[0]}, c)
			if c.RefFrameError!=nil { return nil, c.RefFrameError }
			if len(promises)!=1 { 
				c.RefFrameError=errors.New("star detect did not return exactly one promise") 
				return nil, c.RefFrameError
			} 
			c.RefFrame, c.RefFrameError=promises[0]()
			if c.RefFrameError!=nil { return nil, c.RefFrameError }

			return ins[i]()
		}

		// otherwise, materialize the input promises
		op.materialized,c.RefFrameError=ops.MaterializeAll(ins, c.MaxThreads, false)
		if c.RefFrameError!=nil { return nil, c.RefFrameError }

		// Auto-select mode for (L)RGB
		mode, fileID :=op.Mode, op.FileID
		if mode == RFMLRGB {
			if len(op.materialized)>3 {
				mode, fileID=RFMFileID, 3
			} else {
				mode=RFMStarsOverHFR
			}
		}

		// select reference with given mode
		var refScore float32
		if mode==RFMStarsOverHFR {
			c.RefFrame, refScore, c.RefFrameError=selectReferenceStarsOverHFR(op.materialized)
		} else if mode==RFMMedianLoc {
			c.RefFrame, refScore, c.RefFrameError=selectReferenceMedianLoc(op.materialized, c)
		} else if mode==RFMFileID {
			if fileID<0 || fileID>=len(op.materialized) {
				c.RefFrameError=errors.New(fmt.Sprintf("invalid reference file ID %d", fileID))
				return nil, c.RefFrameError
			}
			c.RefFrame=op.materialized[fileID]
		} else {
			c.RefFrameError=errors.New(fmt.Sprintf("Unknown refrence selection mode %d", op.Mode))
		}
		if c.RefFrame==nil { c.RefFrameError=errors.New("Unable to select reference image.") }
		if c.RefFrameError!=nil { return nil, c.RefFrameError }
		fmt.Fprintf(c.Log, "Using image %d with score %.4g as reference frame.\n", c.RefFrame.ID, refScore)

		// return promise for the materialized image of this instance
		mat:=op.materialized[i]
		op.materialized[i]=nil // remove reference
		return mat, nil
	}
}

func selectReferenceStarsOverHFR(lights []*fits.Image) (refFrame *fits.Image, refScore float32, err error) {
	refFrame, refScore=nil, -1
	for _, lightP:=range lights {
		if lightP==nil { continue }
		score:=float32(len(lightP.Stars))/lightP.HFR
		if len(lightP.Stars)==0 || lightP.HFR==0 { score=0 }
		if score>refScore {
			refFrame, refScore = lightP, score
		}
	}	
	return refFrame, refScore, nil
}

func selectReferenceMedianLoc(lights []*fits.Image, c *ops.Context) (refFrame *fits.Image, refScore float32, err error) {
	refFrame, refScore=nil, -1
	fun:=func(f *fits.Image) float32 { return f.Stats.Location() }
	locs:=inParallel(lights, fun, c.MaxThreads)
    locs=removeNaNs(locs)
	medianLoc:=qsort.QSelectMedianFloat32(locs)
	for _, lightP:=range lights {
		if lightP==nil { continue }
		if lightP.Stats.Location()==medianLoc {
			return lightP, medianLoc, nil
		}
	}	
	return nil, 0, errors.New("Unable to select reference frame with median location")
}

// Applies the given function to the set of images with given maximal number of threads in parallel.
// Returns the results in order
func inParallel(fs []*fits.Image, fun func(*fits.Image) float32, maxThreads int) []float32 {
	if len(fs)==0 { return nil }
	outs:=make([]float32, len(fs))
	limiter:=make(chan bool, maxThreads)
	for i, f := range(fs) {
		limiter <- true 
		go func(i int, theF *fits.Image) {
			defer func() { <-limiter }()
			if theF==nil { 
				outs[i]=float32(math.NaN())
			} else { 
				outs[i]=fun(theF)
			}
		}(i, f)
	}
	for i:=0; i<cap(limiter); i++ {  // wait for goroutines to finish
		limiter <- true
	}
	return outs
}

// Removes NaNs from the given array, returning a new array.
// This is a stable removal, i.e. order is otherwise preserved
func removeNaNs(as []float32) (res []float32) {
	for _,a:=range(as) {
		if !math.IsNaN(float64(a)) {
			res=append(res, a)
		}
	}
	return res
}