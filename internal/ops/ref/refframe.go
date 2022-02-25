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
	RFMFrame                          // Use given frame
)

type OpSelectReference struct {
	ops.OpBase
	Mode            RefSelMode         `json:"mode"`
	FileName        string             `json:"fileName"`
	StarDetect     *pre.OpStarDetect   `json:"starDetect"`
	mutex           sync.Mutex         `json:"-"`
	materialized    []*fits.Image      `json:"-"` 
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpSelectReferenceDefault() })} // register the operator for JSON decoding

func NewOpSelectReferenceDefault() *OpSelectReference { return NewOpSelectReference(RFMStarsOverHFR, "", pre.NewOpStarDetectDefault() )}

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpSelectReference(mode RefSelMode, fileName string, opStarDetect *pre.OpStarDetect) *OpSelectReference {
	op:=OpSelectReference{
		OpBase    : ops.OpBase{Type:"selectRef", Active: true},
		Mode:       mode,
		FileName:   fileName,
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
		if !op.Active { return ins[i]() }

		// acquire lock on reference frame to avoid race conditions
		op.mutex.Lock()
		defer op.mutex.Unlock()
		// if reference frame already set, return materialized input if it exists, else input promise
		if c.RefFrame!=nil { 
			if op.materialized!=nil && op.materialized[i]!=nil {
				mat:=op.materialized[i]
				op.materialized[i]=nil // remove reference
				return mat, nil
			}
			return ins[i]() 
		}

		// if reference image is given in a file, load it and detect stars w/o materializing all input promises
		if op.Mode==RFMFileName {
			promises, err:=ops.NewOpLoad(-3, op.FileName).MakePromises(nil, c)
			if err!=nil { return nil, err }
			if len(promises)!=1 { return nil, errors.New("load operator did not create exactly one promise")}
		
			promises, err=op.StarDetect.MakePromises([]ops.Promise{promises[0]}, c)
			if err!=nil { return nil, err }
			if len(promises)!=1 { return nil, errors.New("star detect did not return exactly one promise") } 
			c.RefFrame, err=promises[0]()
			if err!=nil { return nil, err }

			return ins[i]()
		}

		// otherwise, materialize the input promises
		op.materialized,err=ops.MaterializeAll(ins, c.MaxThreads, false)
		if err!=nil { return nil, err }

		// select reference with given mode
		var refScore float32
		if op.Mode==RFMStarsOverHFR {
			c.RefFrame, refScore, err=selectReferenceStarsOverHFR(op.materialized)
		} else if op.Mode==RFMMedianLoc {
			c.RefFrame, refScore, err=selectReferenceMedianLoc(op.materialized)
		} else {
			err=errors.New(fmt.Sprintf("Unknown refrence selection mode %d", op.Mode))
		}
		if c.RefFrame==nil { err=errors.New("Unable to select reference image.") }
		if err!=nil { return nil, err }
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

func selectReferenceMedianLoc(lights []*fits.Image) (refFrame *fits.Image, refScore float32, err error) {
	refFrame, refScore=nil, -1
	locs:=make([]float32, len(lights))
	num:=0
	for _, lightP:=range lights {
		if lightP==nil { continue }
		locs[num]=lightP.Stats.Location()
		num++
	}	
	medianLoc:=qsort.QSelectMedianFloat32(locs[:num])
	for _, lightP:=range lights {
		if lightP==nil { continue }
		if lightP.Stats.Location()==medianLoc {
			return lightP, medianLoc, nil
		}
	}	
	return nil, 0, errors.New("Unable to select median reference frame")
}
