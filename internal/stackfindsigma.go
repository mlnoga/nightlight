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
	"runtime/debug"
)


// Find lower and upper sigma bounds given desired clipping percentages, and stack using these values
func FindSigmasAndStack(lights []*FITSImage, mode StackMode, weights []float32, refMedian, stClipPercLow, stClipPercHigh float32) (result *FITSImage, numClippedLow, numClippedHigh int32, sigmaLow, sigmaHigh float32, err error) {
	// If desired, auto-select stacking mode based on number of frames    
	if mode==StAuto { 
		mode=autoSelectStackingMode(len(lights))
		LogPrintf("Auto-selected stacking mode %d based on %d frames\n", mode, len(lights))
	}

    // Binary search does not work for linear fit stacking, as changing one bound has an impact on the other.
    // However, Newton search in two dimensions is slower than dual binary search.
	if mode==StLinearFit {
		return newtonMethodAndStack(lights, mode, weights, refMedian, stClipPercLow, stClipPercHigh)
	} else if mode==StWinsorSigma || mode==StSigma {
		return binarySearchAndStack(lights, mode, weights, refMedian, stClipPercLow, stClipPercHigh) 
	} else {
		LogPrintf("Stacking mode %d does not support sigmas, proceeding with normal stack.\n", mode)
		result, numClippedLow, numClippedHigh, err = Stack(lights, mode, weights, refMedian, 0.0, 0.0)
		return result, numClippedLow, numClippedHigh, 0.0, 0.0, err
	}
}

// With binary search, find lower and upper sigma bounds given desired clipping percentages, and stack using these values
func binarySearchAndStack(lights []*FITSImage, mode StackMode, weights []float32, refMedian, stClipPercLow, stClipPercHigh float32) (result *FITSImage, numClippedLow, numClippedHigh int32, sigmaLow, sigmaHigh float32, err error) {
	// initialize binary search intervals
	initialLeft, initialRight:=float32(1.0), float32(11.0)
	lowLeft, lowRight:=initialLeft, initialRight
	lowMid:=0.5*(lowLeft+lowRight)
	highLeft, highRight:=initialLeft, initialRight
	highMid:=0.5*(highLeft+highRight)

	for i:=0; ; i++ {
		// Calculate value for midpoint of each interval
		LogPrintf("Step %d: stSigLow %.2f stSigHigh %.2f\n", i, lowMid, highMid)
		var numClippedLow, numClippedHigh int32
		var err error
		stack, numClippedLow, numClippedHigh, err:=Stack(lights, mode, weights, refMedian, lowMid, highMid)
		if err!=nil { return stack, numClippedLow, numClippedHigh, -1, -1, err }
		percL:=float32(numClippedLow )*100.0/float32(len(stack.Data)*len(lights))
		percH:=float32(numClippedHigh)*100.0/float32(len(stack.Data)*len(lights))
		deltaL:=int(100*percL+0.5)-int(100*stClipPercLow)
		deltaH:=int(100*percH+0.5)-int(100*stClipPercHigh)
		// Test completion and abort criteria
		if deltaL==0 && deltaH==0 {
			LogPrintf("Reached %.2f%% and %.2f%% clipping. Settings are -stSigLow %.3f -stSigHigh %.3f\n", stClipPercLow, stClipPercHigh, lowMid, highMid)
			return stack, numClippedLow, numClippedHigh, lowMid, highMid, nil
		}
		if i>=20 {
			LogPrintf("Warning: Binary search did not converge, proceeding with last approximation %.2f and %.2f\n", lowMid, highMid)
			return stack, numClippedLow, numClippedHigh, lowMid, highMid, nil
		}

		stack=nil // mark memory for free-up
		debug.FreeOSMemory()

		// Adjust binary search interval for lower stacking sigma
		if deltaL>0 {
			lowLeft=lowMid
			lowMid=0.5*(lowLeft+lowRight)
		} else if deltaL<0 {
			lowRight=lowMid
			lowMid=0.5*(lowLeft+lowRight)
		}

		// Adjust binary search interval for upper stacking sigma
		if deltaH>0 {
			highLeft=highMid
			highMid=0.5*(highLeft+highRight)
		} else if deltaH<0 {
			highRight=highMid
			highMid=0.5*(highLeft+highRight)
		}
	}
}

