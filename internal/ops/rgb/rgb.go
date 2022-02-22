
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

package rgb

import (
	// "encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/ops"
	"github.com/mlnoga/nightlight/internal/ops/pre"
	"github.com/mlnoga/nightlight/internal/ops/ref"
	"github.com/mlnoga/nightlight/internal/ops/hsl"
)




type OpRGBLProcess struct {
	StarDetect            *pre.OpStarDetect            `json:"starDetect"`
	SelectReference       *ref.OpSelectReference       `json:"selectReference"`
	RGBCombine            *OpRGBCombine            `json:"RGBCombine"`
	RGBBalance            *OpRGBBalance            `json:"RGBBalance"`
    RGBToHSLuv            *OpRGBToHSLuv            `json:"RGBToHSLuv"`
	HSLApplyLum           *hsl.OpHSLApplyLum           `json:"hslApplyLum"`

	HSLProcess            *ops.OpSequence              `json:"HSLProcess"`         
	// OpHSLApplyLuminance
	// OpHSLNeutralizeBackground
	// OpHSLSaturationGamma
	// OpHSLSelectiveSaturation
	// OpHSLRotateHue
	// OpHSLSCNR
	// OpHSLMidtones
	// OpHSLGamma
	// OpHSLPPGamma
	// OpHSLScaleBlack

    HSLuvToRGB            *OpHSLuvToRGB            `json:"HSLuvToRGB"`
	Save                  *ops.OpSave                  `json:"save"`
	Save2                 *ops.OpSave                  `json:"save"`
	parStarDetect         *ops.OpParallel              `json:"-"`
	mutex                 sync.Mutex               `json:"-"`
}
var _ ops.OperatorJoinFiles = (*OpRGBLProcess)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpRGBLProcess(opStarDetect *pre.OpStarDetect, opSelectReference *ref.OpSelectReference,
                      opRGBCombine *OpRGBCombine, opRGBBalance *OpRGBBalance,
                      opRGBToHSLuv *OpRGBToHSLuv, opHSLApplyLum *hsl.OpHSLApplyLum,
                      opHSLProcess *ops.OpSequence, opHSLuvToRGB *OpHSLuvToRGB,
                      opSave, opSave2 *ops.OpSave) *OpRGBLProcess {
	return &OpRGBLProcess{
		StarDetect      : opStarDetect,
		SelectReference : opSelectReference,
		RGBCombine      : opRGBCombine,
 		RGBBalance      : opRGBBalance,
 		RGBToHSLuv      : opRGBToHSLuv,
 		HSLApplyLum     : opHSLApplyLum,
 		HSLProcess      : opHSLProcess,
 		HSLuvToRGB      : opHSLuvToRGB,
		Save            : opSave,
		Save2           : opSave2,
	}	
}

func (op *OpRGBLProcess) init() (err error) { 
	op.mutex.Lock()
	defer op.mutex.Unlock()
	if op.StarDetect!=nil && op.parStarDetect==nil { 
		op.parStarDetect=ops.NewOpParallel(op.StarDetect, 4) // max 4 channels with separate thread
	} 
	return nil
 }

