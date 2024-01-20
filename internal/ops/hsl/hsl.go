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

package hsl

import (
	"encoding/json"
	"fmt"
	"math"
	
	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/ops"
	"github.com/mlnoga/nightlight/internal/ops/stretch"
	"github.com/mlnoga/nightlight/internal/stats"
)

type OpHSLApplyLum struct {
	ops.OpUnaryBase
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLApplyLumDefault() }) } // register the operator for JSON decoding

func NewOpHSLApplyLumDefault() *OpHSLApplyLum { return NewOpHSLApplyLum() }

func NewOpHSLApplyLum() *OpHSLApplyLum {
	op := &OpHSLApplyLum{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "hslApplyLum"}},
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLApplyLum) UnmarshalJSON(data []byte) error {
	type defaults OpHSLApplyLum
	def := defaults(*NewOpHSLApplyLumDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpHSLApplyLum(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLApplyLum) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if c.LumFrame == nil {
		return f, nil
	}
	fmt.Fprintf(c.Log, "Converting mono luminance image to HSLuv as well...\n")
	c.LumFrame.MonoToHSLuvLum()

	fmt.Fprintf(c.Log, "Applying luminance image to luminance channel...\n")
	f.ApplyLuminanceToCIExyY(c.LumFrame)

	c.LumFrame = nil // free memory
	return f, nil
}

type OpHSLScaleOffsetChannel struct {
	ops.OpUnaryBase
	ChannelID int     `json:"channelID"`
	Scale     float32 `json:"scale"`
	Offset    float32 `json:"offset"`
}

func init() {
	ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLScaleOffsetChannelDefault() })
} // register the operator for JSON decoding

func NewOpHSLScaleOffsetChannelDefault() *OpHSLScaleOffsetChannel {
	return NewOpHSLScaleOffsetChannel(2, 1, 0)
}

func NewOpHSLScaleOffsetChannel(channelID int, scale, offset float32) *OpHSLScaleOffsetChannel {
	op := &OpHSLScaleOffsetChannel{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "hslScaleOffsetChannel"}},
		ChannelID:   channelID,
		Scale:       scale,
		Offset:      offset,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLScaleOffsetChannel) UnmarshalJSON(data []byte) error {
	type defaults OpHSLScaleOffsetChannel
	def := defaults(*NewOpHSLScaleOffsetChannelDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpHSLScaleOffsetChannel(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLScaleOffsetChannel) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if op.Scale == 1 && op.Offset == 0 {
		return f, nil
	}
	fmt.Fprintf(c.Log, "%d: Applying pixel math x = x * %.3f + %.3f%% to channel %d\n", f.ID, op.Scale, op.Offset*100, op.ChannelID)
	f.ApplyScaleOffsetToChannel(op.ChannelID, op.Scale, op.Offset)
	return f, nil
}

type OpHSLNeutralizeBackground struct {
	ops.OpUnaryBase
	SigmaLow  float32 `json:"sigmaLow"`
	SigmaHigh float32 `json:"sigmaHigh"`
}

func init() {
	ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLNeutralizeBackgroundDefault() })
} // register the operator for JSON decoding

func NewOpHSLNeutralizeBackgroundDefault() *OpHSLNeutralizeBackground {
	return NewOpHSLNeutralizeBackground(0.75, 1.0)
}

