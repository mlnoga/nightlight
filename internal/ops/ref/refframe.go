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
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/ops"
	"github.com/mlnoga/nightlight/internal/ops/pre"
	"github.com/mlnoga/nightlight/internal/qsort"
	"math"
	"strconv"
	"sync"
)

// Reference frame selection target
type SelRefTarget int

const (
	SRAlign SelRefTarget = iota // Select star alignment frame
	SRHisto                     // Selct histogram matching frame
)

var srTargetStrings = []string{"alignment", "histogram"}

type OpSelectReference struct {
	ops.OpBase
	Target       SelRefTarget      `json:"target"`
	Mode         string            `json:"mode"`
	StarDetect   *pre.OpStarDetect `json:"starDetect"`
	mutex        sync.Mutex        `json:"-"`
	materialized []*fits.Image     `json:"-"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpSelectReferenceDefault() }) } // register the operator for JSON decoding

func NewOpSelectReferenceDefault() *OpSelectReference {
	return NewOpSelectReference(SRAlign, "%starsHFR", pre.NewOpStarDetectDefault())
}

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpSelectReference(target SelRefTarget, mode string, opStarDetect *pre.OpStarDetect) *OpSelectReference {
	op := OpSelectReference{
		OpBase:     ops.OpBase{Type: "selectRef"},
		Target:     target,
		Mode:       mode,
		StarDetect: opStarDetect,
	}
	return &op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpSelectReference) UnmarshalJSON(data []byte) error {
	type defaults OpSelectReference
	def := defaults(*NewOpSelectReferenceDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpSelectReference(def)
	return nil
}

// Selects a reference for all given input promises using the specified mode.
// This creates separate output promises for each input promise.
// The first of them to acquire the reference mutex evaluates all images
func (op *OpSelectReference) MakePromises(ins []ops.Promise, c *ops.Context) (outs []ops.Promise, err error) {
	if len(ins) == 0 {
		return nil, errors.New(fmt.Sprintf("%s operator needs inputs", op.Type))
	}

	outs = make([]ops.Promise, len(ins))
	for i, _ := range ins {
		outs[i] = op.applySingle(i, ins, c)
	}
	return outs, nil
}

func (op *OpSelectReference) applySingle(i int, ins []ops.Promise, c *ops.Context) ops.Promise {
	return func() (f *fits.Image, err error) {
		op.mutex.Lock()             // lock so a single thread accesses the reference frame
		if c.RefFrameError != nil { // if reference frame detection failed in a prior thread
			op.mutex.Unlock() // return immediately with the same error
			return nil, errors.New("same error")
		}
		if (op.Target == SRAlign && c.AlignStars != nil) ||
			(op.Target == SRHisto && c.MatchHisto != nil) { // if a reference frame already exists
			op.mutex.Unlock() // unlock immediately to allow ...
			if op.materialized == nil || op.materialized[i] == nil {
				return ins[i]() // ... materializations to be parallelized by the caller
			} else {
				mat := op.materialized[i]
				op.materialized[i] = nil // remove reference to free memory
				return mat, nil
			}
		}
		defer op.mutex.Unlock() // else release lock later when reference frame is computed

		mode := op.Mode
		fileID, atoiErr := strconv.Atoi(op.Mode) // reference frame ID, if given

		// if reference image is given in a file, load it and detect stars w/o materializing all input promises
		if mode != "%starsHFR" && mode != "%location" && mode != "%rgb" && atoiErr != nil {
			refFileName := op.Mode
			if refFileName == "" {
				return ins[i]()
			}

			var promises []ops.Promise
			promises, c.RefFrameError = ops.NewOpLoad(-3, refFileName).MakePromises(nil, c)
			if c.RefFrameError != nil {
				return nil, c.RefFrameError
			}
			if len(promises) != 1 {
				c.RefFrameError = errors.New("load operator did not create exactly one promise")
				return nil, c.RefFrameError
			}

			promises, c.RefFrameError = op.StarDetect.MakePromises([]ops.Promise{promises[0]}, c)
			if c.RefFrameError != nil {
				return nil, c.RefFrameError
			}
			if len(promises) != 1 {
				c.RefFrameError = errors.New("star detect did not return exactly one promise")
				return nil, c.RefFrameError
			}
			var refFrame *fits.Image
			refFrame, c.RefFrameError = promises[0]()
			if c.RefFrameError != nil {
				return nil, c.RefFrameError
			}
			op.assignResults(c, refFrame)

			fmt.Fprintf(c.Log, "using loaded image %d as %s reference\n", refFrame.ID, srTargetStrings[op.Target])
			return ins[i]()
		}

		// otherwise, materialize the input promises
		op.materialized, c.RefFrameError = ops.MaterializeAll(ins, c.MaxThreads, false)
		if c.RefFrameError != nil {
			return nil, c.RefFrameError
		}

		// Auto-select mode for (L)RGB
		if mode == "%rgb" {
			if len(op.materialized) > 3 {
				mode, fileID, atoiErr = "3", 3, nil
			} else {
				mode = "%starsHFR"
			}
		}

		// select reference with given mode
		var refScore float32
		var refFrame *fits.Image
		if mode == "%starsHFR" {
			refFrame, refScore, c.RefFrameError = selectReferenceStarsOverHFR(op.materialized)
		} else if mode == "%location" {
			refFrame, refScore, c.RefFrameError = selectReferenceMedianLoc(op.materialized, c)
		} else if atoiErr == nil {
			if fileID < 0 || fileID >= len(op.materialized) {
				c.RefFrameError = errors.New(fmt.Sprintf("invalid reference file ID %d", fileID))
				return nil, c.RefFrameError
			}
			refFrame = op.materialized[fileID]
		} else {
			c.RefFrameError = errors.New(fmt.Sprintf("Unknown refrence selection mode '%s'", op.Mode))
		}
		if refFrame == nil {
			c.RefFrameError = errors.New("Unable to select reference image.")
		}
		if c.RefFrameError != nil {
			return nil, c.RefFrameError
		}
		fmt.Fprintf(c.Log, "Using image %d with score %.4g as %s reference.\n", refFrame.ID, refScore, srTargetStrings[op.Target])
		op.assignResults(c, refFrame)

		// return promise for the materialized image of this instance
		mat := op.materialized[i]
		op.materialized[i] = nil // remove reference
		return mat, nil
	}
}

func (op *OpSelectReference) assignResults(c *ops.Context, refFrame *fits.Image) {
	if op.Target == SRAlign {
		c.AlignNaxisn = refFrame.Naxisn
		c.AlignStars = refFrame.Stars
		c.AlignHFR = refFrame.HFR
	} else if op.Target == SRHisto {
		c.MatchHisto = refFrame.Stats
	} else {
		fmt.Fprintf(c.Log, "Invalid reference selection target %d, skipping.\n", op.Target)
	}
}

func selectReferenceStarsOverHFR(lights []*fits.Image) (refFrame *fits.Image, refScore float32, err error) {
	refFrame, refScore = nil, -1
	for _, lightP := range lights {
		if lightP == nil {
			continue
		}
		score := float32(len(lightP.Stars)) / lightP.HFR
		if len(lightP.Stars) == 0 || lightP.HFR == 0 {
			score = 0
		}
		if score > refScore {
			refFrame, refScore = lightP, score
		}
	}
	return refFrame, refScore, nil
}

func selectReferenceMedianLoc(lights []*fits.Image, c *ops.Context) (refFrame *fits.Image, refScore float32, err error) {
	// calculate locations for all lights
	fun := func(f *fits.Image) float32 { return f.Stats.Location() }
	locs := inParallel(lights, fun, c.MaxThreads)
	locs = removeNaNs(locs)

	// calculate median
	medianLoc := qsort.QSelectMedianFloat32(locs)

	// pick light closest to median
	var closestLight *fits.Image
	closestDistSq := float32(math.MaxFloat32)
	for _, lightP := range lights {
		if lightP == nil {
			continue
		}
		dist := lightP.Stats.Location() - medianLoc
		distSq := dist * dist
		if distSq < closestDistSq {
			closestLight = lightP
			closestDistSq = distSq
		}
	}

	// return closest light if found
	if closestLight != nil {
		return closestLight, medianLoc, nil
	}
	return nil, 0, errors.New("Unable to select reference frame with median location")
}

// Applies the given function to the set of images with given maximal number of threads in parallel.
// Returns the results in order
func inParallel(fs []*fits.Image, fun func(*fits.Image) float32, maxThreads int) []float32 {
	if len(fs) == 0 {
		return nil
	}
	outs := make([]float32, len(fs))
	limiter := make(chan bool, maxThreads)
	for i, f := range fs {
		limiter <- true
		go func(i int, theF *fits.Image) {
			defer func() { <-limiter }()
			if theF == nil {
				outs[i] = float32(math.NaN())
			} else {
				outs[i] = fun(theF)
			}
		}(i, f)
	}
	for i := 0; i < cap(limiter); i++ { // wait for goroutines to finish
		limiter <- true
	}
	return outs
}

// Removes NaNs from the given array, returning a new array.
// This is a stable removal, i.e. order is otherwise preserved
func removeNaNs(as []float32) (res []float32) {
	for _, a := range as {
		if !math.IsNaN(float64(a)) {
			res = append(res, a)
		}
	}
	return res
}
