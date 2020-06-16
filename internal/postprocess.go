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
	"runtime"
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
func PostProcessLights(alignRef, histoRef *FITSImage, lights []*FITSImage, align int32, alignK int32, alignThreshold float32, normalize HistoNormMode, oobMode OutOfBoundsMode, postProcessedPattern string) (numErrors int) {
	var aligner *Aligner=nil
	if align!=0 {
		aligner=NewAligner(alignRef.Naxisn, alignRef.Stars, alignK)
	}
	numErrors=0
	sem   :=make(chan bool, runtime.NumCPU())
	for i, lightP := range(lights) {
		sem <- true 
		go func(i int, lightP *FITSImage) {
			defer func() { <-sem }()
			res, err:=postProcessLight(aligner, histoRef, lightP, alignThreshold, normalize, oobMode)
			if err!=nil {
				LogPrintf("%d: Error: %s\n", lightP.ID, err.Error())
				numErrors++
			} else if postProcessedPattern!="" {
				// Write image to (temporary) file
				res.WriteFile(fmt.Sprintf(postProcessedPattern, lightP.ID))				
			}
			if res!=lightP {
				PutArrayOfFloat32IntoPool(lightP.Data)
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

// Postprocess a single light frame with given settings.
// Post-processing includes normalization, alignment determination and resampling in reference frame
func postProcessLight(aligner *Aligner, histoRef, light *FITSImage, alignThreshold float32, normalize HistoNormMode, oobMode OutOfBoundsMode) (res *FITSImage, err error) {
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
	}

	// skip alignment if this is the reference frame
	if aligner==nil || (len(aligner.RefStars)==len(light.Stars) && (&aligner.RefStars[0]==&light.Stars[0])) {
		light.Trans=IdentityTransform2D()		
		return light, nil
	}

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
	LogPrintf("%d: Transform %v; residual %.3g\n", light.ID, light.Trans, light.Residual)

	// Project image into reference frame
	return light.Project(aligner.Naxisn, trans, outOfBounds)
}