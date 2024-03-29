// Copyright (C) 2021 Markus L. Noga
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


package stretch

import (
	"encoding/json"
	"errors"
	"math"
	"fmt"
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/ops"
)


// Normalizes the image range to [0, 1]. Takes one input, produces one output
type OpNormalizeRange struct {
	ops.OpUnaryBase
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpNormalizeRangeDefault() })} // register the operator for JSON decoding

func NewOpNormalizeRangeDefault() *OpNormalizeRange { return NewOpNormalizeRange() }

func NewOpNormalizeRange() *OpNormalizeRange {
	op:=&OpNormalizeRange{
	  	OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "normRange"}},
  	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpNormalizeRange) UnmarshalJSON(data []byte) error {
	type defaults OpNormalizeRange
	def:=defaults( *NewOpNormalizeRangeDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpNormalizeRange(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpNormalizeRange) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if f.Stats==nil { return nil, errors.New("missing stats") }

	if f.Stats.Max()-f.Stats.Min()<1e-8 {
		fmt.Fprintf(c.Log, "%d: Warning: Image is of uniform intensity %.4g, skipping normalization\n", f.ID, f.Stats.Min())
	} else {
		fmt.Fprintf(c.Log, "%d: Normalizing from [%.4g,%.4g] to [0,1]\n", f.ID, f.Stats.Min(), f.Stats.Max())
    	f.Normalize()
	}
	return f, nil
}


type OpStretchIterative struct {
	ops.OpUnaryBase
	Location    float32   `json:"location"`
	Scale       float32   `json:"scale"`
}

var _ ops.Operator = (*OpStretchIterative)(nil) // this type is an Operator
func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpStretchIterativeDefault() })} // register the operator for JSON decoding

func NewOpStretchIterativeDefault() *OpStretchIterative { return NewOpStretchIterative(0.1, 0.004) }

// must be called /100
func NewOpStretchIterative(loc float32, scale float32) (*OpStretchIterative) {
	op:=&OpStretchIterative{ 
	  	OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "stretch"}},
		Location    : loc, 
		Scale       : scale,
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpStretchIterative) UnmarshalJSON(data []byte) error {
	type defaults OpStretchIterative
	def:=defaults( *NewOpStretchIterativeDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpStretchIterative(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpStretchIterative) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.Location==0 && op.Scale==0 { return f, nil }
	fmt.Fprintf(c.Log, "%d: Auto-stretching loc to %.2f%% and scale to %.2f%% ...\n", f.ID, op.Location*100, op.Scale*100)

	for i:=0; ; i++ {
		if i==50 { 
			fmt.Fprintf(c.Log, "%d: Warning: did not converge after %d iterations\n", f.ID, i)
			break
		}

		loc, scale:=f.Stats.Location(), f.Stats.Scale()

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
			f.ApplyGamma(idealGamma)
			//f.ApplyPartialGamma(0, 0.95, idealGamma)
		} else if loc>op.Location*0.99 && scale<op.Scale {
			fmt.Fprintf(c.Log, "scaling black to move location to %.2f%%...\n", op.Location*100)
			f.ShiftBlackToMove(loc, op.Location)
		} else {
			fmt.Fprintf(c.Log, "done\n")
			break
		}
	}
	return f, nil
}



type OpMidtones struct {
	ops.OpUnaryBase
	Mid    float32 `json:"mid"`
	Black  float32 `json:"black"`
}

var _ ops.Operator = (*OpMidtones)(nil) // this type is an Operator
func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpMidtonesDefault() })} // register the operator for JSON decoding

func NewOpMidtonesDefault() *OpMidtones { return NewOpMidtones(0, 1) }

func NewOpMidtones(mid, black float32) *OpMidtones {
	op:=&OpMidtones{
	  	OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "midtones"}},
		Mid         : mid,
		Black       : black,
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpMidtones) UnmarshalJSON(data []byte) error {
	type defaults OpMidtones
	def:=defaults( *NewOpMidtonesDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpMidtones(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpMidtones) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.Mid==0 { return f, nil }
	fmt.Fprintf(c.Log, "%d: Applying midtone correction with midtone=%.2f%% x scale and black=location - %.2f%% x scale\n", f.ID, op.Mid, op.Black)

	loc, scale:=f.Stats.Location(), f.Stats.Scale()
	absMid:=op.Mid*scale
	absBlack:=loc - op.Black*scale
	fmt.Fprintf(c.Log, "%d: loc %.2f%% scale %.2f%% absMid %.2f%% absBlack %.2f%%\n", f.ID, 100*loc, 100*scale, 100*absMid, 100*absBlack)

	f.ApplyMidtones(absMid, absBlack)
	return f, nil
}



type OpGamma struct {
	ops.OpUnaryBase
	Gamma  float32 `json:"gamma"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpGammaDefault() })} // register the operator for JSON decoding

func NewOpGammaDefault() *OpGamma { return NewOpGamma(1.0) }

func NewOpGamma(gamma float32) *OpGamma {
	op:=&OpGamma{
	  	OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "gamma"}},
		Gamma       : gamma,
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpGamma) UnmarshalJSON(data []byte) error {
	type defaults OpGamma
	def:=defaults( *NewOpGammaDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpGamma(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpGamma) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.Gamma==1.0 { return f, nil }
	fmt.Fprintf(c.Log, "%d: Applying gamma %.3g\n", f.ID, op.Gamma)
	f.ApplyGamma(op.Gamma)
	return f, nil
}


type OpGammaPP struct {
	ops.OpUnaryBase
	Gamma  float32 `json:"gamma"`
	Sigma  float32 `json:"sigma"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpGammaPPDefault() })} // register the operator for JSON decoding

