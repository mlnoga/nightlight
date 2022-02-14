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
	"errors"
	"fmt"
	"io"
	"math"
)



type OpPostProcess struct {
	Normalize   *OpNormalize 	`json:"normalize"`
	Align       *OpAlign 	    `json:"align"`
	Save        *OpSave         `json:"save"`
}
var _ OperatorUnary = (*OpPostProcess)(nil) // Compile time assertion: type implements the interface

func NewOpPostProcess(normalize HistoNormMode, align, alignK int32, alignThreshold float32, 
	                  oobMode OutOfBoundsMode, refSelMode RefSelMode, postProcessedPattern string) *OpPostProcess {
	return &OpPostProcess{
		Normalize : NewOpNormalize(normalize),
		Align : 	NewOpAlign(align, alignK, alignThreshold, oobMode, refSelMode),
		Save :	 	NewOpSave(postProcessedPattern),
	}
}

func (op *OpPostProcess) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if f, err=op.Normalize.Apply(f, logWriter); err!=nil { return nil, err}
	if f, err=op.Align.    Apply(f, logWriter); err!=nil { return nil, err}
	if f, err=op.Save.     Apply(f, logWriter); err!=nil { return nil, err}
	return f, nil
}

func (op *OpPostProcess) Init() (err error) { 
	if err=op.Normalize.Init(); err!=nil { return err }
	if err=op.Align    .Init(); err!=nil { return err }
	if err=op.Save     .Init(); err!=nil { return err }
	return nil
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

type OpNormalize struct {
	Active      bool          `json:"active"`
	Mode        HistoNormMode `json:"mode"`
	Reference  *FITSImage     `json:"-"`}

func NewOpNormalize(mode HistoNormMode) *OpNormalize {
	return &OpNormalize{mode!=HNMNone, mode, nil}
}

func (op *OpNormalize) Init() error { return nil }

func (op *OpNormalize) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error)  {
	if !op.Active || op.Mode==HNMNone { return f, nil }
	switch op.Mode {
		case HNMLocation:
			f.MatchLocation(op.Reference.Stats.Location)
		case HNMLocScale:
			f.MatchHistogram(op.Reference.Stats)
		case HNMLocBlack:
	    	f.ShiftBlackToMove(f.Stats.Location, op.Reference.Stats.Location)
	    	var err error
	    	f.Stats, err=CalcExtendedStats(f.Data, f.Naxisn[0])
	    	if err!=nil { return nil, err }
	}
	fmt.Fprintf(logWriter, "%d: %s\n", f.ID, f.Stats)
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
	Active     bool            `json:"active"`
	K          int32           `json:"k"`
	Threshold  float32         `json:"threshold"`
	OobMode    OutOfBoundsMode `json:"oobMode"`
	RefSelMode RefSelMode      `json:"refSelMode"`
	Reference *FITSImage       `json:"-"`
	HistoRef  *FITSImage       `json:"-"`
	Aligner   *Aligner         `json:"-"`       
}

func NewOpAlign(align, alignK int32, alignThreshold float32, oobMode OutOfBoundsMode, refSelMode RefSelMode) *OpAlign {
	return &OpAlign{
		Active    : align!=0,
		K         : alignK,
		Threshold : alignThreshold,	
		OobMode   : oobMode,
		RefSelMode: refSelMode,
	}
}

func (op *OpAlign) Init() error {
	if !op.Active { return nil }
	if op.Reference==nil || op.Reference.Stars==nil || len(op.Reference.Stars)==0 {
		return errors.New("Unable to align without star detections in reference frame")
	}
	op.Aligner=NewAligner(op.Reference.Naxisn, op.Reference.Stars, op.K)
	return nil
}

func (op *OpAlign) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	// Is alignment to the reference frame required?
	if !op.Active || op.Aligner==nil || op.Aligner.RefStars==nil || len(op.Aligner.RefStars)==0 {
		// Generally not required
		f.Trans=IdentityTransform2D()		
	} else if (len(op.Aligner.RefStars)==len(f.Stars) && (&op.Aligner.RefStars[0]==&f.Stars[0])) {
		// FIXME: comparison is just a heuristic?
		// Not required for reference frame itself
		f.Trans=IdentityTransform2D()		
	} else if f.Stars==nil || len(f.Stars)==0 {
		// No stars - skip alignment and warn
		msg:=fmt.Sprintf("%d: No alignment stars found, skipping frame\n", f.ID)
		return nil, errors.New(msg)
	} else {
		// Alignment is required
		// determine out of bounds fill value
		var outOfBounds float32
		switch(op.OobMode) {
			case OOBModeNaN:         outOfBounds=float32(math.NaN())
			case OOBModeRefLocation: outOfBounds=op.HistoRef.Stats.Location
			case OOBModeOwnLocation: outOfBounds=f          .Stats.Location
		}

		// Determine alignment of the image to the reference frame
		trans, residual := op.Aligner.Align(f.Naxisn, f.Stars, f.ID)
		if residual>op.Threshold {
			msg:=fmt.Sprintf("%d: Alignment residual %g is above threshold %g, skipping frame", f.ID, residual, op.Threshold)
			return nil, errors.New(msg)
		} 
		f.Trans, f.Residual=trans, residual
		fmt.Fprintf(logWriter, "%d: Transform %v; oob %.3g residual %.3g\n", f.ID, f.Trans, outOfBounds, f.Residual)

		// Project image into reference frame
		f, err= f.Project(op.Aligner.Naxisn, trans, outOfBounds)
		if err!=nil { return nil, err }
	}	
	return f, nil
}
