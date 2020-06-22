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
	"math"
	"runtime"
	"sync"
)

type StackMode int

const (
	StMedian StackMode = iota
	StMean 
	StSigma
	StWinsorSigma
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


// Stack a set of light frames. Limits parallelism to the number of available cores
func Stack(lights []*FITSImage, mode StackMode, weights []float32, refMedian, sigmaLow, sigmaHigh float32) (result *FITSImage, numClippedLow, numClippedHigh int32, err error) {
	// validate stacking modes and perform automatic mode selection if necesssary
	if mode<StMedian || mode>StAuto {
		return nil, -1, -1, errors.New("invalid stacking mode")
	}
	if mode==StAuto { 
		mode=autoSelectStackingMode(len(lights))
		LogPrintf("Auto-selected stacking mode %d based on %d frames\n", mode, len(lights))
	}

	// create return value array
	data:=GetArrayOfFloat32FromPool(len(lights[0].Data))

	// split into 8 MB work packages, no fewer than 8*NumCPU()
	numBatches:=4*len(lights)*len(lights[0].Data)/(8192*1024)
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

			// subslice lightsData elements for given batch
			ldBatch:=make([][]float32, len(lights))
			for i, l:=range lights { ldBatch[i]=l.Data[lower:upper] }

			// run stacking for the given batch
			switch mode {
			case StMedian:
				StackMedian(ldBatch, refMedian, data[lower:upper])

			case StMean: 
				if weights==nil {
					StackMean(ldBatch, refMedian, data[lower:upper])
				} else {
					StackMeanWeighted(ldBatch, weights, refMedian, data[lower:upper])
				}

			case StSigma:
				var clipLow, clipHigh int32
				if weights==nil {
					clipLow, clipHigh=StackSigma(ldBatch, refMedian, sigmaLow, sigmaHigh, data[lower:upper])
				} else {
					clipLow, clipHigh=StackSigmaWeighted(ldBatch, weights, refMedian, sigmaLow, sigmaHigh, data[lower:upper])
				}
				numClippedLock.Lock()
				numClippedLow+=clipLow
				numClippedHigh+=clipHigh
				numClippedLock.Unlock()

			case StWinsorSigma:
				var clipLow, clipHigh int32
				if weights==nil {
					clipLow, clipHigh=StackWinsorSigma(ldBatch, refMedian, sigmaLow, sigmaHigh, data[lower:upper])
				} else {
					clipLow, clipHigh=StackWinsorSigmaWeighted(ldBatch, weights, refMedian, sigmaLow, sigmaHigh, data[lower:upper])
				}
				numClippedLock.Lock()
				numClippedLow+=clipLow
				numClippedHigh+=clipHigh
				numClippedLock.Unlock()

			case StLinearFit:
				clipLow, clipHigh:=StackLinearFit(ldBatch, refMedian, sigmaLow, sigmaHigh, data[lower:upper])
				numClippedLock.Lock()
				numClippedLow+=clipLow
				numClippedHigh+=clipHigh
				numClippedLock.Unlock()
			} 

			// display progress indicator
			progressLock.Lock()
			progress+=float32(batchSize)/float32(len(data))
			LogPrintf("\r%d%%", int(progress*100))
			progressLock.Unlock()

		}(lower, upper)
	}
	for i:=0; i<cap(sem); i++ {  // wait for goroutines to finish
		sem <- true
	}
	LogPrint("\r")

	// report back on clipping for modes that apply clipping
	if mode>=StSigma {
		LogPrintf("Clipped low %d (%.2f%%) high %d (%.2f%%)\n", 
			numClippedLow,  float32(numClippedLow )*100.0/(float32(len(data)*len(lights))),
			numClippedHigh, float32(numClippedHigh)*100.0/(float32(len(data)*len(lights))) )
	}

	// Assemble into in-memory FITS
	stack:=FITSImage{
		Header: NewFITSHeader(),
		Bitpix: -32,
		Bzero : 0,
		Naxisn: append([]int32(nil), lights[0].Naxisn...), // clone slice
		Pixels: lights[0].Pixels,
		Data  : data,
		Stats : nil, 
		Trans : IdentityTransform2D(),
		Residual: 0,
	}

	stack.Stats, err=CalcExtendedStats(data, lights[0].Naxisn[0])
	if err!=nil { return nil, -1, -1, err }

	if mode>=StSigma {
		return &stack, numClippedLow, numClippedHigh, nil
	}
	return &stack, -1, -1, nil
}


// Stacking with median function
func StackMedian(lightsData [][]float32, refMedian float32, res []float32) {
	gatheredFull:=GetArrayOfFloat32FromPool(len(lightsData))

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
			res[i]=refMedian 
			continue	
		}
		gatheredCur:=gatheredFull[:numGathered]

		res[i]=QSelectMedianFloat32(gatheredCur)
	}
	PutArrayOfFloat32IntoPool(gatheredFull)
	gatheredFull=nil
}


// Stacking with mean function
func StackMean(lightsData [][]float32, refMedian float32, res []float32) {
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
			res[i]=refMedian 
			continue	
		}
		res[i]=sum/float32(numGathered)
	}
}


// Stacking with mean function and weights
func StackMeanWeighted(lightsData [][]float32, weights []float32, refMedian float32, res []float32) {
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
			res[i]=refMedian 
			continue	
		}
		res[i]=sum/float32(weightSum)
	}
}


