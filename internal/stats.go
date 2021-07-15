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
	"fmt"
	"math"
	"github.com/valyala/fastrand"
	//"time"
)

// Basic statistics on data arrays
type BasicStats struct {
	Min    float32  // Minimum
	Max    float32  // Maximum
	Mean   float32  // Mean (average)
	StdDev float32  // Standard deviation (norm 2, sigma)

	Location float32 // Selected location indicator (standard: randomized sigma-clipped median using randomized Qn)
	Scale    float32 // Selected scale indicator (standard: randomized Qn)

	Noise  float32  // Noise estimation, not calculated by default (expensive)
}


// Enumerated type for location and scale estimator modes
type LSEstimatorMode int
const (
	LSEMeanStdDev LSEstimatorMode = iota
	LSEMedianMAD 
	LSEIKSS
	LSESCMedianQn
	LSEHistogram
)

// Global mode selection for location and scale estimation
var LSEstimator LSEstimatorMode = LSESCMedianQn


// Pretty print basic stats to string
func (s *BasicStats) String() string {
	return fmt.Sprintf("Min %.6g Max %.6g Mean %.6g StdDev %.6g Location %.6g Scale %.6g Noise %.4g", 
	                 	s.Min, s.Max,   s.Mean,   s.StdDev,   s.Location,   s.Scale,   s.Noise)
}


// Pretty print basic stats to CSV header
func (s *BasicStats) ToCSVHeader() string {
	return fmt.Sprintf("Min,Max,Mean,StdDev,Location,Scale,Noise")
}

// Pretty print basic stats to CSV line item 
func (s *BasicStats) ToCSVLine() string {
	return fmt.Sprintf("%.6g,%.6g,%.6g,%.6g,%.6g,%.6g,%.4g", 
		s.Min, s.Max, s.Mean, s.StdDev, s.Location, s.Scale, s.Noise)
}


// Calculate basic statistics for a data array. 
func CalcBasicStats(data []float32) (s *BasicStats) {
	s=&BasicStats{}
	s.Min, s.Mean, s.Max=calcMinMeanMax(data)

	variance:=calcVariance(data, s.Mean)
	s.StdDev=float32(math.Sqrt(float64(variance)))

	return s
}


// Calculates extended statistics and stores in f.Stats 
func CalcExtendedStats(data []float32, width int32) (s *BasicStats, err error) {
	s=CalcBasicStats(data)
	numSamples:=128*1024

	switch LSEstimator {
	case LSEMeanStdDev:
		s.Location, s.Scale=s.Mean, s.StdDev
	case LSEMedianMAD:
		samples:=make([]float32, numSamples)
		s.Location=FastApproxMedian(data, samples)
		s.Scale   =FastApproxMAD(data, s.Location, samples)
		samples=nil
	case LSEIKSS:
		s.Location, s.Scale=IKSS(data, 1e-6, float32(math.Pow(2,-23)))
	case LSESCMedianQn:
		s.Location,   s.Scale=FastApproxSigmaClippedMedianAndQn(data, 2, 2, (s.Max-s.Min)/(65535.0), numSamples)
	case LSEHistogram:
		s.Location, s.Scale=HistogramScaleLoc(data, s.Min, s.Max, 4096)
	}

	s.Noise=EstimateNoise(data, width)

	return s, nil
}	


func MeanStdDev(xs []float32) (mean, stdDev float32) {
	// calculate base statistics for xs
	xmean:=float32(0)
	for _,x:=range(xs) { xmean+=x }
	xmean/=float32(len(xs))
	xvar:=float32(0)
	for _,x:=range(xs) { diff:=x-xmean; xvar+=diff*diff }
	xvar/=float32(len(xs))
	xstddev:=float32(math.Sqrt(float64(xvar)))
	return xmean, xstddev	
}

// Calculate minimum, mean and maximum of given data. Pure go implementation
func calcMinMeanMaxPureGo(data []float32) (min, mean, max float32) {
	mmin, mmean, mmax:=float32(data[0]), float64(0), float32(data[0])
	for _,v := range data {
		mv:=float32(v)
		if mv<mmin { 
			mmin=mv
			//s.NumMin=1 
		} else if mv==mmin { 
			//s.NumMin++ 
		}
		if mv>mmax {
			mmax=mv
			//s.NumMax=1
		} else if mv==mmax {
			//s.NumMax++
		}
		mmean+=float64(mv)
	}
	return mmin, float32(mmean/float64(len(data))), mmax
}


