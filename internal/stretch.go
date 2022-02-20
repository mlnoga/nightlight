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


package internal

import (
	"math"
	"io"
	"fmt"
)

type OpStretch struct {
	NormalizeRange   *OpNormalizeRange   `json:"normalizeRange"`
	StretchIterative *OpStretchIterative `json:"stretchIterative"`
	Midtones         *OpMidtones         `json:"midtones"`
	Gamma            *OpGamma            `json:"gamma"`
	PPGamma          *OpPPGamma          `json:"ppGamma"`
	ScaleBlack       *OpScaleBlack       `json:"scaleBlack"`
	StarDetect       *OpStarDetect       `json:"starDetect"`
	Align            *OpAlign            `json:"align"`
	UnsharpMask      *OpUnsharpMask      `json:"unsharpMask"`
	Save             *OpSave             `json:"save"`
	Save2            *OpSave             `json:"save2"`
}
var _ OperatorUnary = (*OpStretch)(nil) // Compile time assertion: type implements the interface

func NewOpStretch(opNormalizeRange *OpNormalizeRange, opStretchIterative *OpStretchIterative, opMidtones *OpMidtones, 
	              opGamma*OpGamma, opPPGamma *OpPPGamma, opScaleBlack*OpScaleBlack, opStarDetect *OpStarDetect, opAlign *OpAlign, 
	              opUnsharpMask *OpUnsharpMask, opSave, opSave2 *OpSave) *OpStretch {
	return &OpStretch{
		NormalizeRange   : opNormalizeRange  ,
		StretchIterative : opStretchIterative,
		Midtones         : opMidtones        ,
		Gamma            : opGamma           ,
		PPGamma          : opPPGamma         ,
		ScaleBlack       : opScaleBlack      ,
		StarDetect       : opStarDetect      ,
		Align            : opAlign           ,
		UnsharpMask      : opUnsharpMask     ,
		Save             : opSave            ,
		Save2            : opSave2           ,
	}
}

func (op *OpStretch) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if op.NormalizeRange  !=nil { if f,err=op.NormalizeRange  .Apply(f, logWriter); err!=nil { return nil, err } }
	if op.StretchIterative!=nil { if f,err=op.StretchIterative.Apply(f, logWriter); err!=nil { return nil, err } }
	if op.Midtones        !=nil { if f,err=op.Midtones        .Apply(f, logWriter); err!=nil { return nil, err } }
	if op.Gamma           !=nil { if f,err=op.Gamma           .Apply(f, logWriter); err!=nil { return nil, err } }
	if op.PPGamma         !=nil { if f,err=op.PPGamma         .Apply(f, logWriter); err!=nil { return nil, err } }
	if op.ScaleBlack      !=nil { if f,err=op.ScaleBlack      .Apply(f, logWriter); err!=nil { return nil, err } }
	if op.StarDetect      !=nil { if f,err=op.StarDetect      .Apply(f, logWriter); err!=nil { return nil, err } }
	if op.Align           !=nil { if f,err=op.Align           .Apply(f, logWriter); err!=nil { return nil, err } }
	if op.UnsharpMask     !=nil { if f,err=op.UnsharpMask     .Apply(f, logWriter); err!=nil { return nil, err } }
	if op.Save            !=nil { if f,err=op.Save            .Apply(f, logWriter); err!=nil { return nil, err } }
	if op.Save2           !=nil { if f,err=op.Save2           .Apply(f, logWriter); err!=nil { return nil, err } }
	return f, nil
}

type OpNormalizeRange struct {
	Active bool `json:"active"`
}

func NewOpNormalizeRange(active bool) *OpNormalizeRange {
	return &OpNormalizeRange{active}
}

func (op *OpNormalizeRange) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }

	if f.Stats==nil {
		f.Stats, err=CalcExtendedStats(f.Data, f.Naxisn[0])
		if err!=nil { return nil, err }
	}

	if f.Stats.Min==f.Stats.Max {
		fmt.Fprintf(logWriter, "%d: Warning: Image is of uniform intensity %.4g, skipping normalization\n", f.ID, f.Stats.Min)
	} else {
		fmt.Fprintf(logWriter, "%d: Normalizing from [%.4g,%.4g] to [0,1]\n", f.ID, f.Stats.Min, f.Stats.Max)
    	f.Normalize()
    	var err error
		f.Stats, err=CalcExtendedStats(f.Data, f.Naxisn[0])
		if err!=nil { return nil, err }
	}
	return f, nil
}