func NewOpGammaPPDefault() *OpGammaPP { return NewOpGammaPP(1.0, 1.0) }

func NewOpGammaPP(gamma, sigma float32) *OpGammaPP {
	op:=&OpGammaPP{
	  	OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "gammaPP"}},
		Gamma       : gamma, 
		Sigma       : sigma,
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpGammaPP) UnmarshalJSON(data []byte) error {
	type defaults OpGammaPP
	def:=defaults( *NewOpGammaPPDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpGammaPP(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpGammaPP) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.Gamma==1.0 { return f, nil }

	loc, scale:=f.Stats.Location(), f.Stats.Scale()
	from:=loc+float32(op.Sigma)*scale
	to  :=float32(1.0)

	fmt.Fprintf(c.Log, "%d: Based on sigma=%.4g, boosting [%.2f%%, %.2f%%] with gamma %.4g...\n", 
		f.ID, op.Sigma, from*100, to*100, op.Gamma)
	f.ApplyPartialGamma(from, to, float32(op.Gamma))
	return f, nil
}



type OpScaleBlack struct {
	ops.OpUnaryBase
	Location  float32 `json:"location"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpScaleBlackDefault() })} // register the operator for JSON decoding

func NewOpScaleBlackDefault() *OpScaleBlack { return NewOpScaleBlack(0) }

func NewOpScaleBlack(location float32) *OpScaleBlack {
	op:=&OpScaleBlack{
	  	OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "scaleBlack"}},
		Location       : location,
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpScaleBlack) UnmarshalJSON(data []byte) error {
	type defaults OpScaleBlack
	def:=defaults( *NewOpScaleBlackDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpScaleBlack(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpScaleBlack) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.Location==0 { return f, nil }

	loc, scale:=f.Stats.Location(), f.Stats.Scale()
	fmt.Fprintf(c.Log, "%d: Location %.2f%% and scale %.2f%%: ", f.ID, loc*100, scale*100)

	if loc>op.Location {
		fmt.Fprintf(c.Log, "scaling black to move location to %.2f%%...\n", op.Location*100.0)
		f.ShiftBlackToMove(loc, op.Location)
	} else {
		fmt.Fprintf(c.Log, "cannot move to location %.2f%% by scaling black\n", op.Location*100.0)
	}
	return f, nil
}


// An operator to apply gaussian blurring to an image
type OpGaussianBlur struct {
	ops.OpUnaryBase
	Sigma     float32 `json:"sigma"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpGaussianBlurDefault() })} // register the operator for JSON decoding

func NewOpGaussianBlurDefault() *OpGaussianBlur { return NewOpGaussianBlur(2) }

func NewOpGaussianBlur(sigma float32) *OpGaussianBlur {
	op:=&OpGaussianBlur{
	  	OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "gaussianBlur"}},
		Sigma       : sigma, 
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpGaussianBlur) UnmarshalJSON(data []byte) error {
	type defaults OpGaussianBlur
	def:=defaults( *NewOpGaussianBlurDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpGaussianBlur(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpGaussianBlur) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.Sigma==0 { return f, nil }

	kernel:=GaussianKernel1D(op.Sigma)
	fmt.Fprintf(c.Log, "%d: Gaussian blur kernel sigma %.2f size %d: %v\n", 
		        f.ID, op.Sigma, len(kernel), kernel)
	f.Data=GaussianBlur(f.Data, int(f.Naxisn[0]), op.Sigma)
	return f, nil
}


// An operator to apply unsharp masking to an image. Based on subtraction of a gaussian blur from the original image
type OpUnsharpMask struct {
	ops.OpUnaryBase
	Sigma     float32 `json:"sigma"`
	Gain      float32 `json:"gain"`
	Threshold float32 `json:"threshold"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpUnsharpMaskDefault() })} // register the operator for JSON decoding

func NewOpUnsharpMaskDefault() *OpUnsharpMask { return NewOpUnsharpMask(1.5, 0, 1.0) }

func NewOpUnsharpMask(sigma, gain, threshold float32) *OpUnsharpMask {
	op:=&OpUnsharpMask{
	  	OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "unsharpMask"}},
		Sigma       : sigma, 
		Gain        : gain, 
		Threshold   : threshold,
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpUnsharpMask) UnmarshalJSON(data []byte) error {
	type defaults OpUnsharpMask
	def:=defaults( *NewOpUnsharpMaskDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpUnsharpMask(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpUnsharpMask) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.Sigma==0 || op.Gain==0 { return f, nil }

	absThresh:=f.Stats.Location() + f.Stats.Scale()*op.Threshold
	fmt.Fprintf(c.Log, "%d: Unsharp masking with sigma %.3g gain %.3g thresh %.3g absThresh %.3g\n", 
		        f.ID, op.Sigma, op.Gain, op.Threshold, absThresh)
	kernel:=GaussianKernel1D(op.Sigma)
	fmt.Fprintf(c.Log, "%d: Unsharp masking kernel sigma %.2f size %d: %v\n", 
		        f.ID, op.Sigma, len(kernel), kernel)
	f.Data=UnsharpMask(f.Data, int(f.Naxisn[0]), op.Sigma, op.Gain, f.Stats.Min(), f.Stats.Max(), absThresh)
	return f, nil
}