// Calculate variance of given data from provided mean. Pure go implementation
func calcVariancePureGo(data []float32, mean float32) (result float64) {
	variance:=float64(0)
	for _,v :=range data {
		diff:=float64(v-mean)
		variance+=diff*diff
	}
	return variance/float64(len(data))
}


// Returns the sigma clipped median of the data. Does not change the data.
func SigmaClippedMedianAndMAD(data []float32, sigmaLow, sigmaHigh float32) (median, mad float32) {
	tmp:=make([]float32,len(data))
	copy(tmp, data)
	remaining:=tmp
	for {
		median:=QSelectMedianFloat32(remaining) // reorders, doesnt matter

		// calculate std deviation w.r.t. median
		stdDev:=float32(0)
		for _,r:=range remaining {
			diff  :=r-median
			stdDev+=diff*diff
		}
		stdDev/=float32(len(remaining))
		stdDev =float32(math.Sqrt(float64(stdDev)))*1.134

        // reject outliers based on sigma
		lowBound :=median - sigmaLow *stdDev
		highBound:=median + sigmaHigh*stdDev
		kept :=0
		for i:=0; i<len(remaining); i++ {
			r:=remaining[i]
			if r>=lowBound && r<=highBound {
				remaining[kept]=r
				kept++
			}
		}
		rejected:=len(remaining)-kept
		remaining=remaining[:kept]

		// once converged, return results
		if rejected==0 || len(remaining)<=3 {
			for i, d:=range data {
				tmp[i]=float32(math.Abs(float64(d-median)))
			}
			mad=QSelectMedianFloat32(tmp)*1.4826

			tmp, remaining=nil, nil

			return median, mad
		}
	}
}


// Calculates fast approximate median of the (presumably large) data by subsampling the given number of values and taking the median of that. 
// Uses provided samples array as scratchpad
func FastApproxMedian(data []float32, samples []float32) float32 {
	max:=uint32(len(data))
	rng:=fastrand.RNG{}
	for i,_:=range samples {
		index:=rng.Uint32n(max)
		samples[i]=data[index]
	}
	median:=QSelectMedianFloat32(samples)
	return median
}

// Calculates fast approximate median of the (presumably large) data by subsampling the given number of values and taking the median of that. 
// Uses provided samples array as scratchpad
func FastApproxBoundedMedian(data []float32, lowBound, highBound float32, samples []float32) float32 {
	max:=uint32(len(data))
	rng:=fastrand.RNG{}
	for i,_:=range samples {
		var d float32
		for {
			d=data[rng.Uint32n(max)]
			if d>=lowBound && d<=highBound { break }
		}
		samples[i]=d
	}
	median:=QSelectMedianFloat32(samples)
	return median
}



// Calculates fast approximate median of the (presumably large) data by subsampling the given number of values and taking the median of that. 
func FastApproxStdDev(data []float32, location float32, numSamples int) float32 {
	max:=uint32(len(data))
	rng:=fastrand.RNG{}
	sumSqDiff:=float32(0)
	for i:=0; i<numSamples; i++ {
		index:=rng.Uint32n(max)
		diff:=data[index]-location
		sumSqDiff+=diff*diff
	}
	variance:=sumSqDiff/float32(numSamples)
	return float32(math.Sqrt(float64(variance)))
}


// Calculates fast approximate median of the (presumably large) data by subsampling the given number of values and taking the median of that. 
func FastApproxBoundedStdDev(data []float32, location float32, lowBound, highBound float32, numSamples int) float32 {
	max:=uint32(len(data))
	rng:=fastrand.RNG{}
	sumSqDiff:=float32(0)
	for i:=0; i<numSamples; i++ {
		var d float32
		for {
			d=data[rng.Uint32n(max)]
			if d>=lowBound && d<=highBound { break }
		}
		diff:=d-location
		sumSqDiff+=diff*diff
	}
	variance:=sumSqDiff/float32(numSamples)
	return float32(math.Sqrt(float64(variance)))
}