type OpStretchIterative struct {
	Active      bool      `json:"active"`
	Location    float32   `json:"location"`
	Scale       float32   `json:"scale"`
}

// must be called /100
func NewOpStretchIterative(loc float32, scale float32) (*OpStretchIterative) {
	return &OpStretchIterative{ loc!=0 && scale!=0, loc, scale }
}

func (op *OpStretchIterative) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	fmt.Fprintf(logWriter, "%d: Auto-stretching loc to %.2f%% and scale to %.2f%% ...\n", f.ID, op.Location*100, op.Scale*100)

	//gammaLimit:=float32(5.0)
	for i:=0; ; i++ {
		if i==50 { 
			fmt.Fprintf(logWriter, "Warning: did not converge after %d iterations\n",i)
			break
		}

		// calculate basic image stats as a fast location and scale estimate
		var err error
		f.Stats,err=CalcExtendedStats(f.Data, f.Naxisn[0])
		if err!=nil { LogFatal(err) }
		loc, scale:=f.Stats.Location, f.Stats.Scale

		fmt.Fprintf(logWriter, "Linear location %.2f%% and scale %.2f%%, ", loc*100, scale*100)

		if loc<=op.Location*1.01 && scale<op.Scale {
			idealGamma:=float32(1)
			idealGammaDelta:=float32(math.Abs(float64(op.Scale)-float64(scale)))

			//gammaMin:=float32(1)
			//gammaMax:=float32(25)
			for gamma:=float32(1.0); gamma<=8; gamma+=0.01 {
				//gammaMid:=0.5*(gammaMin+gammaMax)
				exponent:=1.0/float64(gamma)
				newLocLower:=float32(math.Pow(float64(loc-scale), exponent))
				newLoc     :=float32(math.Pow(float64(loc        ), exponent))
				newLocUpper:=float32(math.Pow(float64(loc+scale), exponent))

				black:=(op.Location-newLoc)/(op.Location-1)
    			scale:=1/(1-black)

				scaledNewLocLower:=float32(math.Max(0, float64((newLocLower - black) * scale)))
				//scaledNewLoc     :=float32(math.Max(0, float64((newLoc      - black) * scale)))
				scaledNewLocUpper:=float32(math.Max(0, float64((newLocUpper - black) * scale)))

				newScale:=float32(scaledNewLocUpper-scaledNewLocLower)/2
				delta:=float32(math.Abs(float64(op.Scale)-float64(newScale)))
				if delta<idealGammaDelta {
				   // fmt.Fprintf(logWriter, "gamma %.2f lower %.2f%% upper %.2f%% newScale %.2f%% op.Scale %.2f%% delta %.2f%% \n", gamma, scaledNewLocLower*100, scaledNewLocUpper*100, newScale*100, op.Scale*100, delta*100)
					idealGamma=gamma
					idealGammaDelta=delta
				}
/*				if newScale>op.Scale*1.01 {
					gammaMax=gammaMid
				} else if newScale<op.Scale*1.01 {
					gammaMin=gammaMid
				} else {
					idealGamma=gammaMid
					break
				} */
			}

			//idealGamma:=float32(math.Log((float64(op.Location)/float64(op.Scale))*float64(scale))/math.Log(float64(op.Location)))
			//if idealGamma>gammaLimit { idealGamma=gammaLimit }
			if idealGamma<=1.01 { 
				fmt.Fprintf(logWriter, "done\n")
				break
			}

			fmt.Fprintf(logWriter, "applying gamma %.3g\n", idealGamma)
			f.ApplyGamma(idealGamma)
		} else if loc>op.Location*0.99 && scale<op.Scale {
			fmt.Fprintf(logWriter, "scaling black to move location to %.2f%%...\n", op.Location*100)
			f.ShiftBlackToMove(loc, op.Location)
		} else {
			fmt.Fprintf(logWriter, "done\n")
			break
		}
	}
	return f, nil
}




type OpMidtones struct {
	Active bool    `json:"active"`
	Mid    float32 `json:"mid"`
	Black  float32 `json:"black"`
}

func NewOpMidtones(mid, black float32) *OpMidtones {
	return &OpMidtones{
		Active: mid!=0,
		Mid:    mid,
		Black:  black,
	}
}

