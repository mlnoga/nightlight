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

package stats

import (
	"fmt"
	"math"
	"strings"

	"github.com/mlnoga/nightlight/internal/qsort"
	"github.com/valyala/fastrand"
	//"time"
)

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
// FIXME: Need to still remove this global
var LSEstimator LSEstimatorMode = LSESCMedianQn

// Statistics on data arrays, calculated on demand
type Stats struct {
	data  []float32 // The underlying data array
	width int32     // Width of a line in the underlying data array (for noise)

	min      float32 // Minimum
	max      float32 // Maximum
	mean     float32 // Mean (average)
	stdDev   float32 // Standard deviation (norm 2, sigma)
	location float32 // Selected location indicator (standard: randomized sigma-clipped median using randomized Qn)
	scale    float32 // Selected scale indicator (standard: randomized Qn)
	noise    float32 // Noise estimation, not calculated by default (expensive)

	haveMMM      bool // Indicates min/mean/max are fresh
	haveStdDev   bool // Indicates std deviation is fresh
	haveLocScale bool // Indicates location and scale are
	haveNoise    bool // Indicates noise is fresh
}

func NewStats(d []float32, w int32) *Stats {
	return &Stats{data: d, width: w}
}

func NewStatsWithMMM(d []float32, w int32, min, max, mean float32) *Stats {
	return &Stats{data: d, width: w, min: min, max: max, mean: mean, haveMMM: true}
}

func NewStatsForChannel(hcl []float32, w int32, ch, numCh int) *Stats {
	if ch >= numCh || ch < 0 || numCh <= 0 {
		panic(fmt.Sprintf("invalid channel %d with %d total channels", ch, numCh))
	}
	chLen := len(hcl) / numCh
	return &Stats{data: hcl[ch*chLen : (ch+1)*chLen], width: w}
}

func (s *Stats) FreeData() {
	s.data = nil
}

func (s *Stats) SetData(d []float32) {
	s.data = d
	s.Clear()
}

func (s *Stats) Clear() {
	s.haveMMM, s.haveStdDev, s.haveLocScale, s.haveNoise = false, false, false, false
}

func (s *Stats) UpdateCachedWith(multiplier, offset float32) {
	s.min = s.min*multiplier + offset
	s.max = s.max*multiplier + offset
	s.mean = s.mean*multiplier + offset
	s.stdDev = s.stdDev * multiplier
	s.location = s.location*multiplier + offset
	s.scale = s.scale * multiplier
	s.noise = s.noise * multiplier
}

func (s *Stats) Min() float32 {
	if !s.haveMMM {
		if s.data == nil {
			panic("Cannot calculate stats on nil data")
		}
		s.min, s.mean, s.max = calcMinMeanMax(s.data)
		s.haveMMM = true
	}
	return s.min
}

func (s *Stats) Max() float32 {
	if !s.haveMMM {
		if s.data == nil {
			panic("Cannot calculate stats on nil data")
		}
		s.min, s.mean, s.max = calcMinMeanMax(s.data)
		s.haveMMM = true
	}
	return s.max
}

func (s *Stats) Mean() float32 {
	if !s.haveMMM {
		if s.data == nil {
			panic("Cannot calculate stats on nil data")
		}
		s.min, s.mean, s.max = calcMinMeanMax(s.data)
		s.haveMMM = true
	}
	return s.mean
}

func (s *Stats) StdDev() float32 {
	if !s.haveStdDev {
		if s.data == nil {
			panic("Cannot calculate stats on nil data")
		}
		variance := calcVariance(s.data, s.Mean())
		s.stdDev = float32(math.Sqrt(float64(variance)))
		s.haveStdDev = true
	}
	return s.stdDev
}

func (s *Stats) Location() float32 {
	if !s.haveLocScale {
		if s.data == nil {
			panic("Cannot calculate stats on nil data")
		}
		s.updateLocationScale()
	}
	return s.location
}

