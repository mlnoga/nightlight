
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
	"fmt"
	"io"
	colorful "github.com/lucasb-eyer/go-colorful"
)


type OpHSLApplyLum struct {
	Active     bool      `json:"active"`
	Lum 	*FITSImage 		`json:"-"`
}
var _ OperatorUnary = (*OpHSLApplyLum)(nil) // Compile time assertion: type implements the interface

func NewOpHSLApplyLum(active bool) *OpHSLApplyLum {
	return &OpHSLApplyLum{Active: active}
}

// Automatically balance colors with multiple iterations of SetBlackWhitePoints, producing log output
func (op *OpHSLApplyLum) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active || op.Lum==nil { return f, nil }
    fmt.Fprintf(logWriter, "Converting mono luminance image to HSLuv as well...\n")
    op.Lum.MonoToHSLuvLum()

	fmt.Fprintf(logWriter, "Applying luminance image to luminance channel...\n")
	f.ApplyLuminanceToCIExyY(op.Lum)
	return f, nil
}



type OpHSLNeutralizeBackground struct {
	Active     bool      `json:"active"`
	SigmaLow   float32    `json:"sigmaLow"`
	SigmaHigh  float32    `json:"sigmaHigh"`
}
var _ OperatorUnary = (*OpHSLNeutralizeBackground)(nil) // Compile time assertion: type implements the interface

func NewOpHSLNeutralizeBackground(sigmaLow, sigmaHigh float32) *OpHSLNeutralizeBackground {
	return &OpHSLNeutralizeBackground{sigmaLow!=0 || sigmaHigh!=0, sigmaLow, sigmaHigh}
}

// Automatically balance colors with multiple iterations of SetBlackWhitePoints, producing log output
func (op *OpHSLNeutralizeBackground) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	fmt.Fprintf(logWriter, "Neutralizing background values below %.4g sigma, keeping color above %.4g sigma\n", op.SigmaLow, op.SigmaHigh)    	

	_, _, loc, scale, err:=HCLLumMinMaxLocScale(f.Data, f.Naxisn[0])
	if err!=nil { return nil, err }
	low :=loc + scale*float32(op.SigmaLow)
	high:=loc + scale*float32(op.SigmaHigh)
	fmt.Fprintf(logWriter, "Location %.2f%%, scale %.2f%%, low %.2f%% high %.2f%%\n", loc*100, scale*100, low*100, high*100)

	f.NeutralizeBackground(low, high)		
	return f, nil
}




type OpHSLSaturationGamma struct {
	Active  bool      `json:"active"`   
	Gamma   float32   `json:"gamma"`
	Sigma  float32    `json:"sigma"`
}
var _ OperatorUnary = (*OpHSLSaturationGamma)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpHSLSaturationGamma(gamma, sigma float32) *OpHSLSaturationGamma {
	return &OpHSLSaturationGamma{gamma!=1.0, gamma, sigma}
}

func (op *OpHSLSaturationGamma) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
   	fmt.Fprintf(logWriter, "Applying gamma %.2f to saturation for values %.4g sigma above background...\n", op.Gamma, op.Sigma)

	// calculate basic image stats as a fast location and scale estimate
	_, _, loc, scale, err:=HCLLumMinMaxLocScale(f.Data, f.Naxisn[0])
	if err!=nil { return nil, err }
	threshold :=loc + scale*float32(op.Sigma)
	fmt.Fprintf(logWriter, "Location %.2f%%, scale %.2f%%, threshold %.2f%%\n", loc*100, scale*100, threshold*100)

	f.AdjustChroma(op.Gamma, threshold)
	return f, nil
}


type OpHSLSelectiveSaturation struct {
	Active  bool      `json:"active"`   
	From    float32   `json:"from"`
	To      float32   `json:"to"`
	Factor  float32   `json:"factor"`
}
var _ OperatorUnary = (*OpHSLSelectiveSaturation)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpHSLSelectiveSaturation(from, to, factor float32) *OpHSLSelectiveSaturation {
	return &OpHSLSelectiveSaturation{factor!=1, from, to, factor}
}

func (op *OpHSLSelectiveSaturation) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	fmt.Fprintf(logWriter, "Multiplying LCH chroma (saturation) by %.4g for hues in [%g,%g]...\n", op.Factor, op.From, op.To)
	f.AdjustChromaForHues(op.From, op.To, op.Factor)
	return f, nil
}


type OpHSLRotateHue struct {
	Active  bool      `json:"active"`   
	From    float32   `json:"from"`
	To      float32   `json:"to"`
	Offset  float32   `json:"offset"`
	Sigma   float32   `json:"sigma"`
}
var _ OperatorUnary = (*OpHSLRotateHue)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpHSLRotateHue(from, to, offset, sigma float32) *OpHSLRotateHue {
	return &OpHSLRotateHue{offset!=0, from, to, offset, sigma}
}