func (op *OpMidtones) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	fmt.Fprintf(logWriter, "Applying midtone correction with midtone=%.2f%% x scale and black=location - %.2f%% x scale\n", op.Mid, op.Black)

	stats,err:=CalcExtendedStats(f.Data, f.Naxisn[0])
	if err!=nil { return nil, err }
	loc, scale:=stats.Location, stats.Scale
	absMid:=op.Mid*scale
	absBlack:=loc - op.Black*scale
	fmt.Fprintf(logWriter, "loc %.2f%% scale %.2f%% absMid %.2f%% absBlack %.2f%%\n", 100*loc, 100*scale, 100*absMid, 100*absBlack)
	f.ApplyMidtones(absMid, absBlack)
	return f, nil
}




type OpGamma struct {
	Active bool    `json:"active"`
	Gamma  float32 `json:"gamma"`
}

func NewOpGamma(gamma float32) *OpGamma {
	return &OpGamma{gamma!=1.0, gamma}
}

func (op *OpGamma) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	fmt.Fprintf(logWriter, "Applying gamma %.3g\n", op.Gamma)
	f.ApplyGamma(op.Gamma)
	return f, nil
}



type OpPPGamma struct {
	Active bool `json:"active"`
	Gamma  float32 `json:"gamma"`
	Sigma  float32 `json:"sigma"`
}

func NewOpPPGamma(gamma, sigma float32) *OpPPGamma {
	return &OpPPGamma{gamma!=1.0, gamma, sigma}
}

func (op *OpPPGamma) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	stats,err:=CalcExtendedStats(f.Data, f.Naxisn[0])
	if err!=nil { return nil, err }
	loc, scale:=stats.Location, stats.Scale

	from:=loc+float32(op.Sigma)*scale
	to  :=float32(1.0)
	fmt.Fprintf(logWriter, "Based on sigma=%.4g, boosting values in [%.2f%%, %.2f%%] with gamma %.4g...\n", 
		op.Sigma, from*100, to*100, op.Gamma)
	f.ApplyPartialGamma(from, to, float32(op.Gamma))
	return f, nil
}




type OpScaleBlack struct {
	Active bool `json:"active"`
	Black  float32 `json:"value"`
}

func NewOpScaleBlack(black float32) *OpScaleBlack {
	return &OpScaleBlack{black!=0, black}
}

func (op *OpScaleBlack) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }

	stats,err:=CalcExtendedStats(f.Data, f.Naxisn[0])
	if err!=nil { LogFatal(err) }
	loc, scale:=stats.Location, stats.Scale
	fmt.Fprintf(logWriter, "Location %.2f%% and scale %.2f%%: ", loc*100, scale*100)

	if loc>op.Black {
		fmt.Fprintf(logWriter, "scaling black to move location to %.2f%%...\n", op.Black*100.0)
		f.ShiftBlackToMove(loc, op.Black)
	} else {
		fmt.Fprintf(logWriter, "cannot move to location %.2f%% by scaling black\n", op.Black*100.0)
	}
	return f, nil
}


type OpUnsharpMask struct {
	Active    bool    `json:"active"`
	Sigma     float32 `json:"sigma"`
	Gain      float32 `json:"gain"`
	Threshold float32 `json:"threshold"`
}

func NewOpUnsharpMask(sigma, gain, threshold float32) *OpUnsharpMask {
	return &OpUnsharpMask{gain>0, sigma, gain, threshold}
}

func (op *OpUnsharpMask) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }
	f.Stats, err=CalcExtendedStats(f.Data, f.Naxisn[0])
	if err!=nil { return nil, err }

	absThresh:=f.Stats.Location + f.Stats.Scale*op.Threshold
	fmt.Fprintf(logWriter, "%d: Unsharp masking with sigma %.3g gain %.3g thresh %.3g absThresh %.3g\n", f.ID, op.Sigma, op.Gain, op.Threshold, absThresh)
	kernel:=GaussianKernel1D(op.Sigma)
	fmt.Fprintf(logWriter, "Unsharp masking kernel sigma %.2f size %d: %v\n", op.Sigma, len(kernel), kernel)
	f.Data=UnsharpMask(f.Data, int(f.Naxisn[0]), op.Sigma, op.Gain, f.Stats.Min, f.Stats.Max, absThresh)
	return f, nil
}