func NewOpHSLNeutralizeBackground(sigmaLow, sigmaHigh float32) *OpHSLNeutralizeBackground {
	op := &OpHSLNeutralizeBackground{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "hslNeutralizeBackground"}},
		SigmaLow:    sigmaLow,
		SigmaHigh:   sigmaHigh,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLNeutralizeBackground) UnmarshalJSON(data []byte) error {
	type defaults OpHSLNeutralizeBackground
	def := defaults(*NewOpHSLNeutralizeBackgroundDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpHSLNeutralizeBackground(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLNeutralizeBackground) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if op.SigmaLow <= 0 && op.SigmaHigh <= 0 {
		return f, nil
	}
	fmt.Fprintf(c.Log, "Neutralizing background values below %.4g sigma, keeping color above %.4g sigma\n", op.SigmaLow, op.SigmaHigh)

	st := stats.NewStatsForChannel(f.Data, f.Naxisn[0], 2, 3)
	loc, scale := st.Location(), st.Scale()
	low := loc + scale*float32(op.SigmaLow)
	high := loc + scale*float32(op.SigmaHigh)
	fmt.Fprintf(c.Log, "Location %.2f%%, scale %.2f%%, low %.2f%% high %.2f%%\n", loc*100, scale*100, low*100, high*100)

	f.NeutralizeBackground(low, high)
	return f, nil
}

type OpHSLSaturationGamma struct {
	ops.OpUnaryBase
	Gamma float32 `json:"gamma"`
	Sigma float32 `json:"sigma"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLSaturationGammaDefault() }) } // register the operator for JSON decoding

func NewOpHSLSaturationGammaDefault() *OpHSLSaturationGamma {
	return NewOpHSLSaturationGamma(1.75, 0.75)
}

func NewOpHSLSaturationGamma(gamma, sigma float32) *OpHSLSaturationGamma {
	op := &OpHSLSaturationGamma{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "hslSaturationGamma"}},
		Gamma:       gamma,
		Sigma:       sigma,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLSaturationGamma) UnmarshalJSON(data []byte) error {
	type defaults OpHSLSaturationGamma
	def := defaults(*NewOpHSLSaturationGammaDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpHSLSaturationGamma(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLSaturationGamma) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if op.Gamma == 1.0 {
		return f, nil
	}
	fmt.Fprintf(c.Log, "Applying gamma %.2f to saturation for values %.4g sigma above background...\n", op.Gamma, op.Sigma)

	st := stats.NewStatsForChannel(f.Data, f.Naxisn[0], 2, 3)
	loc, scale := st.Location(), st.Scale()
	threshold := loc + scale*float32(op.Sigma)
	fmt.Fprintf(c.Log, "Location %.2f%%, scale %.2f%%, threshold %.2f%%\n", loc*100, scale*100, threshold*100)

	f.AdjustChroma(op.Gamma, threshold)
	return f, nil
}

type OpHSLSelectiveSaturation struct {
	ops.OpUnaryBase
	From   float32 `json:"from"`
	To     float32 `json:"to"`
	Factor float32 `json:"factor"`
}

func init() {
	ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLSelectiveSaturationDefault() })
} // register the operator for JSON decoding

func NewOpHSLSelectiveSaturationDefault() *OpHSLSelectiveSaturation {
	return NewOpHSLSelectiveSaturation(295, 40, 1)
}

func NewOpHSLSelectiveSaturation(from, to, factor float32) *OpHSLSelectiveSaturation {
	op := &OpHSLSelectiveSaturation{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "hslSelectiveSaturation"}},
		From:        from,
		To:          to,
		Factor:      factor,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLSelectiveSaturation) UnmarshalJSON(data []byte) error {
	type defaults OpHSLSelectiveSaturation
	def := defaults(*NewOpHSLSelectiveSaturationDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpHSLSelectiveSaturation(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLSelectiveSaturation) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if op.Factor == 1 {
		return f, nil
	}
	fmt.Fprintf(c.Log, "Multiplying LCH chroma (saturation) by %.4g for hues in [%g,%g]...\n", op.Factor, op.From, op.To)
	f.AdjustChromaForHues(op.From, op.To, op.Factor)
	return f, nil
}

type OpHSLRotateHue struct {
	ops.OpUnaryBase
	From   float32 `json:"from"`
	To     float32 `json:"to"`
	Offset float32 `json:"offset"`
	Sigma  float32 `json:"sigma"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLRotateHueDefault() }) } // register the operator for JSON decoding

func NewOpHSLRotateHueDefault() *OpHSLRotateHue { return NewOpHSLRotateHue(100, 190, 0, 1) }

