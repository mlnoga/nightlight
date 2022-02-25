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


package post

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/star"
	"github.com/mlnoga/nightlight/internal/ops"
)



func NewOpPostProcess(opMatchHistogram *OpMatchHistogram, opAlign *OpAlign, opSave *ops.OpSave) *ops.OpSequence {
	return ops.NewOpSequence([]ops.Operator{
		opMatchHistogram, opAlign, opSave,
	})
}


// Histogram normalization mode for post-processing
type HistoNormMode int
const (
	HNMNone = iota   // Do not normalize histogram
	HNMLocation      // Multiply with a factor to match histogram peak locations
	HNMLocScale      // Normalize histogram by matching location and scale of the reference frame. Good for stacking lights
	HNMLocBlack      // Normalize histogram to match location of the reference frame by shifting black point. Good for RGB
	HNMAuto          // Auto mode. Uses ScaleLoc for stacking, and LocBlack for (L)RGB combination.
)

type OpMatchHistogram struct {
	ops.OpUnaryBase
	Mode        HistoNormMode `json:"mode"`
}

var _ ops.Operator = (*OpMatchHistogram)(nil) // this type is an Operator

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpMatchHistogramDefault() })} // register the operator for JSON decoding

func NewOpMatchHistogramDefault() *OpMatchHistogram { return NewOpMatchHistogram(HNMLocScale) }

func NewOpMatchHistogram(mode HistoNormMode) *OpMatchHistogram {
	active:=mode!=HNMNone
	op:=OpMatchHistogram{
	  	OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "matchHist", Active: active}},
		Mode        : mode,
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return &op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpMatchHistogram) UnmarshalJSON(data []byte) error {
	type defaults OpMatchHistogram
	def:=defaults( *NewOpMatchHistogramDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpMatchHistogram(def)
	return nil
}

func (op *OpMatchHistogram) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if !op.Active || op.Mode==HNMNone { return f, nil }
	if c.RefFrame==nil { return nil, errors.New("missing historgram reference frame")}
	switch op.Mode {
		case HNMLocation:
			f.MatchLocation(c.RefFrame.Stats.Location())
		case HNMLocScale:
			f.MatchHistogram(c.RefFrame.Stats)
		case HNMLocBlack:
	    	f.ShiftBlackToMove(f.Stats.Location(), c.RefFrame.Stats.Location())
	}
	fmt.Fprintf(c.Log, "%d: %s after normalizing histogram to reference %d\n", f.ID, f.Stats, c.RefFrame.ID)
	return f, nil
}

// Replacement mode for out of bounds values when projecting images
type OutOfBoundsMode int
const (
	OOBModeNaN = iota   // Replace with NaN. Stackers ignore NaNs, so they just take frames into account which have data for the given pixel
	OOBModeRefLocation  // Replace with reference frame location estimate. Good for projecting data for one channel before stacking
	OOBModeOwnLocation  // Replace with location estimate for the current frame. Good for projecting RGB, where locations can differ
)

type OpAlign struct {
	ops.OpUnaryBase
	K          int32           `json:"k"`
	Threshold  float32         `json:"threshold"`
	OobMode    OutOfBoundsMode `json:"oobMode"`
	Aligner   *star.Aligner    `json:"-"`
	mutex     sync.Mutex       `json:"-"`       
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpAlignDefault() })} // register the operator for JSON decoding

func NewOpAlignDefault() *OpAlign { return NewOpAlign(1, 20, 1.0, OOBModeNaN) }

func NewOpAlign(align, alignK int32, alignThreshold float32, oobMode OutOfBoundsMode) *OpAlign {
	active:=alignK>0
	op:=OpAlign{
	  	OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "align", Active: active}},
		K         : alignK,
		Threshold : alignThreshold,	
		OobMode   : oobMode,
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return &op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpAlign) UnmarshalJSON(data []byte) error {
	type defaults OpAlign
	def:=defaults( *NewOpAlignDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpAlign(def)
	return nil
}

func (op *OpAlign) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if err =op.init(c); err!=nil { return nil, err } // initialize the aligner

	// Is alignment to the reference frame required?
	if !op.Active || op.Aligner==nil || len(op.Aligner.RefStars)==0 {
		// Generally not required
		f.Trans=star.IdentityTransform2D()		
	} else if (len(op.Aligner.RefStars)==len(f.Stars) && (&op.Aligner.RefStars[0]==&f.Stars[0])) {
		// Not required for reference frame itself
		f.Trans=star.IdentityTransform2D()		
	} else if len(f.Stars)==0 {
		// No stars - skip alignment and warn
		msg:=fmt.Sprintf("%d: No alignment stars found, skipping frame\n", f.ID)
		return nil, errors.New(msg)
	} else {
		// Alignment is required
		// determine out of bounds fill value
		var outOfBounds float32
		switch(op.OobMode) {
			case OOBModeNaN:         outOfBounds=float32(math.NaN())
			case OOBModeRefLocation: outOfBounds=c.RefFrame.Stats.Location()
			case OOBModeOwnLocation: outOfBounds=f         .Stats.Location()
		}

		// Determine alignment of the image to the reference frame
		trans, residual := op.Aligner.Align(f.Naxisn, f.Stars, f.ID)
		if residual>op.Threshold {
			msg:=fmt.Sprintf("%d: Alignment residual %g is above threshold %g, skipping frame", f.ID, residual, op.Threshold)
			return nil, errors.New(msg)
		} 
		f.Trans, f.Residual=trans, residual
		fmt.Fprintf(c.Log, "%d: Transform %v; residual %.3g oob %.3g\n", f.ID, f.Trans, f.Residual, outOfBounds)

		// Project image into reference frame
		f, err= f.Project(op.Aligner.Naxisn, trans, outOfBounds)
		if err!=nil { return nil, err }
	}	
	return f, nil
}

func (op *OpAlign) init(c *ops.Context) error {
	op.mutex.Lock()
	defer op.mutex.Unlock()
	if !op.Active || op.Aligner!=nil { return nil }

	if c.RefFrame==nil {
		return errors.New("Unable to align without reference frame")
	} else if len(c.RefFrame.Stars)==0 {
		return errors.New("Unable to align without star detections in reference frame")
	}
	op.Aligner=star.NewAligner(c.RefFrame.Naxisn, c.RefFrame.Stars, op.K)
	return nil
}

