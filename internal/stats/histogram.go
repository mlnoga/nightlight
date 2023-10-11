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
	"math"
	//"fmt"
	"gonum.org/v1/gonum/optimize" // for alignment. source via "go get gonum.org/v1/gonum"
	// also needs "go get golang.org/x/exp/rand"
	// also needs "go get golang.org/x/tools/container/intsets"
	//"gonum.org/v1/gonum/diff/fd"    // for alignment. source via "go get gonum.org/v1/gonum"
	// also needs "go get golang.org/x/exp/rand"
	// also needs "go get golang.org/x/tools/container/intsets"
	//"gonum.org/v1/gonum/mat"        // for alignment. source via "go get gonum.org/v1/gonum"
	// also needs "go get golang.org/x/exp/rand"
	// also needs "go get golang.org/x/tools/container/intsets"
)

// Calculate histogram of data between min and max into given bins
func Histogram(data []float32, min, max float32, bins []int32) {
	for i := range bins {
		bins[i] = 0
	}
	scale := float32(len(bins)-1) / (max - min)
	for _, d := range data {
		index := (d - min) * scale
		bins[int(index)]++
	}
}

// Returns the location and the value of the histogram peak
func GetPeak(bins []int32, min, max float32) (x, y float32) {
	maxIndex, maxValue := -1, int32(math.MinInt32)
	for i, v := range bins {
		if v > maxValue {
			maxIndex, maxValue = i, v
		}
	}

	x = min + (float32(maxIndex)+0.5)*(max-min)/float32(len(bins)-1)
	y = 0.5 * float32(bins[maxIndex]+bins[maxIndex+1])
	return x, y
}

// Calculates the mode and the standard deviation of the given histogram
func GetModeStdDevFromHistogram(bins []int32, min, max float32) (mode, stdDev float32, err error) {
	// Take an educated initial guess: the maximum value of the histogram
	peak, peakVal := GetPeak(bins, min, max)
	//LogPrintf("Initial peak value %.4g at %.4g\n", peakVal, peak )

	// Now minimize the distance between the histogram and a normal distribution
	x0 := []float64{float64(peakVal), float64(peak), 5.0}
	problem := optimize.Problem{
		Func: func(x []float64) float64 {
			alpha, mu, sigma := float32(x[0]), float32(x[1]), float32(x[2])
			scaler := alpha / (sigma * float32(math.Sqrt(2*math.Pi)))
			sumSqDiff := float32(0)
			//sumAbsDiff:=float32(0)

			for i, y := range bins {
				x := min + (float32(i)+0.5)*(max-min)/float32(len(bins)-1)

				xmusig := (x - mu) / sigma
				yPredict := scaler * float32(math.Exp(float64(-0.5*xmusig*xmusig)))

				diff := float32(y) - yPredict
				sumSqDiff += diff * diff
				//sumAbsDiff+=float32(math.Abs(float64(diff)))
			}
			variance := sumSqDiff / float32(len(bins))
			return math.Sqrt(float64(variance))
			//return math.Sqrt(float64(sumAbsDiff/float32(len(bins))))
		},
	}
	result, err := optimize.Minimize(problem, x0, nil, &optimize.NelderMead{})
	if err != nil {
		return -1, -1, err
	}
	//LogPrintf("Found solution alpha %.4g mu %.4g sigma %.4g with residual %.4g\n", result.X[0], result.X[1], result.X[2], result.F )

	return float32(result.X[1]), float32(result.X[2]), nil
}

func toPerceptualBins(x, min, max float32, numBins int) (logBin float32) {
	return float32(math.Pow(float64((x-min)/(max-min)), 1/2.4)) * float32(numBins-1)
}
func fromPerceptualBins(logBin, min, max float32, numBins int) (x float32) {
	return float32(math.Pow(float64(logBin)/float64(numBins-1), 2.4))*(max-min) + min
}

// Calculate histogram of data between min and max into given bins
func PerceptualHistogram(data []float32, min, max float32, bins []int32) {
	for i := range bins {
		bins[i] = 0
	}
	//scale:=float32(len(bins))/float32(math.Log(float64(max-min+1)))
	for _, d := range data {
		index := toPerceptualBins(d, min, max, len(bins))
		//index:=float32(float64(float32(math.Log(float64(d-min+1)))*scale))
		//index =float32(math.Min(math.Max(float64(index), 0), float64(len(bins)-1)))
		bins[int(index)]++
	}

	/*LogPrintf("log histo min %.4g max %.4g bins %d\n", min, max, len(bins))
	for i,b:=range bins {
		LogPrintf("bin %04d x %.4d count %5d\n", i, fromLogBins(float32(i), min, max, len(bins)), b)
	}*/
}

// Returns the location and the value of the histogram peak
func GetPerceptualHistogramPeak(bins []int32, min, max float32) (x, y float32) {
	maxIndex, maxValue := -1, int32(math.MinInt32)
	for i, v := range bins {
		if v > maxValue {
			maxIndex, maxValue = i, v
		}
	}

	x = fromPerceptualBins(float32(maxIndex)+0.5, min, max, len(bins))
	//scale:=float32(math.Log(float64(max-min+1)))/float32(len(bins))
	//x=min + float32(math.Exp(float64((float32(maxIndex)+0.5)*scale)))
	y = 0.5 * float32(bins[maxIndex]+bins[maxIndex+1])
	return x, y
}

// Calculates the mode and the standard deviation of the given histogram
func GetModeStdDevFromPerceptualHistogram(bins []int32, min, max float32) (mode float32, err error) {
	// Take an educated initial guess: the maximum value of the histogram
	peak, peakVal := GetPerceptualHistogramPeak(bins, min, max)
	//LogPrintf("Initial peak value %.4g at %.4g\n", peakVal, peak )
	//scale:=float32(math.Log(float64(max-min+1)))/float32(len(bins))

	// Now minimize the distance between the histogram and a normal distribution
	x0 := []float64{float64(peakVal), float64(peak), 5.0}
	fcn := func(x []float64) float64 {
		alpha, mu, sigma := float32(x[0]), float32(x[1]), float32(x[2])
		scaler := alpha / (sigma * float32(math.Sqrt(2*math.Pi)))
		sumSqDiff := float32(0)
		//sumAbsDiff:=float32(0)

		for i, y := range bins {
			x := fromPerceptualBins(float32(i)+0.5, min, max, len(bins))
			//x:=min + float32(math.Exp(float64((float32(i)+0.5)*scale)))

			xmusig := (x - mu) / sigma
			yPredict := scaler * float32(math.Exp(float64(-0.5*xmusig*xmusig)))

			diff := float32(y) - yPredict
			sumSqDiff += diff * diff
			//sumAbsDiff+=float32(math.Abs(float64(diff)))
		}
		variance := sumSqDiff / float32(len(bins))
		return math.Sqrt(float64(variance))
		//return math.Sqrt(float64(sumAbsDiff/float32(len(bins))))
	}
	/*
		grad := func(grad, x []float64) {
			fd.Gradient(grad, fcn, x, nil)
		}
		hess := func(h *mat.SymDense, x []float64) {
			fd.Hessian(h, fcn, x, nil)
		}*/

	problem := optimize.Problem{
		Func: fcn,
		/*Grad: grad,
		Hess: hess,*/
	}

	result, err := optimize.Minimize(problem, x0, nil, &optimize.NelderMead{})
	if err != nil {
		return -1, err
	}
	//LogPrintf("Found solution alpha %.4g mu %.4g sigma %.4g with residual %.4g\n", result.X[0], result.X[1], result.X[2], result.F )

	return float32(result.X[1]), nil
}
