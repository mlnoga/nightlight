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


package stack

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/stats"
	"github.com/mlnoga/nightlight/internal/qsort"
	"github.com/mlnoga/nightlight/internal/ops"
)


type StackMode int
const (
	StMedian StackMode = iota
	StMean 
	StSigma
	StWinsorSigma
	StMADSigma
	StLinearFit
	StAuto
)

// Auto-select stacking mode based on number of frames
func autoSelectStackingMode(l int) StackMode {
	if l>=25 {
		return StLinearFit   
    } else if l>=15 {
    	return StWinsorSigma 
    } else if l>= 6 {
    	return StSigma       
    } else {
    	return StMean      
    }
}

type StackWeighting int
const (
	StWeightNone StackWeighting = iota
	StWeightExposure
	StWeightInverseNoise
	StWeightInverseHFR
)


type OpStack struct {
	ops.OpBase
	Mode         StackMode       `json:"mode"`
	Weighting    StackWeighting  `json:"weighting"`
	SigmaLow     float32         `json:"sigmaLow"`
	SigmaHigh    float32         `json:"sigmaHigh"`
	RefFrameLoc  float32         `json:"-"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpStackDefault() })} // register the operator for JSON decoding

func NewOpStackDefault() *OpStack { return NewOpStack(StAuto, StWeightNone, 2.75, 2.75) }

func NewOpStack(mode StackMode, weighting StackWeighting, sigmaLow, sigmaHigh float32) *OpStack {
	op:=&OpStack{
	  	OpBase      : ops.OpBase{Type: "stack"},
		Mode        : mode, 
		Weighting   : weighting, 
		SigmaLow    : sigmaLow, 
		SigmaHigh   : sigmaHigh, 
		RefFrameLoc : 0,
	}
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpStack) UnmarshalJSON(data []byte) error {
	type defaults OpStack
	def:=defaults( *NewOpStackDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpStack(def)
	return nil
}


func (op *OpStack) MakePromises(ins []ops.Promise, c *ops.Context) (outs []ops.Promise, err error) {
	if len(ins)==0 { return nil, errors.New(fmt.Sprintf("%s operator needs inputs", op.Type)) }

	out:=func() (f *fits.Image, err error) {
		fs, err:=ops.MaterializeAll(ins, c.MaxThreads, false) // materialize all input promises
		if err!=nil { return nil, err }
		return op.Apply(fs, c)
	}
	return []ops.Promise{out}, nil
}


// Stack a set of light frames. Limits parallelism to the number of available cores
func (op *OpStack) Apply(f []*fits.Image, c *ops.Context) (result *fits.Image, err error) {
	// validate stacking modes and perform automatic mode selection if necesssary
	mode:=op.Mode
	if mode<StMedian || mode>StAuto {
		return nil, errors.New("invalid stacking mode")
	}
	if mode==StAuto { 
		mode=autoSelectStackingMode(len(f))
	}
	fmt.Fprintf(c.Log, "Stacking %d frames with stacking mode %d and sigma low %g high %g:\n", 
				len(f), mode, op.SigmaLow, op.SigmaHigh)

	// select weights if applicable
	weights, err:=getWeights(f, op.Weighting)
	if err!=nil { return nil, err }

	// create return value array
	data:=make([]float32,len(f[0].Data))

	// split into 8 MB work packages, no fewer than 8*NumCPU()
	numBatches:=4*len(f)*len(f[0].Data)/(8192*1024)
	if numBatches < 8*runtime.NumCPU() { numBatches=8*runtime.NumCPU() }
	batchSize:=(len(data)+numBatches-1)/(numBatches)
	sem   :=make(chan bool, runtime.NumCPU()) // limit parallelism to NumCPUs()

	numClippedLock, numClippedLow, numClippedHigh:=sync.Mutex{}, int32(0), int32(0)
	progressLock, progress:=sync.Mutex{}, float32(0)
	for lower:=0; lower<len(data); lower+=batchSize {
		upper:=lower+batchSize
		if upper>len(data) { upper=len(data) }

		sem <- true 
		go func(lower, upper int) {
			defer func() { <-sem }()

			// subslice f data elements for given batch
			ldBatch:=make([][]float32, len(f))
			for i, l:=range f { ldBatch[i]=l.Data[lower:upper] }

			var clipLow, clipHigh int32
			// run stacking for the given batch
			switch mode {
			case StMedian:
				StackMedian(ldBatch, op.RefFrameLoc, data[lower:upper])

			case StMean: 
				if weights==nil {
					StackMean(ldBatch, op.RefFrameLoc, data[lower:upper])
				} else {
					StackMeanWeighted(ldBatch, weights, op.RefFrameLoc, data[lower:upper])
				}

			case StSigma:
				if weights==nil {
					clipLow, clipHigh=StackSigma(ldBatch, op.RefFrameLoc, op.SigmaLow, op.SigmaHigh, data[lower:upper])
				} else {
					clipLow, clipHigh=StackSigmaWeighted(ldBatch, weights, op.RefFrameLoc, op.SigmaLow, op.SigmaHigh, data[lower:upper])
				}

			case StWinsorSigma:
				if weights==nil {
					clipLow, clipHigh=StackWinsorSigma(ldBatch, op.RefFrameLoc, op.SigmaLow, op.SigmaHigh, data[lower:upper])
				} else {
					clipLow, clipHigh=StackWinsorSigmaWeighted(ldBatch, weights, op.RefFrameLoc, op.SigmaLow, op.SigmaHigh, data[lower:upper])
				}

			case StMADSigma:
				if weights==nil {
					clipLow, clipHigh=StackMADSigma(ldBatch, op.RefFrameLoc, op.SigmaLow, op.SigmaHigh, data[lower:upper])
				} else {
					panic("MADSigma stacking with weights is still unimplemented")
				}

			case StLinearFit:
				clipLow, clipHigh=StackLinearFit(ldBatch, op.RefFrameLoc, op.SigmaLow, op.SigmaHigh, data[lower:upper])
			} 

			// update clipping totals
			if clipLow>0 || clipHigh>0 {
				numClippedLock.Lock()
				numClippedLow+=clipLow
				numClippedHigh+=clipHigh
				numClippedLock.Unlock()
			}

			// display progress indicator
			progressLock.Lock()
			progress+=float32(batchSize)/float32(len(data))
			fmt.Fprintf(c.Log, "\r%d%%", int(progress*100))
			progressLock.Unlock()

		}(lower, upper)
	}
	for i:=0; i<cap(sem); i++ {  // wait for goroutines to finish
		sem <- true
	}
	fmt.Fprintf(c.Log, "\r")

	// report back on clipping for modes that apply clipping
	if mode>=StSigma {
		fmt.Fprintf(c.Log, "Clipped low %d (%.2f%%) high %d (%.2f%%)\n", 
			numClippedLow,  float32(numClippedLow )*100.0/(float32(len(data)*len(f))),
			numClippedHigh, float32(numClippedHigh)*100.0/(float32(len(data)*len(f))) )
	}

	exposureSum:=float32(0)
	for _,l :=range f { exposureSum+=l.Exposure }

	// Assemble into in-memory FITS
	stack:=fits.NewImageFromNaxisn(f[0].Naxisn, data)
	stack.Exposure = exposureSum
	return stack, nil
}


// Prepare weights for stacking based on selected weighting mode and given images 
func getWeights(f []*fits.Image, weighting StackWeighting) (weights []float32, err error)  {
	weights=[]float32(nil)
	if weighting==StWeightNone {
		weights=nil
	} else if weighting==StWeightExposure { // exposure weighted stacking, longer exposure gets bigger weight
		weights =make([]float32, len(f))
		for i:=0; i<len(f); i+=1 {
			if f[i].Exposure==0 { return nil, errors.New(fmt.Sprintf("%d: Missing exposure information for exposure-weighted stacking", f[i].ID)) }
			weights[i]=f[i].Exposure
		}
	} else if weighting==StWeightInverseNoise { // noise weighted stacking, smaller noise gets bigger weight
		minNoise, maxNoise:=float32(math.MaxFloat32), float32(-math.MaxFloat32)
		for i:=0; i<len(f); i+=1 {
			if f[i].Stats==nil { return nil, errors.New(fmt.Sprintf("%d: Missing stats information for noise-weighted stacking", f[i].ID)) }
			n:=f[i].Stats.Noise()
			if n<minNoise { minNoise=n }
			if n>maxNoise { maxNoise=n }
		}		
		weights =make([]float32, len(f))
		for i:=0; i<len(f); i+=1 {
			//f[i].Stats.Noise=nl.EstimateNoise(f[i].Data, f[i].Naxisn[0])
			weights[i]=1/(1+4*(f[i].Stats.Noise()-minNoise)/(maxNoise-minNoise))
		}
	} else if weighting==StWeightInverseHFR { // HFR weighted stacking, smaller HFR gets bigger weight
		minHFR, maxHFR:=float32(math.MaxFloat32), float32(-math.MaxFloat32)
		for i:=0; i<len(f); i+=1 {
			h:=f[i].HFR
			if h<minHFR { minHFR=h }
			if h>maxHFR { maxHFR=h }
		}		
		weights =make([]float32, len(f))
		for i:=0; i<len(f); i+=1 {
			//f[i].Stats.Noise=nl.EstimateNoise(f[i].Data, f[i].Naxisn[0])
			weights[i]=1/(1+4*(f[i].HFR-minHFR)/(maxHFR-minHFR))
		}
	} else {
		return nil, errors.New(fmt.Sprintf("Invalid weighting mode %d\n", weighting))
	}
	return weights, nil
}


// Stacking with median function
func StackMedian(lightsData [][]float32, RefFrameLoc float32, res []float32) {
	gatheredFull:=make([]float32,len(lightsData))

	// for all pixels
	for i, _:=range lightsData[0] {
		// gather data for this pixel across all lights, skipping NaNs
		numGathered:=0
		for li, _:=range lightsData {
			value:=lightsData[li][i]
			if !math.IsNaN(float64(value)) {
				gatheredFull[numGathered]=value
				numGathered++
			}
		}
		if numGathered==0 {
			// If no valid data points available, replace with overall mean.
			// This is subobptimal, but NaN would break subsequent processing,
			// unless all operations are made NaN-proof. As IEEE NaN does not
			// compare equal to itself, this would require a full reimplementation
			// of basic partitioning and sorting primitives on float32. 
			// Not going down that rabbit hole for now. 
			res[i]=RefFrameLoc 
			continue	
		}
		gatheredCur:=gatheredFull[:numGathered]

		res[i]=qsort.QSelectMedianFloat32(gatheredCur)
	}
	gatheredFull=nil
}


// Stacking with mean function
func StackMean(lightsData [][]float32, RefFrameLoc float32, res []float32) {
	// for all pixels
	for i, _:=range res {

		// gather data for this pixel across all lights, skipping NaNs
		numGathered:=0
		sum:=float32(0)
		for li, _:=range lightsData {
			value:=lightsData[li][i]
			if !math.IsNaN(float64(value)) {
				sum+=value
				numGathered++
			}
		}
		if numGathered==0 {
			// If no valid data points available, replace with overall mean.
			// This is subobptimal, but NaN would break subsequent processing,
			// unless all operations are made NaN-proof. As IEEE NaN does not
			// compare equal to itself, this would require a full reimplementation
			// of basic partitioning and sorting primitives on float32. 
			// Not going down that rabbit hole for now. 
			res[i]=RefFrameLoc 
			continue	
		}
		res[i]=sum/float32(numGathered)
	}
}


// Stacking with mean function and weights
func StackMeanWeighted(lightsData [][]float32, weights []float32, RefFrameLoc float32, res []float32) {
	// for all pixels
	for i, _:=range res {

		// gather data for this pixel across all lights, skipping NaNs
		numGathered:=0
		sum:=float32(0)
		weightSum:=float32(0)
		for li, _:=range lightsData {
			value:=lightsData[li][i]
			if !math.IsNaN(float64(value)) {
				weight:=weights[li]
				sum+=value*weight
				weightSum+=weight
				numGathered++
			}
		}
		if numGathered==0 {
			// If no valid data points available, replace with overall mean.
			// This is subobptimal, but NaN would break subsequent processing,
			// unless all operations are made NaN-proof. As IEEE NaN does not
			// compare equal to itself, this would require a full reimplementation
			// of basic partitioning and sorting primitives on float32. 
			// Not going down that rabbit hole for now. 
			res[i]=RefFrameLoc 
			continue	
		}
		res[i]=sum/float32(weightSum)
	}
}


// Mean stacking with sigma clipping. Values which are more than sigmaLow/sigmaHigh
// standard deviations away from the mean are excluded from the average calculation.
// The standard deviation is calculated w.r.t the mean for robustness.
func StackSigma(lightsData [][]float32, RefFrameLoc, sigmaLow, sigmaHigh float32, res []float32) (clipLow, clipHigh int32) {
	gatheredFull:=make([]float32,len(lightsData))
	numClippedLow, numClippedHigh:=int32(0), int32(0)

	// for all pixels
	for i, _:=range lightsData[0] {

		// gather data for this pixel across all lights, skipping NaNs
		numGathered:=0
		for li, _:=range lightsData {
			value:=lightsData[li][i]
			if !math.IsNaN(float64(value)) {
				gatheredFull[numGathered]=value
				numGathered++
			}
		}
		if numGathered==0 {
			// If no valid data points available, replace with overall mean.
			// This is subobptimal, but NaN would break subsequent processing,
			// unless all operations are made NaN-proof. As IEEE NaN does not
			// compare equal to itself, this would require a full reimplementation
			// of basic partitioning and sorting primitives on float32. 
			// Not going down that rabbit hole for now. 
			res[i]=RefFrameLoc 
			continue	
		}
		gatheredCur:=gatheredFull[:numGathered]

		// repeat until results for this pixelare stable
		for {

			// calculate median, mean, standard deviation and variance across gathered data
			median:=qsort.QSelectMedianFloat32(gatheredCur)
			mean, stdDev:=stats.MeanStdDev(gatheredCur)

			// remove out-of-bounds values
			lowBound :=median - sigmaLow *stdDev
			highBound:=median + sigmaHigh*stdDev
			prevClipped:=numClippedLow+numClippedHigh
			for j:=0; j<len(gatheredCur); j++ {
				g:=gatheredCur[j]
				if g<lowBound {
					gatheredCur[j]=gatheredCur[len(gatheredCur)-1]
					gatheredCur=gatheredCur[:len(gatheredCur)-1]
					numClippedLow++
					j--
				} else if g>highBound {
					gatheredCur[j]=gatheredCur[len(gatheredCur)-1]
					gatheredCur=gatheredCur[:len(gatheredCur)-1]
					numClippedHigh++
					j--
				}
			}

			// terminate if no more values are out of bounds, or all but one value consumed
            if (numClippedLow+numClippedHigh)==prevClipped || len(gatheredCur)<=1 {
				res[i]=mean
            	break
            }
		}
	}

	gatheredFull=nil
	return numClippedLow, numClippedHigh
}


// Weighted mean stacking with sigma clipping. Values which are more than sigmaLow/sigmaHigh
// standard deviations away from the mean are excluded from the average calculation.
// The standard deviation is calculated w.r.t the mean for robustness.
func StackSigmaWeighted(lightsData [][]float32, weights []float32, RefFrameLoc, sigmaLow, sigmaHigh float32, res []float32) (clipLow, clipHigh int32) {
	gatheredFull:=make([]float32,len(lightsData))
	weightsFull :=make([]float32,len(weights))
	numClippedLow, numClippedHigh:=int32(0), int32(0)

	// for all pixels
	for i, _:=range lightsData[0] {

		// gather data for this pixel across all lights, skipping NaNs
		numGathered:=0
		for li, _:=range lightsData {
			value:=lightsData[li][i]
			if !math.IsNaN(float64(value)) {
				gatheredFull[numGathered]=value
				weightsFull[numGathered]=weights[li]
				numGathered++
			}
		}
		if numGathered==0 {
			// If no valid data points available, replace with overall mean.
			// This is subobptimal, but NaN would break subsequent processing,
			// unless all operations are made NaN-proof. As IEEE NaN does not
			// compare equal to itself, this would require a full reimplementation
			// of basic partitioning and sorting primitives on float32. 
			// Not going down that rabbit hole for now. 
			res[i]=RefFrameLoc 
			continue	
		}
		gatheredCur:=gatheredFull[:numGathered]
		weightsCur :=weightsFull [:numGathered]

		/*
		// gather data for this pixel across all frames
		for li, _:=range lightsData {
			gatheredFull[li]=lightsData[li][i]
		}
		gatheredCur:=gatheredFull
		copy(weightsFull, weights)
		weightsCur :=weightsFull
		*/

		// repeat until results for this pixelare stable
		for {

			// calculate median, mean, standard deviation and variance across gathered data
			median:=qsort.QSelectMedianFloat32(gatheredCur)
			_, stdDev:=stats.MeanStdDev(gatheredCur)

			// remove out-of-bounds values
			lowBound :=median - sigmaLow *stdDev
			highBound:=median + sigmaHigh*stdDev
			prevClipped:=numClippedLow+numClippedHigh
			for j:=0; j<len(gatheredCur); j++ {
				g:=gatheredCur[j]
				if g<lowBound {
					gatheredCur[j]=gatheredCur[ len(gatheredCur)-1]
					gatheredCur   =gatheredCur[:len(gatheredCur)-1]
					weightsCur[j] =weightsCur [ len(weightsCur )-1]
					weightsCur    =weightsCur [:len(weightsCur )-1]
					numClippedLow++
					j--
				} else if g>highBound {
					gatheredCur[j]=gatheredCur[ len(gatheredCur)-1]
					gatheredCur   =gatheredCur[:len(gatheredCur)-1]
					weightsCur[j] =weightsCur [ len(weightsCur )-1]
					weightsCur    =weightsCur [:len(weightsCur )-1]
					numClippedHigh++
					j--
				}
			}

			// terminate if no more values are out of bounds, or all but one value consumed
            if (numClippedLow+numClippedHigh)==prevClipped || len(gatheredCur)<=1 {
            	// calculate weighted mean
            	weightedSum, weightsSum:=float32(0), float32(0)
            	for i,_:=range gatheredCur {
            		weightedSum+=gatheredCur[i] * weightsCur[i]
            		weightsSum +=weightsCur[i]
            	}
				res[i]=weightedSum/weightsSum
            	break
            }
		}
	}

	gatheredFull=nil
	weightsFull=nil

	return numClippedLow, numClippedHigh
}


// Mean stacking with sigma clipping. Values which are more than sigmaLow/sigmaHigh
// MADs away from the median are excluded from the average calculation.
func StackMADSigma(lightsData [][]float32, RefFrameLoc, sigmaLow, sigmaHigh float32, res []float32) (clipLow, clipHigh int32) {
	gatheredFull:=make([]float32,len(lightsData))
	adGatheredFull:=make([]float32,len(lightsData))
	numClippedLow, numClippedHigh:=int32(0), int32(0)

	// for all pixels
	for i, _:=range lightsData[0] {

		// gather data for this pixel across all lights, skipping NaNs
		numGathered:=0
		for li, _:=range lightsData {
			value:=lightsData[li][i]
			if !math.IsNaN(float64(value)) {
				gatheredFull[numGathered]=value
				numGathered++
			}
		}
		if numGathered==0 {
			// If no valid data points available, replace with overall mean.
			// This is subobptimal, but NaN would break subsequent processing,
			// unless all operations are made NaN-proof. As IEEE NaN does not
			// compare equal to itself, this would require a full reimplementation
			// of basic partitioning and sorting primitives on float32. 
			// Not going down that rabbit hole for now. 
			res[i]=RefFrameLoc 
			continue	
		}
		gatheredCur:=gatheredFull[:numGathered]

		// calculate median across gathered data
		median:=qsort.QSelectMedianFloat32(gatheredCur)

		// calculate median absolute distance (MAD)
		adGatheredCur:=adGatheredFull[:numGathered]
		for i,g:=range gatheredCur {
			ad:=g-median
			if ad<0 { ad=-ad }
			adGatheredCur[i]=ad
		}
		mad:=qsort.QSelectMedianFloat32(adGatheredCur)
		stdDev:=mad*1.4826  // normalize to Gaussian std dev equivalent value

		// remove out-of-bounds values
		lowBound :=median - sigmaLow *stdDev
		highBound:=median + sigmaHigh*stdDev
		for j:=0; j<len(gatheredCur); {
			g:=gatheredCur[j]
			if g<lowBound {
				gatheredCur[j]=gatheredCur[len(gatheredCur)-1]
				gatheredCur=gatheredCur[:len(gatheredCur)-1]
				numClippedLow++
			} else if g>highBound {
				gatheredCur[j]=gatheredCur[len(gatheredCur)-1]
				gatheredCur=gatheredCur[:len(gatheredCur)-1]
				numClippedHigh++
			} else {
				j++
			}
		}

		// calculate mean
		mean:=float32(0)
		for _,g:=range(gatheredCur) { mean+=g }
		mean/=float32(len(gatheredCur))
		res[i]=mean		
	}

	gatheredFull=nil
	return numClippedLow, numClippedHigh
}



// Weighted mean stacking with sigma clipping. Values which are more than sigmaLow/sigmaHigh
// standard deviations away from the mean are replaced with the lowest/highest valid value.
func StackWinsorSigma(lightsData [][]float32, RefFrameLoc, sigmaLow, sigmaHigh float32, res []float32) (clipLow, clipHigh int32) {
	gatheredFull  :=make([]float32,len(lightsData))
	winsorizedFull:=make([]float32,len(lightsData))
	numClippedLow, numClippedHigh:=int32(0), int32(0)

	// for all pixels
	for i, _:=range lightsData[0] {

		// gather data for this pixel across all lights, skipping NaNs
		numGathered:=0
		for li, _:=range lightsData {
			value:=lightsData[li][i]
			if !math.IsNaN(float64(value)) {
				gatheredFull[numGathered]=value
				numGathered++
			}
		}
		if numGathered==0 {
			// If no valid data points available, replace with overall mean.
			// This is subobptimal, but NaN would break subsequent processing,
			// unless all operations are made NaN-proof. As IEEE NaN does not
			// compare equal to itself, this would require a full reimplementation
			// of basic partitioning and sorting primitives on float32. 
			// Not going down that rabbit hole for now. 
			res[i]=RefFrameLoc 
			continue	
		}
		gatheredCur:=gatheredFull[:numGathered]

		// repeat until results for this pixel are stable
		for {
			// calculate median and standard deviation across all frames
			median:=qsort.QSelectMedianFloat32(gatheredCur)
			mean, stdDev:=stats.MeanStdDev(gatheredCur)

			// calculate winsorized standard deviation (removes outliers/tighter)
			winsorized:=winsorizedFull[0:len(gatheredCur)]
			copy(winsorized, gatheredCur)
			for {
				// replace outliers with low/high bound
				lowBound :=median - 1.5*stdDev
				highBound:=median + 1.5*stdDev
				changed:=0
				for i, w :=range winsorized {
					if w<lowBound { 
						winsorized[i]=lowBound
						changed++
					} else if w>highBound {
						winsorized[i]=highBound
						changed++
					}
				}
				// median is invariant to outlier substitution, no need to recompute
				oldStdDev:=stdDev
				_, stdDev=stats.MeanStdDev(winsorized) // also keep original mean
				stdDev=1.134*stdDev

				factor:=float32(math.Abs(float64(stdDev-oldStdDev)))/oldStdDev
				if changed==0 || factor <= 0.0005 {
					break
				}
			}

			// remove out-of-bounds values
			lowBound :=median - sigmaLow *stdDev
			highBound:=median + sigmaHigh*stdDev
			prevClipped:=numClippedLow+numClippedHigh
			for j:=0; j<len(gatheredCur); j++ {
				g:=gatheredCur[j]
				if g<lowBound {
					gatheredCur[j]=gatheredCur[ len(gatheredCur)-1]
					gatheredCur   =gatheredCur[:len(gatheredCur)-1]
					numClippedLow++
					j--
				} else if g>highBound {
					gatheredCur[j]=gatheredCur[ len(gatheredCur)-1]
					gatheredCur   =gatheredCur[:len(gatheredCur)-1]
					numClippedHigh++
					j--
				}
			}

			// terminate if no more values are out of bounds, or all but one value consumed
            if (numClippedLow+numClippedHigh)==prevClipped || len(gatheredCur)<=1 {
				res[i]=mean
            	break
            }
        }
	}

	gatheredFull=nil
	winsorizedFull=nil

	return numClippedLow, numClippedHigh
}


// Weighted mean stacking with sigma clipping. Values which are more than sigmaLow/sigmaHigh
// standard deviations away from the mean are replaced with the lowest/highest valid value.
func StackWinsorSigmaWeighted(lightsData [][]float32, weights []float32, RefFrameLoc, sigmaLow, sigmaHigh float32, res []float32) (clipLow, clipHigh int32) {
	gatheredFull  :=make([]float32,len(lightsData))
	weightsFull   :=make([]float32,len(weights))
	winsorizedFull:=make([]float32,len(lightsData))
	numClippedLow, numClippedHigh:=int32(0), int32(0)

	// for all pixels
	for i, _:=range lightsData[0] {

		// gather data for this pixel across all lights, skipping NaNs
		numGathered:=0
		for li, _:=range lightsData {
			value:=lightsData[li][i]
			if !math.IsNaN(float64(value)) {
				gatheredFull[numGathered]=value
				weightsFull[numGathered]=weights[li]
				numGathered++
			}
		}
		if numGathered==0 {
			// If no valid data points available, replace with overall mean.
			// This is subobptimal, but NaN would break subsequent processing,
			// unless all operations are made NaN-proof. As IEEE NaN does not
			// compare equal to itself, this would require a full reimplementation
			// of basic partitioning and sorting primitives on float32. 
			// Not going down that rabbit hole for now. 
			res[i]=RefFrameLoc 
			continue	
		}
		gatheredCur:=gatheredFull[:numGathered]
		weightsCur :=weightsFull [:numGathered]

		/*
		// gather data for this pixel across all frames
		for li, _:=range lightsData {
			gatheredFull[li]=lightsData[li][i]
		}
		gatheredCur:=gatheredFull
		copy(weightsFull, weights)
		weightsCur :=weightsFull
		*/

		// repeat until results for this pixel are stable
		for {

			// calculate median and standard deviation across all frames
			median:=qsort.QSelectMedianFloat32(gatheredCur)
			_, stdDev:=stats.MeanStdDev(gatheredCur)

			// calculate winsorized standard deviation (removes outliers/tighter)
			winsorized:=winsorizedFull[0:len(gatheredCur)]
			copy(winsorized, gatheredCur)
			for {
				// replace outliers with low/high bound
				lowBound :=median - 1.5*stdDev
				highBound:=median + 1.5*stdDev
				changed:=0
				for i, w :=range winsorized {
					if w<lowBound { 
						winsorized[i]=lowBound
						changed++
					} else if w>highBound {
						winsorized[i]=highBound
						changed++
					}
				}
				// median is invariant to outlier substitution, no need to recompute
				oldStdDev:=stdDev
				_, stdDev=stats.MeanStdDev(winsorized) // also keep original mean
				stdDev=1.134*stdDev

				factor:=float32(math.Abs(float64(stdDev-oldStdDev)))/oldStdDev
				if changed==0 || factor <= 0.0005 {
					break
				}
			}

			// remove out-of-bounds values
			lowBound :=median - sigmaLow *stdDev
			highBound:=median + sigmaHigh*stdDev
			prevClipped:=numClippedLow+numClippedHigh
			for j:=0; j<len(gatheredCur); j++ {
				g:=gatheredCur[j]
				if g<lowBound {
					gatheredCur[j]=gatheredCur[ len(gatheredCur)-1]
					gatheredCur   =gatheredCur[:len(gatheredCur)-1]
					weightsCur[j] =weightsCur [ len(weightsCur )-1]
					weightsCur    =weightsCur [:len(weightsCur )-1]
					numClippedLow++
					j--
				} else if g>highBound {
					gatheredCur[j]=gatheredCur[ len(gatheredCur)-1]
					gatheredCur   =gatheredCur[:len(gatheredCur)-1]
					weightsCur[j] =weightsCur [ len(weightsCur )-1]
					weightsCur    =weightsCur [:len(weightsCur )-1]
					numClippedHigh++
					j--
				}
			}

			// terminate if no more values are out of bounds, or all but one value consumed
            if (numClippedLow+numClippedHigh)==prevClipped || len(gatheredCur)<=1 {
            	// calculate weighted mean
            	weightedSum, weightsSum:=float32(0), float32(0)
            	for i,_:=range gatheredCur {
            		weightedSum+=gatheredCur[i] * weightsCur[i]
            		weightsSum +=weightsCur[i]
            	}
				res[i]=weightedSum/weightsSum
            	break
            }
        }
	}

	gatheredFull=nil
	weightsFull=nil
	winsorizedFull=nil

	return numClippedLow, numClippedHigh
}


// Stacking with linear regression fit. Values which are more than sigmaLow/sigmaHigh
// standard deviations away from linear fit  are excluded from the average calculation.
func StackLinearFit(lightsData [][]float32, RefFrameLoc, sigmaLow, sigmaHigh float32, res []float32) (clipLow, clipHigh int32) {
	gatheredFull:=make([]float32,len(lightsData))
	xs:=make([]float32,len(lightsData))
	for i, _:=range(xs) {
		xs[i]=float32(i)
	}
	numClippedLow, numClippedHigh:=int32(0), int32(0)

	// for all pixels
	skippedNaNs:=int64(0)
	for i, _:=range lightsData[0] {
		// gather data for this pixel across all lights, skipping NaNs
		numGathered:=0
		for li, _:=range lightsData {
			value:=lightsData[li][i]
			if !math.IsNaN(float64(value)) {
				gatheredFull[numGathered]=value
				numGathered++
			} else {
				skippedNaNs++
			}
		}
		if numGathered==0 {
			// If no valid data points available, replace with overall mean.
			// This is subobptimal, but NaN would break subsequent processing,
			// unless all operations are made NaN-proof. As IEEE NaN does not
			// compare equal to itself, this would require a full reimplementation
			// of basic partitioning and sorting primitives on float32. 
			// Not going down that rabbit hole for now. 
			res[i]=RefFrameLoc 
			continue	
		}
		gatheredCur:=gatheredFull[:numGathered]

		// reject outliers until none left
		mean:=float32(0)
		for {
			// sort the data
			qsort.QSortFloat32(gatheredCur)

			// calculate linear fit
			var slope, intercept float32
			slope, intercept, _, _, mean, _=stats.LinearRegression(xs[:len(gatheredCur)], gatheredCur)

			// calculate average distance from prediction
			sigma:=float32(0)
			for i, g:=range gatheredCur {
				lin:=float32(i)*slope+intercept
				diff:=g-lin
				sigma+=float32(math.Abs(float64(diff)))
				//sigma+=diff*diff
			}
			sigma/=float32(len(gatheredCur))
			//sigma=float32(math.Sqrt(float64(sigma)))

			// reject outliers
			left     :=0
			lowBound :=sigmaLow *sigma
			highBound:=sigmaHigh*sigma
			for i,g:=range gatheredCur {
				lin:=float32(i)*slope+intercept
				if lin-g>lowBound {
					gatheredCur[i]=gatheredCur[left]  // reject
					left+=1
					numClippedLow++
				} else if g-lin>highBound {
					gatheredCur[i]=gatheredCur[left]  // reject
					left+=1
					numClippedHigh++
				}				
			}

			if left==0 || len(gatheredCur)<3{
            	break
            }
			gatheredCur=gatheredCur[left:]
		}
		res[i]=mean
	}

	gatheredFull=nil
	xs=nil

	return numClippedLow, numClippedHigh
}


// Incrementally stacks the light onto the given stack, weighted by the given weight. 
// Creates a new stack with same dimensions as light if stack is nil. 
// Returns the modified or created stack. Does not calculate statistics, run star detections etc.
func StackIncremental(stack, light *fits.Image, weight float32) *fits.Image {
	if stack==nil {
		stack=fits.NewImageFromImage(light)
		for i,d:=range light.Data {
			stack.Data[i]=d*weight
		}
	} else {
		stack.Exposure+=light.Exposure
		for i,d:=range light.Data {
			stack.Data[i]+=d*weight
		}
	}
	return stack
}

// Finalizes an incremental stack. Divides pixel values by weight sum, and calculates extended stats
func StackIncrementalFinalize(stack *fits.Image, weightSum float32) {
	factor:=1.0/weightSum
	for i,d:=range stack.Data { stack.Data[i]=d*factor }
	stack.Stats=stats.NewStats(stack.Data, stack.Naxisn[0])
}