func (s *Stats) Scale() float32 {
	if !s.haveLocScale {
		if s.data == nil {
			panic("Cannot calculate stats on nil data")
		}
		s.updateLocationScale()
	}
	return s.scale
}

func (s *Stats) Noise() float32 {
	if !s.haveNoise {
		if s.data == nil {
			panic("Cannot calculate stats on nil data")
		}
		s.noise = EstimateNoise(s.data, s.width)
		s.haveNoise = true
	}
	return s.noise
}

// Pretty print Stats stats to string. Lazily prints only values available
func (s *Stats) String() string {
	precision := 6
	if s.haveMMM {
		if m := s.Max(); m >= 1000000 {
			precision = 0
		} else if m >= 100000 {
			precision = 1
		} else if m >= 10000 {
			precision = 2
		} else if m >= 1000 {
			precision = 3
		} else if m > 100 {
			precision = 4
		} else if m > 10 {
			precision = 5
		}
	}
	b := strings.Builder{}
	space := ""
	if s.haveMMM {
		fmt.Fprintf(&b, "Min %.*f Max %.*f Mean %.*f", precision, s.Min(), precision, s.Max(), precision, s.Mean())
		space = " "
	}
	if s.haveStdDev {
		fmt.Fprintf(&b, "%sStdDev %.*f", space, precision, s.StdDev())
		space = " "
	}
	if s.haveLocScale {
		fmt.Fprintf(&b, "%sLocation %.*f Scale %.*f", space, precision, s.Location(), precision, s.Scale())
		space = " "
	}
	if s.haveNoise {
		fmt.Fprintf(&b, "%sNoise %.*f", space, precision, s.Noise())
		space = " "
	}
	if b.Len() == 0 {
		return "(no stats yet)"
	}
	return b.String()
}

func (s *Stats) StringEager() string {
	return fmt.Sprintf("Min %.6g Max %.6g Mean %.6g StdDev %.6g Location %.6g Scale %.6g Noise %.4g",
		s.Min(), s.Max(), s.Mean(), s.StdDev(), s.Location(), s.Scale(), s.Noise())
}

// Calculates extended statistics and stores in f.Stats
func (s *Stats) updateLocationScale() {
	numSamples := 128 * 1024

	switch LSEstimator {
	case LSEMeanStdDev:
		s.location, s.scale = s.Mean(), s.StdDev()
	case LSEMedianMAD:
		samples := make([]float32, numSamples)
		s.location = FastApproxMedian(s.data, samples)
		s.scale = FastApproxMAD(s.data, s.location, samples)
		samples = nil
	case LSEIKSS:
		s.location, s.scale = IKSS(s.data, 1e-6, float32(math.Pow(2, -23)))
	case LSESCMedianQn:
		s.location, s.scale = FastApproxSigmaClippedMedianAndQn(s.data, 2, 2, (s.Max()-s.Min())/(65535.0), numSamples)
	case LSEHistogram:
		s.location, s.scale = HistogramScaleLoc(s.data, s.Min(), s.Max(), 4096)
	}
	s.haveLocScale = true
}

func MeanStdDev(xs []float32) (mean, stdDev float32) {
	// calculate base statistics for xs
	xmean := float32(0)
	for _, x := range xs {
		xmean += x
	}
	xmean /= float32(len(xs))
	xvar := float32(0)
	for _, x := range xs {
		diff := x - xmean
		xvar += diff * diff
	}
	xvar /= float32(len(xs))
	xstddev := float32(math.Sqrt(float64(xvar)))
	return xmean, xstddev
}

// Calculate minimum, mean and maximum of given data. Pure go implementation
func calcMinMeanMaxPureGo(data []float32) (min, mean, max float32) {
	mmin, mmean, mmax := float32(data[0]), float64(0), float32(data[0])
	for _, v := range data {
		mv := float32(v)
		if mv < mmin {
			mmin = mv
		}
		if mv > mmax {
			mmax = mv
		}
		mmean += float64(mv)
	}
	return mmin, float32(mmean / float64(len(data))), mmax
}