// Calculates fast approximate median of absolute differences of the (presumably large) data by subsampling the given number of values and taking the MAD of that. 
func FastApproxMAD(data []float32, location float32, samples []float32) float32 {
	max:=uint32(len(data))
	rng:=fastrand.RNG{}
	for i,_:=range samples {
		index:=rng.Uint32n(max)
		samples[i]=float32(math.Abs(float64(data[index]-location)))
	}
	mad:=QSelectMedianFloat32(samples)*1.4826  // normalize to Gaussian std dev.
	return mad
}

// Calculates fast approximate median of absolute differences of the (presumably large) data by subsampling the given number of values and taking the MAD of that. 
func FastApproxBoundedMAD(data []float32, location float32, lowBound, highBound float32, numSamples int) float32 {
	samples:=make([]float32,numSamples)
	max:=uint32(len(data))
	rng:=fastrand.RNG{}
	for i,_:=range samples {
		var d float32
		for {
			d=data[rng.Uint32n(max)]
			if d>=lowBound && d<=highBound { break }
		}
		samples[i]=float32(math.Abs(float64(d-location)))
	}
	mad:=QSelectMedianFloat32(samples)*1.4826  // normalize to Gaussian std dev.
	samples=nil
	return mad
}


// Calculates fast approximate Qn scale estimate of the (presumably large) data by subsampling the given number of pairs and taking the first quartile of that. 
// Original paper http://web.ipac.caltech.edu/staff/fmasci/home/astro_refs/BetterThanMAD.pdf
// Original n*log n implementation technical report https://www.researchgate.net/profile/Christophe_Croux/publication/228595593_Time-Efficient_Algorithms_for_Two_Highly_Robust_Estimators_of_Scale/links/09e4150f52c2fcabb0000000/Time-Efficient-Algorithms-for-Two-Highly-Robust-Estimators-of-Scale.pdf
// Sampling approach appears to be mine
func FastApproxQn(data []float32, samples []float32) float32 {
	max:=uint32(len(data))
	rng:=fastrand.RNG{}
	for i,_:=range samples {
		index1:=1+rng.Uint32n(max-1)
		index2:=rng.Uint32n(index1)
		samples[i]=float32(math.Abs(float64(data[index1]-data[index2])))
	}
	qn:=QSelectFirstQuartileFloat32(samples)*2.21914  // normalize to Gaussian std dev, for large numSamples >>1000. 
	// Original paper had wrong constant, source for constant https://rdrr.io/cran/robustbase/man/Qn.html
	return qn
}


// Calculates fast approximate Qn scale estimate of the (presumably large) data by subsampling the given number of pairs and taking the first quartile of that. 
func FastApproxBoundedQn(data []float32, lowBound, highBound float32, samples []float32) float32 {
	max:=uint32(len(data))
	rng:=fastrand.RNG{}
	for i,_:=range samples {
		var d1, d2 float32
		for {
			index1:=1+rng.Uint32n(max-1)
			d1=data[index1]
			if d1< lowBound || d1> highBound { continue }
			d2=data[rng.Uint32n(index1)]
			if d2>=lowBound && d2<=highBound { break    }
		}
		samples[i]=float32(math.Abs(float64(d1-d2)))
	}
	qn:=QSelectFirstQuartileFloat32(samples)*2.21914  // normalize to Gaussian std dev, for large numSamples >>1000
	// Original paper had wrong constant, source for constant https://rdrr.io/cran/robustbase/man/Qn.html
	samples=nil
	return qn
}



// Returns a rapid robust estimation of location and scale. Uses a fast approximate median based on randomized sampling,
// iteratively sigma clipped with a fast approximate Qn based on random sampling. Exits once the absolute change in 
// location and scale is below epsilon.
func FastApproxSigmaClippedMedianAndQn(data []float32, sigmaLow, sigmaHigh float32, epsilon float32, numSamples int) (location, scale float32) {
	samples:=make([]float32, numSamples)
	location=FastApproxMedian(data, samples) // sampling
	scale   =FastApproxQn    (data, samples) // sampling

	for i:=0; ; i++ {
		lowBound :=location - sigmaLow*scale
		highBound:=location + sigmaLow*scale

		newLocation:=FastApproxBoundedMedian(data, lowBound, highBound, samples) // sampling
		newScale   :=FastApproxBoundedQn    (data, lowBound, highBound, samples) // sampling
		newScale   *=1.134                                    // adjust for subsequent clipping

		// once converged, return results
		if float32(math.Abs(float64(newLocation-location))+math.Abs(float64(newScale-scale)))<=epsilon || i>=10 {
			scale=FastApproxQn(data, samples) // sampling
			samples=nil
			return location, scale
		}

		location, scale = newLocation, newScale
	}
}