func (op *OpHSLRotateHue) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	fmt.Fprintf(logWriter, "Rotating LCH hue angles in [%g,%g] by %.4g for lum>=loc+%g*scale...\n", op.From, op.To, op.Offset, op.Sigma)
	_, _, loc, scale, err:=HCLLumMinMaxLocScale(f.Data, f.Naxisn[0])
	if err!=nil { return nil, err }
	threshold :=loc + scale*float32(op.Sigma)
	f.RotateColors(op.From, op.To, op.Offset, threshold)

	return f, nil
}



type OpHSLSCNR struct {
	Active  bool      `json:"active"`   
	Factor  float32   `json:"factor"`
}
var _ OperatorUnary = (*OpHSLSCNR)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpHSLSCNR(factor float32) *OpHSLSCNR {
	return &OpHSLSCNR{factor!=0, factor}
}

func (op *OpHSLSCNR) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	fmt.Fprintf(logWriter, "Applying SCNR of %.4g ...\n", op.Factor)
	f.SCNR(op.Factor)

	return f, nil
}





type OpHSLMidtones struct {
	Active bool    `json:"active"`
	Mid    float32 `json:"mid"`
	Black  float32 `json:"black"`
}

func NewOpHSLMidtones(mid, black float32) *OpHSLMidtones {
	return &OpHSLMidtones{
		Active: mid!=0,
		Mid:    mid,
		Black:  black,
	}
}

func (op *OpHSLMidtones) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	fmt.Fprintf(logWriter, "Applying midtone correction with midtone=%.2f%% x scale and black=location - %.2f%% x scale\n", op.Mid, op.Black)
	// calculate basic image stats as a fast location and scale estimate
	_, _, loc, scale, err:=HCLLumMinMaxLocScale(f.Data, f.Naxisn[0])
	if err!=nil { return nil, err }
	absMid:=op.Mid*scale
	absBlack:=loc - op.Black*scale
	fmt.Fprintf(logWriter, "loc %.2f%% scale %.2f%% absMid %.2f%% absBlack %.2f%%\n", 100*loc, 100*scale, 100*absMid, 100*absBlack)
	f.ApplyMidtonesToChannel(2, absMid, absBlack)
	return f, nil
}




type OpHSLGamma struct {
	Active bool    `json:"active"`
	Gamma  float32 `json:"gamma"`
}

func NewOpHSLGamma(gamma float32) *OpHSLGamma {
	return &OpHSLGamma{gamma!=1.0, gamma}
}

func (op *OpHSLGamma) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	fmt.Fprintf(logWriter, "Applying gamma %.3g\n", op.Gamma)
	f.ApplyGammaToChannel(2, op.Gamma)
	return f, nil
}



type OpHSLPPGamma struct {
	Active bool `json:"active"`
	Gamma  float32 `json:"gamma"`
	Sigma  float32 `json:"sigma"`
}

func NewOpHSLPPGamma(gamma, sigma float32) *OpHSLPPGamma {
	return &OpHSLPPGamma{gamma!=1.0, gamma, sigma}
}

func (op *OpHSLPPGamma) Init() error { return nil }

func (op *OpHSLPPGamma) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	_, _, loc, scale, err:=HCLLumMinMaxLocScale(f.Data, f.Naxisn[0])
	if err!=nil { return nil, err }
	from:=loc+op.Sigma*scale
	to  :=float32(1.0)
	fmt.Fprintf(logWriter, "Based on sigma=%.4g, boosting values in [%.2f%%, %.2f%%] with gamma %.4g...\n", op.Sigma, from*100, to*100, op.Gamma)
	f.ApplyPartialGammaToChannel(2, from, to, op.Gamma)
	return f, nil
}



// must be /100
type OpHSLScaleBlack struct {
	Active bool `json:"active"`
	Black  float32 `json:"value"`
}

func NewOpHSLScaleBlack(black float32) *OpHSLScaleBlack {
	return &OpHSLScaleBlack{black!=0, black}
}

func (op *OpHSLScaleBlack) Init() error { return nil }

func (op *OpHSLScaleBlack) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }

	_, _, loc, scale, err:=HCLLumMinMaxLocScale(f.Data, f.Naxisn[0])
	if err!=nil { return nil, err }
	fmt.Fprintf(logWriter, "Location %.2f%% and scale %.2f%%: ", loc*100, scale*100)
	_,_,hclTargetBlack:=colorful.Xyy(0,0,float64(op.Black)).Hcl()
	targetBlack:=float32(hclTargetBlack)
	if loc>targetBlack {
		fmt.Fprintf(logWriter, "scaling black to move location to HCL %.2f%% for linear %.2f%%...\n", targetBlack*100.0, op.Black)
		f.ShiftBlackToMoveChannel(2,loc, targetBlack)
	} else {
		fmt.Fprintf(logWriter, "cannot move to location %.2f%% by scaling black\n", targetBlack*100.0)
	}
	return f, nil
}