// Calculate variance of given data from provided mean. Pure go implementation
func calcVariancePureGo(data []float32, mean float32) (result float64) {
	variance := float64(0)
	for _, v := range data {
		diff := float64(v - mean)
		variance += diff * diff
	}
	return variance / float64(len(data))
}

// Returns the sigma clipped median of the data. Does not change the data.
func SigmaClippedMedianAndMAD(data []float32, sigmaLow, sigmaHigh float32) (median, mad float32) {
	tmp := make([]float32, len(data))
	copy(tmp, data)
	remaining := tmp
	for {
		median := qsort.QSelectMedianFloat32(remaining) // reorders, doesnt matter

		// calculate std deviation w.r.t. median
		stdDev := float32(0)
		for _, r := range remaining {
			diff := r - median
			stdDev += diff * diff
		}
		stdDev /= float32(len(remaining))
		stdDev = float32(math.Sqrt(float64(stdDev))) * 1.134

		// reject outliers based on sigma
		lowBound := median - sigmaLow*stdDev
		highBound := median + sigmaHigh*stdDev
		kept := 0
		for i := 0; i < len(remaining); i++ {
			r := remaining[i]
			if r >= lowBound && r <= highBound {
				remaining[kept] = r
				kept++
			}
		}
		rejected := len(remaining) - kept
		remaining = remaining[:kept]

		// once converged, return results
		if rejected == 0 || len(remaining) <= 3 {
			for i, d := range data {
				tmp[i] = float32(math.Abs(float64(d - median)))
			}
			mad = qsort.QSelectMedianFloat32(tmp) * 1.4826

			tmp, remaining = nil, nil

			return median, mad
		}
	}
}

// Calculates fast approximate median of the (presumably large) data by subsampling the given number of values and taking the median of that.
// Uses provided samples array as scratchpad
func FastApproxMedian(data []float32, samples []float32) float32 {
	max := uint32(len(data))
	rng := fastrand.RNG{}
	for i := range samples {
		index := rng.Uint32n(max)
		samples[i] = data[index]
	}
	median := qsort.QSelectMedianFloat32(samples)
	return median
}

// Calculates fast approximate median of the (presumably large) data by subsampling the given number of values and taking the median of that.
// Uses provided samples array as scratchpad
func FastApproxBoundedMedian(data []float32, lowBound, highBound float32, samples []float32) float32 {
	max := uint32(len(data))
	rng := fastrand.RNG{}
	for i := range samples {
		var d float32
		for {
			d = data[rng.Uint32n(max)]
			if d >= lowBound && d <= highBound {
				break
			}
		}
		samples[i] = d
	}
	median := qsort.QSelectMedianFloat32(samples)
	return median
}

// Calculates fast approximate median of the (presumably large) data by subsampling the given number of values and taking the median of that.
func FastApproxStdDev(data []float32, location float32, numSamples int) float32 {
	max := uint32(len(data))
	rng := fastrand.RNG{}
	sumSqDiff := float32(0)
	for i := 0; i < numSamples; i++ {
		index := rng.Uint32n(max)
		diff := data[index] - location
		sumSqDiff += diff * diff
	}
	variance := sumSqDiff / float32(numSamples)
	return float32(math.Sqrt(float64(variance)))
}

// Calculates fast approximate median of the (presumably large) data by subsampling the given number of values and taking the median of that.
func FastApproxBoundedStdDev(data []float32, location float32, lowBound, highBound float32, numSamples int) float32 {
	max := uint32(len(data))
	rng := fastrand.RNG{}
	sumSqDiff := float32(0)
	for i := 0; i < numSamples; i++ {
		var d float32
		for {
			d = data[rng.Uint32n(max)]
			if d >= lowBound && d <= highBound {
				break
			}
		}
		diff := d - location
		sumSqDiff += diff * diff
	}
	variance := sumSqDiff / float32(numSamples)
	return float32(math.Sqrt(float64(variance)))
}

