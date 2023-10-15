
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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/ops"
	"github.com/mlnoga/nightlight/internal/ops/pre"
	"github.com/mlnoga/nightlight/internal/ops/ref"
	"github.com/mlnoga/nightlight/internal/ops/hsl"
)


// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpRGBLProcess(opStarDetect *pre.OpStarDetect, opSelectReference *ref.OpSelectReference,
                      opRGBCombine *OpRGBCombine, opRGBBalance *OpRGBBalance,
                      opRGBToHSLuv *OpRGBToHSLuv, opHSLApplyLum *hsl.OpHSLApplyLum,
                      opHSLProcess *ops.OpSequence, opHSLuvToRGB *OpHSLuvToRGB,
                      opSave, opSave2 *ops.OpSave) *ops.OpSequence {
	return ops.NewOpSequence(
		opStarDetect, opSelectReference, opRGBCombine, opRGBBalance, opRGBToHSLuv, opHSLApplyLum, 
		opHSLProcess, opHSLuvToRGB, opSave, opSave2,
	)
}


type OpRGBCombine struct {
	ops.OpBase
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpRGBCombineDefault() })} // register the operator for JSON decoding

func NewOpRGBCombineDefault() *OpRGBCombine { return NewOpRGBCombine() }

func NewOpRGBCombine() *OpRGBCombine {
	return &OpRGBCombine{
		OpBase: ops.OpBase{Type:"rgbCombine"},
	}
}

func (op *OpRGBCombine) MakePromises(ins []ops.Promise, c *ops.Context) (outs []ops.Promise, err error) {
	if len(ins)<3 || len(ins)>4 { return nil, errors.New(fmt.Sprintf("%s operator with %d inputs", op.Type, len(ins))) }
	out:=func() (fOut *fits.Image, err error) {
		fs,err:=ops.MaterializeAll(ins, c.MaxThreads, false)
		if err!=nil { return nil, err }
		return op.Apply(fs, c)
	}
	return []ops.Promise{out}, nil
}

func (op *OpRGBCombine) Apply(fs []*fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if len(fs)<3 || len(fs)>4 {
		return nil, errors.New(fmt.Sprintf("Invalid number of channels for color combination: %d", len(fs)))
	}
	if len(fs)==4 {
		c.LumFrame=fs[3]
	}
	fmt.Fprintf(c.Log, "\nCombining RGB color channels...\n")
	fOut=fits.NewRGBFromChannels(fs[:3], c.AlignStars, c.AlignHFR, c.Log)
	return fOut, nil
}



type OpRGBBalance struct {
	ops.OpUnaryBase
	Block      int32      `json:"block"`
	Border     float32    `json:"border"`
	SkipBright float32    `json:"skipBright"`
	SkipDim    float32    `json:"skipDim"`
	Shadows    fits.RGB   `json:"shadows"`
	Highlights fits.RGB   `json:"highlights"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpRGBBalanceDefault() })} // register the operator for JSON decoding

func NewOpRGBBalanceDefault() *OpRGBBalance { return NewOpRGBBalance(16, 0.1, 0, 0.75, fits.RGB{1,1,1}, fits.RGB{1,1,1} ) }

func NewOpRGBBalance(block int32, border, skipBright, skipDim float32, shadows, highlights fits.RGB) *OpRGBBalance {
	op:=&OpRGBBalance{
		OpUnaryBase : ops.OpUnaryBase{OpBase: ops.OpBase{Type:"rgbBalance"}},
		Block       : block,
		Border      : border,
		SkipBright  : skipBright,
		SkipDim     : skipDim,
		Shadows     : shadows,
		Highlights  : highlights,
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpRGBBalance) UnmarshalJSON(data []byte) error {
	type defaults OpRGBBalance
	def:=defaults( *NewOpRGBBalanceDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpRGBBalance(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

// Automatically balance colors with multiple iterations of SetBlackWhitePoints, producing log output
func (op *OpRGBBalance) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if f.Stars==nil || len(f.Stars)==0 {
		return nil, errors.New("Cannot balance colors with zero stars detected")
	} 

	fmt.Fprintf(c.Log, "Balancing darkest %dx%d block outside %.1f%% border to color tint %s and stars skipping brightest %.1f%% and dimmest %.1f%% to %s\n", 
	            op.Block, op.Block, 100*op.Border, op.Shadows, 100*op.SkipBright, 100*op.SkipDim, op.Highlights)
	err=f.SetBlackWhitePoints(op.Block, op.Border, op.SkipBright, op.SkipDim, op.Shadows, op.Highlights, c.Log)
	return f, err
}



type OpRGBToHSLuv struct {
	ops.OpUnaryBase
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpRGBToHSLuvDefault() })} // register the operator for JSON decoding

func NewOpRGBToHSLuvDefault() *OpRGBToHSLuv { return NewOpRGBToHSLuv() }

func NewOpRGBToHSLuv() *OpRGBToHSLuv {
	op:=&OpRGBToHSLuv{
		OpUnaryBase : ops.OpUnaryBase{OpBase: ops.OpBase{Type:"rgbToHSLuv"}},
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpRGBToHSLuv) UnmarshalJSON(data []byte) error {
	type defaults OpRGBToHSLuv
	def:=defaults( *NewOpRGBToHSLuvDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpRGBToHSLuv(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpRGBToHSLuv) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	fmt.Fprintf(c.Log,"Converting linear RGB to nonlinear HSLuv...\n")
	f.RGBToHSLuv()
	return f, nil
}




type OpHSLuvToRGB struct {
	ops.OpUnaryBase
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLuvToRGBDefault() })} // register the operator for JSON decoding

func NewOpHSLuvToRGBDefault() *OpHSLuvToRGB { return NewOpHSLuvToRGB() }

func NewOpHSLuvToRGB() *OpHSLuvToRGB {
	op:=&OpHSLuvToRGB{
		OpUnaryBase : ops.OpUnaryBase{OpBase: ops.OpBase{Type:"hsluvToRGB"}},
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLuvToRGB) UnmarshalJSON(data []byte) error {
	type defaults OpHSLuvToRGB
	def:=defaults( *NewOpHSLuvToRGBDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpHSLuvToRGB(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLuvToRGB) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	fmt.Fprintf(c.Log, "Converting nonlinear HSLuv to linear RGB\n")
    f.HSLuvToRGB()
	return f, nil
}