// Mean stacking with sigma clipping. Values which are more than sigmaLow/sigmaHigh
// standard deviations away from the mean are excluded from the average calculation.
// The standard deviation is calculated w.r.t the mean for robustness.
func StackSigma(lightsData [][]float32, refMedian, sigmaLow, sigmaHigh float32, res []float32) (clipLow, clipHigh int32) {
	gatheredFull:=GetArrayOfFloat32FromPool(len(lightsData))
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
			res[i]=refMedian 
			continue	
		}
		gatheredCur:=gatheredFull[:numGathered]

		// repeat until results for this pixelare stable
		for {

			// calculate median, mean, standard deviation and variance across gathered data
			median:=QSelectMedianFloat32(gatheredCur)
			mean, stdDev:=MeanStdDev(gatheredCur)

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

	PutArrayOfFloat32IntoPool(gatheredFull)
	gatheredFull=nil
	return numClippedLow, numClippedHigh
}


// Weighted mean stacking with sigma clipping. Values which are more than sigmaLow/sigmaHigh
// standard deviations away from the mean are excluded from the average calculation.
// The standard deviation is calculated w.r.t the mean for robustness.
func StackSigmaWeighted(lightsData [][]float32, weights []float32, refMedian, sigmaLow, sigmaHigh float32, res []float32) (clipLow, clipHigh int32) {
	gatheredFull:=GetArrayOfFloat32FromPool(len(lightsData))
	weightsFull :=GetArrayOfFloat32FromPool(len(weights))
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
			res[i]=refMedian 
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
			median:=QSelectMedianFloat32(gatheredCur)
			_, stdDev:=MeanStdDev(gatheredCur)

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

	PutArrayOfFloat32IntoPool(gatheredFull)
	gatheredFull=nil
	PutArrayOfFloat32IntoPool(weightsFull)
	weightsFull=nil

	return numClippedLow, numClippedHigh
}


// Weighted mean stacking with sigma clipping. Values which are more than sigmaLow/sigmaHigh
// standard deviations away from the mean are replaced with the lowest/highest valid value.
func StackWinsorSigma(lightsData [][]float32, refMedian, sigmaLow, sigmaHigh float32, res []float32) (clipLow, clipHigh int32) {
	gatheredFull  :=GetArrayOfFloat32FromPool(len(lightsData))
	winsorizedFull:=GetArrayOfFloat32FromPool(len(lightsData))
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
			res[i]=refMedian 
			continue	
		}
		gatheredCur:=gatheredFull[:numGathered]

		// repeat until results for this pixel are stable
		for {
			// calculate median and standard deviation across all frames
			median:=QSelectMedianFloat32(gatheredCur)
			mean, stdDev:=MeanStdDev(gatheredCur)

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
				_, stdDev=MeanStdDev(winsorized) // also keep original mean
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

	PutArrayOfFloat32IntoPool(gatheredFull)
	gatheredFull=nil
	PutArrayOfFloat32IntoPool(winsorizedFull)
	winsorizedFull=nil

	return numClippedLow, numClippedHigh
}


// Weighted mean stacking with sigma clipping. Values which are more than sigmaLow/sigmaHigh
// standard deviations away from the mean are replaced with the lowest/highest valid value.
func StackWinsorSigmaWeighted(lightsData [][]float32, weights []float32, refMedian, sigmaLow, sigmaHigh float32, res []float32) (clipLow, clipHigh int32) {
	gatheredFull  :=GetArrayOfFloat32FromPool(len(lightsData))
	weightsFull   :=GetArrayOfFloat32FromPool(len(weights))
	winsorizedFull:=GetArrayOfFloat32FromPool(len(lightsData))
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
			res[i]=refMedian 
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
			median:=QSelectMedianFloat32(gatheredCur)
			_, stdDev:=MeanStdDev(gatheredCur)

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
				_, stdDev=MeanStdDev(winsorized) // also keep original mean
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

	PutArrayOfFloat32IntoPool(gatheredFull)
	gatheredFull=nil
	PutArrayOfFloat32IntoPool(weightsFull)
	weightsFull=nil
	PutArrayOfFloat32IntoPool(winsorizedFull)
	winsorizedFull=nil

	return numClippedLow, numClippedHigh
}


// Stacking with linear regression fit. Values which are more than sigmaLow/sigmaHigh
// standard deviations away from linear fit  are excluded from the average calculation.
func StackLinearFit(lightsData [][]float32, refMedian, sigmaLow, sigmaHigh float32, res []float32) (clipLow, clipHigh int32) {
	gatheredFull:=GetArrayOfFloat32FromPool(len(lightsData))
	xs:=GetArrayOfFloat32FromPool(len(lightsData))
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
			res[i]=refMedian 
			continue	
		}
		gatheredCur:=gatheredFull[:numGathered]

		// reject outliers until none left
		mean:=float32(0)
		for {
			// sort the data
			QSortFloat32(gatheredCur)

			// calculate linear fit
			var slope, intercept float32
			slope, intercept, _, _, mean, _=LinearRegression(xs[:len(gatheredCur)], gatheredCur)

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

	PutArrayOfFloat32IntoPool(gatheredFull)
	gatheredFull=nil
	PutArrayOfFloat32IntoPool(xs)
	xs=nil

	return numClippedLow, numClippedHigh
}