// Calculates fast approximate median of absolute differences of the (presumably large) data by subsampling the given number of values and taking the MAD of that.
func FastApproxMAD(data []float32, location float32, samples []float32) float32 {
	max := uint32(len(data))
	rng := fastrand.RNG{}
	for i := range samples {
		index := rng.Uint32n(max)
		samples[i] = float32(math.Abs(float64(data[index] - location)))
	}
	mad := qsort.QSelectMedianFloat32(samples) * 1.4826 // normalize to Gaussian std dev.
	return mad
}

// Calculates fast approximate median of absolute differences of the (presumably large) data by subsampling the given number of values and taking the MAD of that.
func FastApproxBoundedMAD(data []float32, location float32, lowBound, highBound float32, numSamples int) float32 {
	samples := make([]float32, numSamples)
	max := uint32(len(data))
	rng := fastrand.RNG{}
	for i := range samples {
		var d float32
		for {
			d = data[rng.Uint32n(max)]
			if d >= lowBound && d <= highBound {
				break
			}
		}
		samples[i] = float32(math.Abs(float64(d - location)))
	}
	mad := qsort.QSelectMedianFloat32(samples) * 1.4826 // normalize to Gaussian std dev.
	samples = nil
	return mad
}

// Calculates fast approximate Qn scale estimate of the (presumably large) data by subsampling the given number of pairs and taking the first quartile of that.
// Original paper http://web.ipac.caltech.edu/staff/fmasci/home/astro_refs/BetterThanMAD.pdf
// Original n*log n implementation technical report https://www.researchgate.net/profile/Christophe_Croux/publication/228595593_Time-Efficient_Algorithms_for_Two_Highly_Robust_Estimators_of_Scale/links/09e4150f52c2fcabb0000000/Time-Efficient-Algorithms-for-Two-Highly-Robust-Estimators-of-Scale.pdf
// Sampling approach appears to be mine
func FastApproxQn(data []float32, samples []float32) float32 {
	max := uint32(len(data))
	rng := fastrand.RNG{}
	for i := range samples {
		index1 := 1 + rng.Uint32n(max-1)
		index2 := rng.Uint32n(index1)
		samples[i] = float32(math.Abs(float64(data[index1] - data[index2])))
	}
	qn := qsort.QSelectFirstQuartileFloat32(samples) * 2.21914 // normalize to Gaussian std dev, for large numSamples >>1000.
	// Original paper had wrong constant, source for constant https://rdrr.io/cran/robustbase/man/Qn.html
	return qn
}

// Calculates fast approximate Qn scale estimate of the (presumably large) data by subsampling the given number of pairs and taking the first quartile of that.
func FastApproxBoundedQn(data []float32, lowBound, highBound float32, samples []float32) float32 {
	max := uint32(len(data))
	rng := fastrand.RNG{}
	for i := range samples {
		var d1, d2 float32
		for {
			index1 := 1 + rng.Uint32n(max-1)
			d1 = data[index1]
			if d1 < lowBound || d1 > highBound {
				continue
			}
			d2 = data[rng.Uint32n(index1)]
			if d2 >= lowBound && d2 <= highBound {
				break
			}
		}
		samples[i] = float32(math.Abs(float64(d1 - d2)))
	}
	qn := qsort.QSelectFirstQuartileFloat32(samples) * 2.21914 // normalize to Gaussian std dev, for large numSamples >>1000
	// Original paper had wrong constant, source for constant https://rdrr.io/cran/robustbase/man/Qn.html
	samples = nil
	return qn
}