// Returns the biweight midvariance of the xs. tmp must be same length as xs, is used as temp storage
func bwmv(xs []float32, median float32, tmp []float32) float32 {
	mads:=tmp[:len(xs)]
	for i,x:=range xs {
		mads[i]=float32(math.Abs(float64(x-median)))
	}
	mad:=QSelectMedianFloat32(mads)
	mads=nil

	ys:=tmp[:len(xs)]
	for i,x:=range xs {
		ys[i]=(x-median)/(9*mad)
	}

	numSum, denomSum:=float32(0), float32(0)
	for i,x:=range xs {
		y:=ys[i]
		a:=float32(0)
		if y>-1 && y<1 { a=float32(1) }

		xMinusM:=x-median
		oneMinusYSquared:=1-y*y
		oneMinusYSquaredSquared:=oneMinusYSquared*oneMinusYSquared
		numSum+=a*xMinusM*xMinusM*oneMinusYSquaredSquared*oneMinusYSquaredSquared

		oneMinus5YSquared:=1-5*y*y
		denomSum+=a*oneMinusYSquared*oneMinus5YSquared
	}
	return float32(len(xs))*numSum/(denomSum*denomSum)
}


// Returns the iterative k-sigma estimators of locations and scale
func IKSS(data []float32, epsilon float32, e float32) (location, scale float32) {
   	xs :=make([]float32,len(data))
   	copy(xs, data)
   	QSortFloat32(xs)

   	tmp:=make([]float32,len(data))

   	i, j:=0, len(xs)
   	s0:=float32(1)
   	for {
   		if j-i<1              { return 0, 0 }
   		m:=xs[(i+j)>>1]       // median is easy as xs are sorted
   		s:=float32(math.Sqrt(float64(bwmv(xs[i:j], m, tmp))))
   		if s<epsilon          { return m, 0 }
   		if s0-s < s*epsilon   { return m, 0.991*s }
   		s0=s
   		xlow :=m-4*s
   		xhigh:=m+4*s
   		for xs[i]<xlow {
   			i++
   		}
   		for xs[j-1]>xhigh {
   			j--
   		}
   	}
}


// Calculate linear regression for xs and ys
func LinearRegression(xs, ys []float32) (slope, intercept, xmean, xstddev, ymean, ystddev float32) {
	xmean, xstddev=MeanStdDev(xs)
	ymean, ystddev=MeanStdDev(ys)

	// calculate correlation betwen the xs and ys
	corr:=float32(0)
	for i,_:=range(xs) { diff:=(xs[i]-xmean)*(ys[i]-ymean); corr+=diff }
	corr/=xstddev*ystddev*(float32(len(xs))+1)

	// calculate the regression line
	slope=corr*ystddev/xstddev
	intercept=ymean-slope*xmean

	return slope, intercept, xmean, xstddev, ymean, ystddev
}



// Returns the half sample mode of the data, an estimator for the mode. Does not modify data. 
// Turned out not to be that robust after all, not using this.
// From Bickel and Fruehwirth 2006, https://arxiv.org/ftp/math/papers/0505/0505419.pdf
func HalfSampleMode(data []float32) float32 {
	tmp:=make([]float32,len(data))
	copy(tmp, data)
	QSortFloat32(tmp)
	hsm:=HalfSampleModeSorted(tmp)
	tmp=nil
	return hsm
}