func NewOpHSLRotateHue(from, to, offset, sigma float32) *OpHSLRotateHue {
	op := &OpHSLRotateHue{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "hslRotateHue"}},
		From:        from,
		To:          to,
		Offset:      offset,
		Sigma:       sigma,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLRotateHue) UnmarshalJSON(data []byte) error {
	type defaults OpHSLRotateHue
	def := defaults(*NewOpHSLRotateHueDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpHSLRotateHue(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLRotateHue) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if op.Offset == 0 {
		return f, nil
	}
	fmt.Fprintf(c.Log, "Rotating LCH hue angles in [%g,%g] by %.4g for lum>=loc+%g*scale...\n", op.From, op.To, op.Offset, op.Sigma)

	st := stats.NewStatsForChannel(f.Data, f.Naxisn[0], 2, 3)
	loc, scale := st.Location(), st.Scale()
	threshold := loc + scale*float32(op.Sigma)

	f.RotateColors(op.From, op.To, op.Offset, threshold)
	return f, nil
}

type OpHSLSCNR struct {
	ops.OpUnaryBase
	Factor float32 `json:"factor"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLSCNRDefault() }) } // register the operator for JSON decoding

func NewOpHSLSCNRDefault() *OpHSLSCNR { return NewOpHSLSCNR(0) }

func NewOpHSLSCNR(factor float32) *OpHSLSCNR {
	op := &OpHSLSCNR{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "hslSCNR"}},
		Factor:      factor,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLSCNR) UnmarshalJSON(data []byte) error {
	type defaults OpHSLSCNR
	def := defaults(*NewOpHSLSCNRDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpHSLSCNR(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLSCNR) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if op.Factor == 0 {
		return f, nil
	}
	fmt.Fprintf(c.Log, "Applying SCNR of %.4g ...\n", op.Factor)
	f.SCNR(op.Factor)

	return f, nil
}

type OpHSLMidtones struct {
	ops.OpUnaryBase
	Mid   float32 `json:"mid"`
	Black float32 `json:"black"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLMidtonesDefault() }) } // register the operator for JSON decoding

func NewOpHSLMidtonesDefault() *OpHSLMidtones { return NewOpHSLMidtones(0, 2) }

func NewOpHSLMidtones(mid, black float32) *OpHSLMidtones {
	op := &OpHSLMidtones{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "hslMidtones"}},
		Mid:         mid,
		Black:       black,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLMidtones) UnmarshalJSON(data []byte) error {
	type defaults OpHSLMidtones
	def := defaults(*NewOpHSLMidtonesDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpHSLMidtones(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLMidtones) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if op.Mid == 0 {
		return f, nil
	}
	fmt.Fprintf(c.Log, "Applying midtone correction with midtone=%.2f%% x scale and black=location - %.2f%% x scale\n", op.Mid, op.Black)

	st := stats.NewStatsForChannel(f.Data, f.Naxisn[0], 2, 3)
	loc, scale := st.Location(), st.Scale()
	absMid := op.Mid * scale
	absBlack := loc - op.Black*scale

	fmt.Fprintf(c.Log, "loc %.2f%% scale %.2f%% absMid %.2f%% absBlack %.2f%%\n", 100*loc, 100*scale, 100*absMid, 100*absBlack)
	f.ApplyMidtonesToChannel(2, absMid, absBlack)
	return f, nil
}

type OpHSLGamma struct {
	ops.OpUnaryBase
	Gamma float32 `json:"gamma"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLGammaDefault() }) } // register the operator for JSON decoding

func NewOpHSLGammaDefault() *OpHSLGamma { return NewOpHSLGamma(1.0) }

func NewOpHSLGamma(gamma float32) *OpHSLGamma {
	op := &OpHSLGamma{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "hslGamma"}},
		Gamma:       gamma,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLGamma) UnmarshalJSON(data []byte) error {
	type defaults OpHSLGamma
	def := defaults(*NewOpHSLGammaDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpHSLGamma(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLGamma) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if op.Gamma == 1.0 {
		return f, nil
	}
	fmt.Fprintf(c.Log, "Applying gamma %.3g\n", op.Gamma)
	f.ApplyGammaToChannel(2, op.Gamma)
	return f, nil
}

type OpHSLGammaPP struct {
	ops.OpUnaryBase
	Gamma float32 `json:"gamma"`
	Sigma float32 `json:"sigma"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLGammaPPDefault() }) } // register the operator for JSON decoding

func NewOpHSLGammaPPDefault() *OpHSLGammaPP { return NewOpHSLGammaPP(1.0, 1.0) }