// Returns a rapid robust estimation of location and scale. Uses a fast approximate median based on randomized sampling,
// iteratively sigma clipped with a fast approximate Qn based on random sampling. Exits once the absolute change in
// location and scale is below epsilon.
func FastApproxSigmaClippedMedianAndQn(data []float32, sigmaLow, sigmaHigh float32, epsilon float32, numSamples int) (location, scale float32) {
	samples := make([]float32, numSamples)
	location = FastApproxMedian(data, samples) // sampling
	scale = FastApproxQn(data, samples)        // sampling

	for i := 0; ; i++ {
		lowBound := location - sigmaLow*scale
		highBound := location + sigmaLow*scale

		newLocation := FastApproxBoundedMedian(data, lowBound, highBound, samples) // sampling
		newScale := FastApproxBoundedQn(data, lowBound, highBound, samples)        // sampling
		newScale *= 1.134                                                          // adjust for subsequent clipping

		// once converged, return results
		if float32(math.Abs(float64(newLocation-location))+math.Abs(float64(newScale-scale))) <= epsilon || i >= 10 {
			scale = FastApproxQn(data, samples) // sampling
			samples = nil
			return location, scale
		}

		location, scale = newLocation, newScale
	}
}

// Returns the biweight midvariance of the xs. tmp must be same length as xs, is used as temp storage
func bwmv(xs []float32, median float32, tmp []float32) float32 {
	mads := tmp[:len(xs)]
	for i, x := range xs {
		mads[i] = float32(math.Abs(float64(x - median)))
	}
	mad := qsort.QSelectMedianFloat32(mads)
	mads = nil

	ys := tmp[:len(xs)]
	for i, x := range xs {
		ys[i] = (x - median) / (9 * mad)
	}

	numSum, denomSum := float32(0), float32(0)
	for i, x := range xs {
		y := ys[i]
		a := float32(0)
		if y > -1 && y < 1 {
			a = float32(1)
		}

		xMinusM := x - median
		oneMinusYSquared := 1 - y*y
		oneMinusYSquaredSquared := oneMinusYSquared * oneMinusYSquared
		numSum += a * xMinusM * xMinusM * oneMinusYSquaredSquared * oneMinusYSquaredSquared

		oneMinus5YSquared := 1 - 5*y*y
		denomSum += a * oneMinusYSquared * oneMinus5YSquared
	}
	return float32(len(xs)) * numSum / (denomSum * denomSum)
}

// Returns the iterative k-sigma estimators of locations and scale
func IKSS(data []float32, epsilon float32, e float32) (location, scale float32) {
	xs := make([]float32, len(data))
	copy(xs, data)
	qsort.QSortFloat32(xs)

	tmp := make([]float32, len(data))

	i, j := 0, len(xs)
	s0 := float32(1)
	for {
		if j-i < 1 {
			return 0, 0
		}
		m := xs[(i+j)>>1] // median is easy as xs are sorted
		s := float32(math.Sqrt(float64(bwmv(xs[i:j], m, tmp))))
		if s < epsilon {
			return m, 0
		}
		if s0-s < s*epsilon {
			return m, 0.991 * s
		}
		s0 = s
		xlow := m - 4*s
		xhigh := m + 4*s
		for xs[i] < xlow {
			i++
		}
		for xs[j-1] > xhigh {
			j--
		}
	}
}

// Calculate linear regression for xs and ys
func LinearRegression(xs, ys []float32) (slope, intercept, xmean, xstddev, ymean, ystddev float32) {
	xmean, xstddev = MeanStdDev(xs)
	ymean, ystddev = MeanStdDev(ys)

	// calculate correlation betwen the xs and ys
	corr := float32(0)
	for i := range xs {
		diff := (xs[i] - xmean) * (ys[i] - ymean)
		corr += diff
	}
	corr /= xstddev * ystddev * (float32(len(xs)) + 1)

	// calculate the regression line
	slope = corr * ystddev / xstddev
	intercept = ymean - slope*xmean

	return slope, intercept, xmean, xstddev, ymean, ystddev
}

