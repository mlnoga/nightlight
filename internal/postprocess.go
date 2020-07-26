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
	"fmt"
	"math"
)

// Replaceemnt mode for out of bounds values when projecting images
type HistoNormMode int
const (
	HNMNone = iota   // Do not normalize histogram
	HNMLocScale      // Normalize histogram by matching location and scale of the reference frame. Good for stacking lights
	HNMLocBlack      // Normalize histogram to match location of the reference frame by shifting black point. Good for RGB
	HNMAuto          // Auto mode. Uses ScaleLoc for stacking, and LocBlack for (L)RGB combination.
)


// Replaceemnt mode for out of bounds values when projecting images
type OutOfBoundsMode int
const (
	OOBModeNaN = iota   // Replace with NaN. Stackers ignore NaNs, so they just take frames into account which have data for the given pixel
	OOBModeRefLocation  // Replace with reference frame location estimate. Good for projecting data for one channel before stacking
	OOBModeOwnLocation  // Replace with location estimate for the current frame. Good for projecting RGB, where locations can differ
)

// Postprocess all light frames with given settings, limiting concurrency to the number of available CPUs
func PostProcessLights(alignRef, histoRef *FITSImage, lights []*FITSImage, align int32, alignK int32, alignThreshold float32, 
	                   normalize HistoNormMode, oobMode OutOfBoundsMode, usmSigma, usmGain, usmThresh float32, 
	                   postProcessedPattern string, imageLevelParallelism int32) (numErrors int) {
	var aligner *Aligner=nil
	if align!=0 {
		if alignRef==nil || alignRef.Stars==nil || len(alignRef.Stars)==0 {
			LogFatal("Unable to align without star detections in reference frame")
		}
		aligner=NewAligner(alignRef.Naxisn, alignRef.Stars, alignK)
	}
	if usmGain>0 { 
		kernel:=GaussianKernel1D(usmSigma)
		LogPrintf("Unsharp masking kernel sigma %.2f size %d: %v\n", usmSigma, len(kernel), kernel)
	}
	numErrors=0
	sem   :=make(chan bool, imageLevelParallelism)
	for i, lightP := range(lights) {
		sem <- true 
		go func(i int, lightP *FITSImage) {
			defer func() { <-sem }()
			res, err:=postProcessLight(aligner, histoRef, lightP, alignThreshold, normalize, oobMode, usmSigma, usmGain, usmThresh)
			if err!=nil {
				LogPrintf("%d: Error: %s\n", lightP.ID, err.Error())
				numErrors++
			} else if postProcessedPattern!="" {
				// Write image to (temporary) file
				err=res.WriteFile(fmt.Sprintf(postProcessedPattern, lightP.ID))				
				if err!=nil { LogFatalf("Error writing file: %s\n", err) }
			}
			if res!=lightP {
				lightP.Data=nil
				lights[i]=res
			}
		}(i, lightP)
	}
	for i:=0; i<cap(sem); i++ {  // wait for goroutines to finish
		sem <- true
	}
	return numErrors
}

// Postprocess a single light frame with given settings. Processing steps can include:
// normalization, alignment and resampling in reference frame, and unsharp masking 
func postProcessLight(aligner *Aligner, histoRef, light *FITSImage, alignThreshold float32, normalize HistoNormMode, 
					  oobMode OutOfBoundsMode, usmSigma, usmGain, usmThresh float32) (res *FITSImage, err error) {
	// Match reference frame histogram 
	switch normalize {
		case HNMNone: 
			// do nothing
		case HNMLocScale:
			light.MatchHistogram(histoRef.Stats)
			LogPrintf("%d: %s\n", light.ID, light.Stats)
		case HNMLocBlack:
	    	light.ShiftBlackToMove(light.Stats.Location, histoRef.Stats.Location)
	    	var err error
	    	light.Stats, err=CalcExtendedStats(light.Data, light.Naxisn[0])
	    	if err!=nil { return nil, err }
			LogPrintf("%d: %s\n", light.ID, light.Stats)
	}

	// Is alignment to the reference frame required?
	if aligner==nil || aligner.RefStars==nil || len(aligner.RefStars)==0 {
		// Generally not required
		light.Trans=IdentityTransform2D()		
	} else if (len(aligner.RefStars)==len(light.Stars) && (&aligner.RefStars[0]==&light.Stars[0])) {
		// Not required for reference frame itself
		light.Trans=IdentityTransform2D()		
	} else if light.Stars==nil || len(light.Stars)==0 {
		// No stars - skip alignment and warn
		LogPrintf("%d: warning: no stars found, skipping alignment", light.ID)
		light.Trans=IdentityTransform2D()		
	} else {
		// Alignment is required
		// determine out of bounds fill value
		var outOfBounds float32
		switch(oobMode) {
			case OOBModeNaN:         outOfBounds=float32(math.NaN())
			case OOBModeRefLocation: outOfBounds=histoRef.Stats.Location
			case OOBModeOwnLocation: outOfBounds=light   .Stats.Location
		}

		// Determine alignment of the image to the reference frame
		trans, residual := aligner.Align(light.Naxisn, light.Stars, light.ID)
		if residual>alignThreshold {
			msg:=fmt.Sprintf("%d:Skipping image as residual %g is above limit %g", light.ID, residual, alignThreshold)
			return nil, errors.New(msg)
		} 
		light.Trans, light.Residual=trans, residual
		LogPrintf("%d: Transform %v; oob %.3g residual %.3g\n", light.ID, light.Trans, outOfBounds, light.Residual)

		// Project image into reference frame
		light, err= light.Project(aligner.Naxisn, trans, outOfBounds)
		if err!=nil { return nil, err }
	}

	// apply unsharp masking, if requested
	if usmGain>0 {
		light.Stats, err=CalcExtendedStats(light.Data, light.Naxisn[0])
		if err!=nil { return nil, err }
		absThresh:=light.Stats.Location + light.Stats.Scale*usmThresh
		LogPrintf("%d: Unsharp masking with sigma %.3g gain %.3g thresh %.3g absThresh %.3g\n", light.ID, usmSigma, usmGain, usmThresh, absThresh)
		light.Data=UnsharpMask(light.Data, int(light.Naxisn[0]), usmSigma, usmGain, light.Stats.Min, light.Stats.Max, absThresh)
		light.Stats=CalcBasicStats(light.Data)
	}

	return light, nil
}