// With Newton's method, find lower and upper sigma bounds given desired clipping percentages, and stack using these values
func newtonMethodAndStack(lights []*FITSImage, mode StackMode, weights []float32, refMedian, stClipPercLow, stClipPercHigh float32) (result *FITSImage, numClippedLow, numClippedHigh int32, sigmaLow, sigmaHigh float32, err error) {
	sigLow, sigHigh, epsilon :=float32(6.0), float32(6.0), float32(0.005)

	for i:=0; ; i++ {
		// Calculate value for current sigmas
		LogPrintf("Step %d: stSigLow %.2f stSigHigh %.2f\n", i, sigLow, sigHigh)
		var numClippedLow, numClippedHigh int32
		var err error
		stack, numClippedLow, numClippedHigh, err:=Stack(lights, mode, weights, refMedian, sigLow, sigHigh)
		if err!=nil { return stack, numClippedLow, numClippedHigh, stClipPercLow, stClipPercHigh, err }
		percL:=float32(numClippedLow )*100.0/float32(len(stack.Data)*len(lights))
		percH:=float32(numClippedHigh)*100.0/float32(len(stack.Data)*len(lights))
		deltaL:=percL-stClipPercLow
		deltaH:=percH-stClipPercLow

		deltaLi:=int(100*deltaL+0.5)
		deltaHi:=int(100*deltaH+0.5)

		// Test completion and abort criteria
		if deltaLi==0 && deltaHi==0 {
			LogPrintf("Reached %.2f%% and %.2f%% clipping. Settings are -stSigLow %.3f -stSigHigh %.3f\n", stClipPercLow, stClipPercHigh, sigLow, sigHigh)
			return stack, numClippedLow, numClippedHigh, sigLow, sigHigh, nil
		}
		if i>=20 {
			LogPrintf("Warning: Newton method did not converge, proceeding with last approximation %.2f and %.2f\n", sigLow, sigHigh)
			return stack, numClippedLow, numClippedHigh, sigLow, sigHigh, nil
		}
		stack=nil // mark memory for free-up
		debug.FreeOSMemory()

		// Vary sigmaLow by epsilon, and compute new value via Newton's rule x_n+1 = x_n - f(x_n)/f'(x_n)
		i++
		LogPrintf("Step %d: stSigLow+eps %.2f, stSigHigh %.2f\n", i, sigLow+epsilon, sigHigh)
		stack2, numClippedLow2, numClippedHigh2, err:=Stack(lights, mode, weights, refMedian, sigLow+epsilon, sigHigh)
		if err!=nil { return stack2, numClippedLow2, numClippedHigh2, sigLow+epsilon, sigHigh, err }
		percL2:=float32(numClippedLow2 )*100.0/float32(len(stack2.Data)*len(lights))
		deltaL2:=percL2-stClipPercLow
		deltaLDiff:=(deltaL2-deltaL)/epsilon
		if deltaLDiff==0 {
			LogPrintf("Warning: Newton method did not converge, proceeding with last approximation %.2f and %.2f\n", sigLow, sigHigh)
			return stack, numClippedLow, numClippedHigh, sigLow, sigHigh, nil
		}
		newSigLow:=sigLow-deltaL/deltaLDiff
		if newSigLow<0.1 { newSigLow=0.1 }
		if newSigLow>20  { newSigLow=20  }
		stack2=nil // mark memory for free-up
		debug.FreeOSMemory()

		// Vary sigmaHigh by epsilon, and compute new value via Newton's rule x_n+1 = x_n - f(x_n)/f'(x_n)
		i++
		LogPrintf("Step %d: stSigLow %.2f, stSigHigh+eps %.2f\n", i, sigLow, sigHigh+epsilon)
		stack3, numClippedLow3, numClippedHigh3, err:=Stack(lights, mode, weights, refMedian, sigLow, sigHigh+epsilon)
		if err!=nil { return stack3, numClippedLow3, numClippedHigh3, sigLow, sigHigh+epsilon, err }
		percH3:=float32(numClippedHigh3)*100.0/float32(len(stack3.Data)*len(lights))
		deltaH3:=percH3-stClipPercLow
		deltaHDiff:=(deltaH3-deltaH)/epsilon
		if deltaHDiff==0 {
			LogPrintf("Warning: Newton method did not converge, proceeding with last approximation %.2f and %.2f\n", sigLow, sigHigh)
			return stack, numClippedLow, numClippedHigh, sigLow, sigHigh, nil
		}
		newSigHigh:=sigHigh-deltaH/deltaHDiff
		if newSigHigh<0.1 { newSigHigh=0.1 }
		if newSigHigh>20  { newSigHigh=20  }
		stack3=nil // mark memory for free-up
		debug.FreeOSMemory()

		// Update them last, so the new value for sigLow does not modify the eval for sigHigh
		sigLow, sigHigh=newSigLow, newSigHigh
	}
}