// Returns the half sample mode of the data, an estimator for the mode. Does not modify data.
// Turned out not to be that robust after all, not using this.
// From Bickel and Fruehwirth 2006, https://arxiv.org/ftp/math/papers/0505/0505419.pdf
func HalfSampleMode(data []float32) float32 {
	tmp := make([]float32, len(data))
	copy(tmp, data)
	qsort.QSortFloat32(tmp)
	hsm := HalfSampleModeSorted(tmp)
	tmp = nil
	return hsm
}

// Returns the half sample mode of the data, an estimator for the mode. Does not modify data.
// Prerequisite: data is sorted. Turned out not to be that robust after all, not using this.
// From Bickel and Fruehwirth 2006, https://arxiv.org/ftp/math/papers/0505/0505419.pdf
func HalfSampleModeSorted(data []float32) float32 {
	if len(data) == 1 {
		return data[0]
	} else if len(data) == 2 {
		return 0.5 * (data[0] + data[1])
	} else if len(data) == 3 {
		widthDiff := (data[1] - data[0]) - (data[2] - data[1])
		if widthDiff < 0 {
			return 0.5 * (data[1] - data[0])
		} else if widthDiff > 0 {
			return 0.5 * (data[2] - data[1])
		} else {
			return data[1]
		}
	} else {
		halfLen := len(data) / 2
		minIndices := []int{}
		minIndex, minWidth := -1, float32(math.MaxFloat32)
		for i := 0; i < len(data)-halfLen+1; i++ {
			width := data[i+halfLen-1] - data[i]
			if width < minWidth {
				minIndex, minWidth = i, width
				minIndices = minIndices[:0]
			} else {
				minIndices = append(minIndices, i)
			}
		}
		if len(minIndices) > 0 {
			mi := minIndices[len(minIndices)/2]
			return HalfSampleModeSorted(data[mi : mi+halfLen])
		} else {
			return HalfSampleModeSorted(data[minIndex : minIndex+halfLen])
		}
	}
}

// Calculate scale and location based on histogram
func HistogramScaleLoc(data []float32, min, max float32, numBins uint32) (loc, scale float32) {
	// deal with edge case
	if min == max {
		return min, 0
	}

	// calculate histogram
	//LogPrintf("calculating %d bin histogram for %d data points in [%.2f%% .. %.2f%%]\n", numBins, len(data), min*100, max*100)

	bins := make([]uint32, numBins)
	valueToBin := float32(numBins-1) / (max - min)
	for _, d := range data {
		bin := uint32(((d - min) * valueToBin) + 0.5)
		bins[bin]++
	}

	// find inner peak (avoid edges which may be distorted by clipping)
	peakBin, peakCount := uint32(0), uint32(0)
	for bin, count := range bins[1 : numBins-1] {
		if count > peakCount {
			peakBin, peakCount = uint32(bin+1), count
		}
	}
	loc = min + float32(peakBin)/valueToBin
	// LogPrintf("histogram peak: bin[%d] = %d (%.2f%%) -> value %.2f%%\n", peakBin, peakCount, 100*float32(peakCount)/float32(len(data)), loc*100)

	// Find standard deviation around the histogram peak by cumulating adjacent bins until one sigma threshold of 68.27% is reached
	// See https://en.wikipedia.org/wiki/68%E2%80%9395%E2%80%9399.7_rule
	sigmaThreshold := uint32(float32(len(data)) * 0.6827)
	intervalLimit := peakBin
	if numBins-1-peakBin < intervalLimit {
		intervalLimit = numBins - 1 - peakBin
	}
	cum := peakCount
	scale = 0.5 * float32(1.0) / valueToBin
	i := uint32(0)

	if cum < sigmaThreshold {
		for i = 1; i <= intervalLimit; i++ {
			cum = cum + bins[peakBin-i] + bins[peakBin+i]
			scale = 0.5 * float32(2*i+1) / valueToBin
			if cum >= sigmaThreshold {
				break
			}
		}
	}
	// LogPrintf("bins[%d +/-%d]=%d vs threshold %d; scale %.2f%%\n", peakBin,i, cum, sigmaThreshold, scale*100)
	return loc, scale
}