// Returns the half sample mode of the data, an estimator for the mode. Does not modify data. 
// Prerequisite: data is sorted. Turned out not to be that robust after all, not using this.
// From Bickel and Fruehwirth 2006, https://arxiv.org/ftp/math/papers/0505/0505419.pdf
func HalfSampleModeSorted(data []float32) float32 {
	if len(data)==1 {
		return data[0]
	} else if len(data)==2 {
		return 0.5*(data[0]+data[1])
	} else if len(data)==3 {
		widthDiff:=(data[1]-data[0]) - (data[2]-data[1])
		if widthDiff<0 {
			return 0.5*(data[1]-data[0])
		} else if widthDiff>0 {
			return 0.5*(data[2]-data[1])
		} else {
			return data[1]
		}
	} else {
		halfLen:=len(data)/2
		minIndices:=[]int{}
		minIndex, minWidth:=-1, float32(math.MaxFloat32)
		for i:=0; i<len(data)-halfLen+1; i++ {
			width:=data[i+halfLen-1]-data[i]
			if width<minWidth {
				minIndex, minWidth = i, width
				minIndices=minIndices[:0]
			} else {
				minIndices=append(minIndices,i)
			}
		}
		if len(minIndices)>0 {
			mi:=minIndices[len(minIndices)/2]
			return HalfSampleModeSorted(data[mi:mi+halfLen])
		} else {
			return HalfSampleModeSorted(data[minIndex:minIndex+halfLen])
		}
	}
}

// Returns greyscale location and scale for given RGB image
func RGBGreyLocScale(data []float32, width int32) (loc, scale float32, err error) {
	l:=len(data)/3
	rStats,err:=CalcExtendedStats(data[0*l:1*l], width)
   	if err!=nil { return 0,0, err }
	gStats,err:=CalcExtendedStats(data[1*l:2*l], width)
   	if err!=nil { return 0,0, err }
	bStats,err:=CalcExtendedStats(data[2*l:3*l], width)
   	if err!=nil { return 0,0, err }
	loc  =0.299*rStats.Location +0.587*gStats.Location +0.114*bStats.Location
	scale=0.299*rStats.Scale    +0.587*gStats.Scale    +0.114*bStats.Scale
	return loc, scale, nil
}


// Returns greyscale location and scale for given HCL image
func HCLLumMinMaxLocScale(data []float32, width int32) (min, max, loc, scale float32, err error) {
	l:=len(data)/3
	lumStats,err:=CalcExtendedStats(data[2*l:3*l], width)
   	if err!=nil { return 0,0,0,0, err }
	return lumStats.Min, lumStats.Max, lumStats.Location, lumStats.Scale,  nil
}

// Calculate scale and location based on histogram
func HistogramScaleLoc(data []float32, min, max float32, numBins uint32) (loc, scale float32) {
	// deal with edge case
	if min==max { return min, 0 }

	// calculate histogram
	LogPrintf("calculating %d bin histogram for %d data points in [%.2f%% .. %.2f%%]\n", numBins, len(data), min*100, max*100)

	bins:=make([]uint32, numBins)
	valueToBin:=float32(numBins-1)/(max-min)
	for _, d:=range(data) {
		bin:=uint32( ((d-min)*valueToBin) + 0.5 )
        bins[bin]++
	} 

	// find inner peak (avoid edges which may be distorted by clipping)
	peakBin, peakCount:=uint32(0), uint32(0)
    for bin, count:=range(bins[1:numBins-1]) {
    	if count>peakCount {
    		peakBin, peakCount=uint32(bin+1), count
    	}
    }
	loc=min+float32(peakBin)/valueToBin
	LogPrintf("histogram peak: bin[%d] = %d (%.2f%%) -> value %.2f%%\n", peakBin, peakCount, 100*float32(peakCount)/float32(len(data)), loc*100)

	// Find standard deviation around the histogram peak by cumulating adjacent bins until one sigma threshold of 68.27% is reached
	// See https://en.wikipedia.org/wiki/68%E2%80%9395%E2%80%9399.7_rule
	sigmaThreshold:=uint32(float32(len(data))*0.6827)
	intervalLimit:=peakBin
	if numBins-1-peakBin<intervalLimit { 
		intervalLimit=numBins-1-peakBin 
	}
	cum:=peakCount
	scale=0.5*float32(1.0)/valueToBin
	i:=uint32(0)

	if cum<sigmaThreshold {
		for i=1; i<=intervalLimit; i++ {
			cum=cum+bins[peakBin-i]+bins[peakBin+i]
			scale=0.5*float32(2*i+1)/valueToBin
			if cum>=sigmaThreshold { 
				break 
			}
		}
	}
	LogPrintf("bins[%d +/-%d]=%d vs threshold %d; scale %.2f%%\n", peakBin,i, cum, sigmaThreshold, scale*100)
	return loc, scale
}