func (op *OpRGBLProcess) Apply(opLoadFiles []*ops.OpLoadFile, logWriter io.Writer) (fOut *fits.Image, err error) {
	if err=op.init(); err!=nil { return nil, err }

	var fs []*fits.Image
	fmt.Fprintf(logWriter, "Reading color channels and detecting stars:\n")
	if op.parStarDetect  !=nil { if fs,   err=op.parStarDetect  .ApplyToFiles(opLoadFiles, logWriter); err!=nil { return nil, err} }

	if l:=len(fs); l<3 || l>4 {
		return nil, errors.New(fmt.Sprintf("Need 3 or 4 channels for RGB(L) combination, have %d",l))
	} else if len(fs)==4 { 
		op.HSLApplyLum.Lum=fs[3] 
		op.SelectReference.Frame=fs[3]
		op.SelectReference.Mode=ref.RFMFrame
		fs=fs[:3]
	} 

	if op.SelectReference!=nil { if fs,   err=op.SelectReference.ApplyToFITS (fs,          logWriter); err!=nil { return nil, err} }
	if op.SelectReference!=nil && op.RGBCombine!=nil { op.RGBCombine.RefFrame=op.SelectReference.Frame }
	if op.RGBCombine     !=nil { if fOut, err=op.RGBCombine     .Apply       (fs,          logWriter); err!=nil { return nil, err} 
    } else {
    	 return nil, errors.New("Combine operator missing")
    }
	if op.RGBBalance     !=nil && op.RGBBalance    .Active { if fOut, err=op.RGBBalance    .Apply(fOut, logWriter); err!=nil { return nil, err } }	
	if op.RGBToHSLuv     !=nil && op.RGBToHSLuv    .Active { if fOut, err=op.RGBToHSLuv    .Apply(fOut, logWriter); err!=nil { return nil, err } }	
	if op.HSLApplyLum    !=nil && op.HSLApplyLum   .Active { if fOut, err=op.HSLApplyLum   .Apply(fOut, logWriter); err!=nil { return nil, err } }	
	if op.HSLProcess     !=nil && op.HSLProcess    .Active { if fOut, err=op.HSLProcess    .Apply(fOut, logWriter); err!=nil { return nil, err } }	
	if op.HSLuvToRGB     !=nil && op.HSLuvToRGB    .Active { if fOut, err=op.HSLuvToRGB    .Apply(fOut, logWriter); err!=nil { return nil, err } }	
	if op.Save           !=nil && op.Save          .Active { if fOut, err=op.Save          .Apply(fOut, logWriter); err!=nil { return nil, err } }
	if op.Save2          !=nil && op.Save2         .Active { if fOut, err=op.Save2         .Apply(fOut, logWriter); err!=nil { return nil, err } }
	return fOut, nil
}



type OpRGBCombine struct {
	Active     bool      `json:"active"`
	RefFrame       *fits.Image          `json:"-"`
}
var _ ops.OperatorJoin = (*OpRGBCombine)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpRGBCombine(active bool) *OpRGBCombine {
	return &OpRGBCombine{Active: active}
}

func (op *OpRGBCombine) Apply(fs []*fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) {
	if !op.Active { return nil, errors.New("RGB combination inactive, unable to produce image") }
	if len(fs)!=3 {
		return nil, errors.New(fmt.Sprintf("Invalid number of channels for color combination: %d", len(fs)))
	}

	fmt.Fprintf(logWriter, "\nCombining RGB color channels...\n")
	fOut=fits.NewRGBFromChannels(fs, op.RefFrame, logWriter)
	return fOut, nil
}



type OpRGBBalance struct {
	Active     bool      `json:"active"`
}
var _ ops.OperatorUnary = (*OpRGBBalance)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpRGBBalance(active bool) *OpRGBBalance {
	return &OpRGBBalance{active}
}

// Automatically balance colors with multiple iterations of SetBlackWhitePoints, producing log output
func (op *OpRGBBalance) Apply(f *fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) {
	if !op.Active { return f, nil }
	if f.Stars==nil || len(f.Stars)==0 {
		return nil, errors.New("Cannot balance colors with zero stars detected")
	} 

	fmt.Fprintf(logWriter, "Setting black point so histogram peaks align and white point so median star color becomes neutral...\n")
	err=f.SetBlackWhitePoints(logWriter)
	return f, err
}


type OpRGBToHSLuv struct {
	Active     bool      `json:"active"`
}
var _ ops.OperatorUnary = (*OpRGBToHSLuv)(nil) // Compile time assertion: type implements the interface

func NewOpRGBToHSLuv(active bool) *OpRGBToHSLuv {
	return &OpRGBToHSLuv{active}
}

func (op *OpRGBToHSLuv) Apply(f *fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) {
	if !op.Active { return f, nil }
	f.RGBToHSLuv()
	return f, nil
}



type OpHSLuvToRGB struct {
	Active     bool      `json:"active"`
}
var _ ops.OperatorUnary = (*OpHSLuvToRGB)(nil) // Compile time assertion: type implements the interface

func NewOpHSLuvToRGB(active bool) *OpHSLuvToRGB {
	return &OpHSLuvToRGB{active}
}

func (op *OpHSLuvToRGB) Apply(f *fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) {
	if !op.Active { return f, nil }
	fmt.Fprintf(logWriter, "Converting nonlinear HSLuv to linear RGB\n")
    f.HSLuvToRGB()
	return f, nil
}

