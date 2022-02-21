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
	"errors"
	"fmt"
	"io"
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/stats"
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
	Active          bool               `json:"active"`
	Mode            RefSelMode         `json:"mode"`
	FileName        string             `json:"fileName"`
	StarDetect     *pre.OpStarDetect   `json:"starDetect"`
	Frame          *fits.Image         `json:"-"`
	Score           float32            `json:"-"`
}
var _ ops.OperatorParallel = (*OpSelectReference)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpSelectReference(mode RefSelMode, fileName string, opStarDetect *pre.OpStarDetect) *OpSelectReference {
	return &OpSelectReference{
		Active:     true, 
		Mode:       mode,
		FileName:   fileName,
		StarDetect: opStarDetect,
	}
}

func (op *OpSelectReference) ApplyToFITS(fs []*fits.Image, logWriter io.Writer) (fsOut []*fits.Image, err error) {
	if !op.Active { return fs, nil }
	switch op.Mode {
	case RFMStarsOverHFR:
		op.Frame, op.Score, err=selectReferenceStarsOverHFR(fs)

	case RFMMedianLoc:
		op.Frame, op.Score, err=selectReferenceMedianLoc(fs)

	case RFMFileName:
		op.Frame, err=ops.LoadAndCalcStats(op.FileName,-3,"align", logWriter)
		if err!=nil { return nil, err }
		op.Frame.Stats, err=stats.CalcExtendedStats(op.Frame.Data, op.Frame.Naxisn[0])
		if err!=nil { return nil, err }
		op.Frame, err=op.StarDetect.Apply(op.Frame, logWriter)
	
	case RFMFrame:
		// do nothing, frame was given 

	default: 
		err=errors.New(fmt.Sprintf("Unknown refrence selection mode %d", op.Mode))
	}
	if op.Frame==nil { err=errors.New("Unable to select reference image.") }
	if err!=nil { return nil, err }
	fmt.Fprintf(logWriter, "Using image %d with score %.4g as reference frame.\n", op.Frame.ID, op.Score)

	return fs, nil
}

func (op *OpSelectReference) ApplyToFiles(opLoadFiles []*ops.OpLoadFile, logWriter io.Writer) (fsOut []*fits.Image, err error) {
	return nil, errors.New("Not implemented: func (op *OpSelectReference) ApplyToFITS()")
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
		locs[num]=lightP.Stats.Location
		num++
	}	
	medianLoc:=qsort.QSelectMedianFloat32(locs[:num])
	for _, lightP:=range lights {
		if lightP==nil { continue }
		if lightP.Stats.Location==medianLoc {
			return lightP, medianLoc, nil
		}
	}	
	return nil, 0, errors.New("Unable to select median reference frame")
}