func NewOpHSLGammaPP(gamma, sigma float32) *OpHSLGammaPP {
	op := &OpHSLGammaPP{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "hslGammaPP"}},
		Gamma:       gamma,
		Sigma:       sigma,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLGammaPP) UnmarshalJSON(data []byte) error {
	type defaults OpHSLGammaPP
	def := defaults(*NewOpHSLGammaPPDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpHSLGammaPP(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLGammaPP) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if op.Gamma == 1.0 {
		return f, nil
	}

	st := stats.NewStatsForChannel(f.Data, f.Naxisn[0], 2, 3)
	loc, scale := st.Location(), st.Scale()
	from := loc + op.Sigma*scale
	to := float32(1.0)

	fmt.Fprintf(c.Log, "Based on sigma=%.4g, boosting values in [%.2f%%, %.2f%%] with gamma %.4g...\n", op.Sigma, from*100, to*100, op.Gamma)
	f.ApplyPartialGammaToChannel(2, from, to, op.Gamma)
	return f, nil
}

type OpHSLUnsharpMask struct {
	ops.OpUnaryBase
	Sigma     float32 `json:"sigma"`
	Gain      float32 `json:"gain"`
	Threshold float32 `json:"threshold"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLUnsharpMaskDefault() }) } // register the operator for JSON decoding

func NewOpHSLUnsharpMaskDefault() *OpHSLUnsharpMask {
	return NewOpHSLUnsharpMask(1.5, 0.0, 0.75)
}

func NewOpHSLUnsharpMask(sigma, gain, threshold float32) *OpHSLUnsharpMask {
	op := &OpHSLUnsharpMask{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "HSLUnsharpMask"}},
		Sigma:       sigma,
		Gain:        gain,
		Threshold:   threshold,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLUnsharpMask) UnmarshalJSON(data []byte) error {
	type defaults OpHSLUnsharpMask
	def := defaults(*NewOpHSLUnsharpMaskDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpHSLUnsharpMask(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLUnsharpMask) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if op.Gain == 0.0 {
		return f, nil
	}

	st := stats.NewStatsForChannel(f.Data, f.Naxisn[0], 2, 3)
	absThresh := st.Location() + st.Scale()*op.Threshold
	fmt.Fprintf(c.Log, "%d: Unsharp masking with sigma %.3g gain %.3g thresh %.3g absThresh %.3g\n",
		f.ID, op.Sigma, op.Gain, op.Threshold, absThresh)
	kernel := stretch.GaussianKernel1D(op.Sigma)
	fmt.Fprintf(c.Log, "%d: Unsharp masking kernel sigma %.2f size %d: %v\n",
		f.ID, op.Sigma, len(kernel), kernel)
	// apply to luminance (channel 2) only
	sharpened := stretch.UnsharpMask(f.Data[2*f.Pixels/3:], int(f.Naxisn[0]), op.Sigma, op.Gain, st.Min(), st.Max(), absThresh)
	for i := range sharpened {
		f.Data[2*f.Pixels/3+int32(i)] = sharpened[i]
	}
	return f, nil
}

// must be /100
type OpHSLScaleBlack struct {
	ops.OpUnaryBase
	Location float32 `json:"location"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLScaleBlackDefault() }) } // register the operator for JSON decoding

func NewOpHSLScaleBlackDefault() *OpHSLScaleBlack { return NewOpHSLScaleBlack(0) }

