
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
	// "encoding/json"
	"errors"
	"fmt"
	"io"
)




type OpRGBLProcess struct {
	StarDetect            *OpStarDetect            `json:"starDetect"`
	SelectReference       *OpSelectReference       `json:"selectReference"`
	RGBCombine            *OpRGBCombine            `json:"RGBCombine"`
	RGBBalance            *OpRGBBalance            `json:"RGBBalance"`
    RGBToHSLuv            *OpRGBToHSLuv            `json:"RGBToHSLuv"`
	HSLApplyLum           *OpHSLApplyLum           `json:"hslApplyLum"`

	HSLProcess            *OpSequence              `json:"HSLProcess"`         
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
	Save                  *OpSave                  `json:"save"`
	Save2                 *OpSave                  `json:"save"`
	parStarDetect         *OpParallel              `json:"-"`
}
var _ OperatorJoinFiles = (*OpRGBLProcess)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpRGBLProcess(opStarDetect *OpStarDetect, opSelectReference *OpSelectReference,
                      opRGBCombine *OpRGBCombine, opRGBBalance *OpRGBBalance,
                      opRGBToHSLuv *OpRGBToHSLuv, opHSLApplyLum *OpHSLApplyLum,
                      opHSLProcess *OpSequence, opHSLuvToRGB *OpHSLuvToRGB,
                      opSave, opSave2 *OpSave) *OpRGBLProcess {
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

func (op *OpRGBLProcess) Init() (err error) { 
	if op.StarDetect     !=nil { if err=op.StarDetect     .Init(); err!=nil { return err } 
		op.parStarDetect=NewOpParallel(op.StarDetect, 4) // max 4 channels with separate thread
	} 
	if op.SelectReference!=nil { if err=op.SelectReference.Init(); err!=nil { return err } }
	if op.RGBCombine     !=nil { if err=op.RGBCombine     .Init(); err!=nil { return err } }
	if op.RGBBalance     !=nil { if err=op.RGBBalance     .Init(); err!=nil { return err } }	
	if op.RGBToHSLuv     !=nil { if err=op.RGBToHSLuv     .Init(); err!=nil { return err } }	
	if op.HSLApplyLum    !=nil { if err=op.HSLApplyLum    .Init(); err!=nil { return err } }	
	if op.HSLProcess     !=nil { if err=op.HSLProcess     .Init(); err!=nil { return err } }	
	if op.HSLuvToRGB     !=nil { if err=op.HSLuvToRGB     .Init(); err!=nil { return err } }	
	if op.Save           !=nil { if err=op.Save           .Init(); err!=nil { return err } }
	if op.Save2          !=nil { if err=op.Save2          .Init(); err!=nil { return err } }
	return nil
 }

func (op *OpRGBLProcess) Apply(opLoadFiles []*OpLoadFile, logWriter io.Writer) (fOut *FITSImage, err error) {
	var fs []*FITSImage
	if l:=len(opLoadFiles); l<3 || l>4 {
		return nil, errors.New(fmt.Sprintf("Need 3 or 4 channels for RGB(L) combination, have %d",l))
	} else if len(fs)==4 { 
		op.HSLApplyLum.Lum=fs[3] 
	} 

	fmt.Fprintf(logWriter, "Reading color channels and detecting stars:\n")
	if op.parStarDetect  !=nil { if fs,   err=op.parStarDetect  .ApplyToFiles(opLoadFiles, logWriter); err!=nil { return nil, err} }
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



type OpSelectReference struct {
	Active          bool               `json:"active"`
	Mode            RefSelMode         `json:"mode"`
	Frame          *FITSImage          `json:"-"`
	Score           float32            `json:"-"`
}
var _ OperatorParallel = (*OpSelectReference)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpSelectReference(mode RefSelMode) *OpSelectReference {
	return &OpSelectReference{Mode: mode}
}

func (op *OpSelectReference) Init() (err error) { return nil }

func (op *OpSelectReference) ApplyToFITS(fs []*FITSImage, logWriter io.Writer) (fsOut []*FITSImage, err error) {
	if !op.Active { return fs, nil }
	if len(fs)==4 {
		op.Frame=fs[3]
		fmt.Fprintf(logWriter, "Using luminance channel %d as reference frame\n", op.Frame.ID)
	} else {
		op.Frame, op.Score=SelectReferenceFrame(fs, op.Mode)
		if op.Frame==nil { panic("Reference channel for alignment not found.") }
		fmt.Fprintf(logWriter, "Using channel %d with score %.4g as reference for alignment and normalization.\n\n", op.Frame.ID, op.Score)
	}
	return fs, nil
}

func (op *OpSelectReference) ApplyToFiles(opLoadFiles []*OpLoadFile, logWriter io.Writer) (fsOut []*FITSImage, err error) {
	return nil, errors.New("Not implemented: func (op *OpSelectReference) ApplyToFITS()")
}



type OpRGBCombine struct {
	Active     bool      `json:"active"`
	RefFrame       *FITSImage          `json:"-"`
}
var _ OperatorJoin = (*OpRGBCombine)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpRGBCombine(active bool) *OpRGBCombine {
	return &OpRGBCombine{Active: active}
}

func (op *OpRGBCombine) Init() (err error) { return nil }

func (op *OpRGBCombine) Apply(fs []*FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return nil, errors.New("RGB combination inactive, unable to produce image") }
	if len(fs)<3 || len(fs)>4 {
		return nil, errors.New(fmt.Sprintf("Invalid number of channels for color combination: %d", len(fs)))
	}
	fmt.Fprintf(logWriter, "\nCombining RGB color channels...\n")
	fOut2:=CombineRGB(fs, op.RefFrame)
	return &fOut2, nil
}



type OpRGBBalance struct {
	Active     bool      `json:"active"`
}
var _ OperatorUnary = (*OpRGBBalance)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpRGBBalance(active bool) *OpRGBBalance {
	return &OpRGBBalance{active}
}

func (op *OpRGBBalance) Init() (err error) { return nil }

// Automatically balance colors with multiple iterations of SetBlackWhitePoints, producing log output
func (op *OpRGBBalance) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	if f.Stars==nil || len(f.Stars)==0 {
		return nil, errors.New("Cannot balance colors with zero stars detected")
	} 

	fmt.Fprintf(logWriter, "Setting black point so histogram peaks align and white point so median star color becomes neutral...")
	err=f.SetBlackWhitePoints()
	return f, err
}


type OpRGBToHSLuv struct {
	Active     bool      `json:"active"`
}
var _ OperatorUnary = (*OpRGBToHSLuv)(nil) // Compile time assertion: type implements the interface

func NewOpRGBToHSLuv(active bool) *OpRGBToHSLuv {
	return &OpRGBToHSLuv{active}
}

func (op *OpRGBToHSLuv) Init() (err error) { return nil }

func (op *OpRGBToHSLuv) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	f.RGBToHSLuv()
	return f, nil
}



type OpHSLuvToRGB struct {
	Active     bool      `json:"active"`
}
var _ OperatorUnary = (*OpHSLuvToRGB)(nil) // Compile time assertion: type implements the interface

func NewOpHSLuvToRGB(active bool) *OpHSLuvToRGB {
	return &OpHSLuvToRGB{active}
}

func (op *OpHSLuvToRGB) Init() (err error) { return nil }

func (op *OpHSLuvToRGB) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	LogPrintln("Converting nonlinear HSLuv to linear RGB")
    f.HSLuvToRGB()
	return f, nil
}