func NewOpHSLScaleBlack(location float32) *OpHSLScaleBlack {
	op := &OpHSLScaleBlack{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "hslScaleBlack"}},
		Location:    location,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLScaleBlack) UnmarshalJSON(data []byte) error {
	type defaults OpHSLScaleBlack
	def := defaults(*NewOpHSLScaleBlackDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpHSLScaleBlack(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLScaleBlack) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if op.Location == 0 {
		return f, nil
	}

	st := stats.NewStatsForChannel(f.Data, f.Naxisn[0], 2, 3)
	loc, scale := st.Location(), st.Scale()
	fmt.Fprintf(c.Log, "Location %.2f%% and scale %.2f%%: ", loc*100, scale*100)
	//	_,_,hclTargetBlack:=colorful.Xyy(0,0,float64(op.Location)).Hcl()
	//	targetBlack:=float32(hclTargetBlack)
	_, _, hslUVTargetBlack := colorful.LinearRgb(float64(op.Location), float64(op.Location), float64(op.Location)).HSLuv()
	targetBlack := float32(hslUVTargetBlack)

	if loc > targetBlack {
		fmt.Fprintf(c.Log, "scaling black to move location to HSLuv %.2f%% for linear %.2f%%...\n", targetBlack*100.0, op.Location*100.0)
		f.ShiftBlackToMoveChannel(2, loc, targetBlack)
	} else {
		fmt.Fprintf(c.Log, "cannot move to location %.2f%% by scaling black\n", targetBlack*100.0)
	}
	return f, nil
}


type OpHSLStretchIterative struct {
	ops.OpUnaryBase
	Location    float32   `json:"location"`
	Scale       float32   `json:"scale"`
}

var _ ops.Operator = (*OpHSLStretchIterative)(nil) // this type is an Operator
func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpHSLStretchIterativeDefault() })} // register the operator for JSON decoding

func NewOpHSLStretchIterativeDefault() *OpHSLStretchIterative { return NewOpHSLStretchIterative(0.1, 0.004) }

// must be called /100
func NewOpHSLStretchIterative(loc float32, scale float32) (*OpHSLStretchIterative) {
	op:=&OpHSLStretchIterative{ 
	  	OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "hslStretch"}},
		Location    : loc, 
		Scale       : scale,
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpHSLStretchIterative) UnmarshalJSON(data []byte) error {
	type defaults OpHSLStretchIterative
	def:=defaults( *NewOpHSLStretchIterativeDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpHSLStretchIterative(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpHSLStretchIterative) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.Location==0 && op.Scale==0 { return f, nil }
	fmt.Fprintf(c.Log, "%d: Auto-stretching HSL loc to %.2f%% and scale to %.2f%% ...\n", f.ID, op.Location*100, op.Scale*100)

	for i:=0; ; i++ {
		if i==50 { 
			fmt.Fprintf(c.Log, "%d: Warning: did not converge after %d iterations\n", f.ID, i)
			break
		}

		st := stats.NewStatsForChannel(f.Data, f.Naxisn[0], 2, 3)
		loc, scale := st.Location(), st.Scale()

		fmt.Fprintf(c.Log, "%d: Linear location %.2f%% and scale %.2f%%, ", f.ID, loc*100, scale*100)

		if loc<=op.Location*1.01 && scale<op.Scale {
			idealGamma:=float32(1)
			idealGammaDelta:=float32(math.Abs(float64(op.Scale)-float64(scale)))

			maxGamma:=float32(5.0)
			for gamma:=float32(1.0); gamma<=maxGamma; gamma+=0.01 {
				exponent:=1.0/float64(gamma)
				newLocLower:=float32(math.Pow(float64(loc-scale), exponent))
				newLoc     :=float32(math.Pow(float64(loc        ), exponent))
				newLocUpper:=float32(math.Pow(float64(loc+scale), exponent))

				black:=(op.Location-newLoc)/(op.Location-1)
    			scale:=1/(1-black)

				scaledNewLocLower:=float32(math.Max(0, float64((newLocLower - black) * scale)))
				scaledNewLocUpper:=float32(math.Max(0, float64((newLocUpper - black) * scale)))

				newScale:=float32(scaledNewLocUpper-scaledNewLocLower)/2
				delta:=float32(math.Abs(float64(op.Scale)-float64(newScale)))
				if delta<idealGammaDelta {
					idealGamma=gamma
					idealGammaDelta=delta
				}
			}

			if idealGamma<=1.01 { 
				fmt.Fprintf(c.Log, "done\n")
				break
			}

			fmt.Fprintf(c.Log, "applying gamma %.3g\n", idealGamma)
			f.ApplyGammaToChannel(2, idealGamma)
			//f.ApplyPartialGamma(0, 0.95, idealGamma)
		} else if loc>op.Location*0.99 && scale<op.Scale {
			fmt.Fprintf(c.Log, "scaling black to move location to %.2f%%...\n", op.Location*100)
			f.ShiftBlackToMoveChannel(2, loc, op.Location)
		} else {
			fmt.Fprintf(c.Log, "done\n")
			break
		}
	}
	return f, nil
}
