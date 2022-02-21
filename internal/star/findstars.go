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


package star

import (
	"io"
	"fmt"
	"math"
	"github.com/valyala/fastrand"
	"github.com/mlnoga/nightlight/internal/stats"
	"github.com/mlnoga/nightlight/internal/median"
	//"sort"
)

// A star, as found on an image by star detection
type Star struct {
	Index int32 		// Index of the star in the data array. int32(x)+width*int32(y)
	Value float32       // Value of the star in the data array. data[index]
	X     float32       // Precise star x position via center of mass
	Y     float32       // Precise star y position via center of mass
	Mass  float32       // Star mass. Summed pixel values above location estimate, within given radius
	HFR	  float32       // Half-Flux Radius of the star, in pixels
}

// Adapter method 1 to make Star work with KD-Tree  
func (s *Star) Dimensions() int {
	return 2
}

// Adapter method 2 to make Star work with KD-Tree  
func (s *Star) Dimension(i int) float64 {
	if i==0 { return float64(s.X) }
	return float64(s.Y)
}

// Prints given array of stars as CSV 
func PrintStars(w io.Writer, stars []Star) {
	fmt.Fprintln(w,"Index,Value,X,Y,Mass,HFR")
	for _,s :=range stars {
		fmt.Fprintf(w,"%d,%g,%g,%g,%g,%g\n", s.Index, s.Value, s.X, s.Y, s.Mass, s.HFR)
	}
}

// Find stars in the given image with data type int16
func FindStars(data []float32, width int32, location, scale, starSig, bpSigma, starInOut float32, radius int32, medianDiffStats *stats.Basic) (stars []Star, sumOfShifts, avgHFR float32) {
	// Begin star identification based on pixels significantly above the background
	stars=findBrightPixels(data, width, location+scale*starSig, radius)
	//LogPrintf("%d (%.4g%%) initial stars \n", len(stars), (100.0*float32(len(stars))/float32(len(data))))

	// reject bad pixels which differ significantly from the local median
	if bpSigma>0 {
		stars=rejectBadPixels(stars, data, width, bpSigma, medianDiffStats)
		// LogPrintf("%d (%.4g%%) stars after bad pixel rejection \n", len(stars), (100.0*float32(len(stars))/float32(len(data))))
	}
	
	// filter out faint stars overlapped by brighter ones
	QSortStarsDesc(stars)
	stars=filterOutOverlaps(stars, width, int32(len(data))/width, radius)
	//LogPrintf("%d (%.4g%%) stars left after +/-%d blocking mask\n", len(stars), (100.0*float32(len(stars))/float32(len(data))), radius)

	// move stars to centroid position
	sumOfShifts=shiftToCenterOfMass(stars, data, width, location+scale*starSig*0.5, radius)
	//LogPrintf("%.6g sum of shifts with center of mass box +/-%d\n", sumOfShifts, radius)

	// filter out faint stars again
	QSortStarsDesc(stars)
	stars=filterOutOverlaps(stars, width, int32(len(data))/width, radius)
	//LogPrintf("%d (%.4g%%) stars left after +/-%d blocking mask\n", len(stars), (100.0*float32(len(stars))/float32(len(data))), radius)

	// remove implausible stars based on HFR and mass
	stars, avgHFR=calcAndFilterHalfFluxRadius(stars, data, width, float32(radius), location, starInOut)
	//LogPrintf("%d (%.2g%%) stars left after HFR calc, avg HFR %.2g\n", len(stars), (100.0*float32(len(stars))/float32(len(data))), avgHFR)

	// maxIndex:=10
	// if maxIndex>len(stars) { maxIndex=len(stars)}
	// LogPrintf("Top    %d stars: %v\n", maxIndex, stars[:maxIndex])
	// LogPrintf("Bottom %d stars: %v\n", maxIndex, stars[len(stars)-maxIndex:])
	// PrintStars(stars)

	// Return a clone of the final shortlist of stars, so the longer original object can be reclaimed
	res:=make([]Star, len(stars))
	copy(res, stars)
	stars=nil

	return res, sumOfShifts, avgHFR
}


// Find pixels above the threshold and return them as stars. Applies early overlap rejection based on radius to reduce allocations.
// Uses central pixel value as initial mass, 1 as initial HFR.
func findBrightPixels(data []float32, width int32, threshold float32, radius int32) []Star {
	stars:=make([]Star,len(data)/100)[:0] // []Star{}

	for i,v :=range data {
		if v>threshold {
			is:=Star{Index:int32(i), Value:v, X:float32(int32(i) % width), Y:float32(int32(i) / width), Mass:v, HFR:1}

			// check if within radius distance of the previously detected candidate star to optimize memory usage
			if len(stars)>0 {
				oldS:=stars[len(stars)-1]
				if oldS.Y==is.Y && oldS.X>=is.X-float32(radius) {
					if oldS.Value>=is.Value { 
						continue  // keep old candidate, as it's brighter
					} else {
						stars[len(stars)-1]=is
						continue  // replace old candidate with brighter new one
					}
				}
			}

			stars=append(stars, is)  // add as additional candidate
		}
	}	
	return stars
}


// Reject bad pixels which differ from the local median by more than sigma times the estimated standard deviation
// Modifies the given stars array values, and returns shortened slice
func rejectBadPixels(stars []Star, data []float32, width int32, sigma float32, medianDiffStats *stats.Basic) []Star {
	// Create mask for local 9-neighborhood
	mask:=CreateMask(width, 1.5)
	buffer:=make([]float32, len(mask))

	if medianDiffStats==nil {
		// Estimate standard deviation of pixels from local neighborhood median based on random 1% of pixels
		numSamples:=len(data)/100
		samples:=make([]float32,numSamples)
		rng:=fastrand.RNG{}
		for i:=0; i<numSamples; i++ {
			index:=int32(rng.Uint32n(uint32(len(data))))
			median :=median.GatherAndMedian(data, index, mask, buffer)
			samples[i]=data[index]-median
		}
		medianDiffStats=stats.CalcBasicStats(samples)
		samples=nil
	}

	// Filter out star candidates more than sigma standard deviations away from the estimated local median 
	threshold:=medianDiffStats.StdDev*sigma
	remainingStars:=0
	for _,s := range(stars) {
		median:=median.GatherAndMedian(data, s.Index, mask, buffer)
		diff:=data[s.Index]-median
		if diff<threshold && -diff<threshold {
			// keep result
			stars[remainingStars]=s
			remainingStars++
		} else {
			// DESTRUCTIVE: replace pixel in image with mean
			// data[s.Index]=mean
		}
	}
	return stars[:remainingStars]
}


// Calculates the average of the neighbors of the given index. Indices/offsets outside the data range are ignored. 
func averageNeighbors(data []float32, index int32, neighborOffsets []int32) float32 {
	sum, count:=float32(0), int32(0)
	for _,offset:=range neighborOffsets {
		i:=index+offset
		if i>=0 && i<=int32(len(data)) {
			sum+=data[i]
			count++
		}
	}
	return sum/float32(count)
}


// Creates a mask of given radius. Returns a list of index offsets
func CreateMask(width int32, radius float32) []int32 {
	mask:=[]int32{}
	rad:=int32(radius)
	for y:=-rad; y<=rad; y++ {
		for x:=-rad; x<=rad; x++ {
			dist:=float32(math.Sqrt(float64(y*y+x*x)))
			if dist<=radius+1e-8 {
				offset:=y*int32(width)+x
				mask=append(mask, offset)
			}
		}
	}
	return mask
}

// A singly linked list of stars. Used for filtering out overlaps
type starListItem struct {
	Star *Star
	Next *starListItem
}

// Filters out overlaps from the stars. Uses distance and centroid mass as ordering criteria.
func filterOutOverlaps(stars []Star, width, height, radius int32) []Star {
	// To avoid quadratic search effort, we bin the stars into a 2D grid.
	// Each bin is a linked list of stars, sorted by descending mass
	binSize:=int32(256)
	xBins  :=(width +binSize-1)/binSize
	yBins  :=(height+binSize-1)/binSize
	bins   :=make([]*starListItem,int(xBins*yBins))
	slis   :=make([]starListItem,((len(stars)+1023)/1024)*1024) // use tiered sizing to help the allocator
	radiusSquared:=radius*radius

	// For all stars, filter list in place
	numRemainingStars:=0
	forAllStars:
	for _,s:=range stars {
		// Find grid cell of this star
		xCell, yCell:=int32(s.X+0.5)/binSize, int32(s.Y+0.5)/binSize

		// For this grid cell and all adjacent cells
		for dy:=int32(-1); dy<=1; dy++ {
			if yCell+dy<0 || yCell+dy>=yBins { continue }
			for dx:=int32(-1); dx<=1; dx++ {
				if xCell+dx<0 || xCell+dx>=xBins { continue }
				cellIndex:=(xCell+dx)+(yCell+dy)*xBins
				
				// For all prior stars in that cell
				for ptr:=bins[cellIndex]; ptr!=nil; ptr=ptr.Next {
					s2    :=ptr.Star
					xDist :=s.X-s2.X
					yDist :=s.Y-s2.Y
					sqDist:=int32(xDist*xDist + yDist*yDist+0.5)

					// Skip current star if it's close to a prior star
					if sqDist<=radiusSquared {
						continue forAllStars
					}
				}
			}
		}

		// Retain star for output
		stars[numRemainingStars]=s

		// Insert star into grid cell
		slis[numRemainingStars]  =starListItem{&(stars[numRemainingStars]),nil}
		cellIndex:=xCell+yCell*xBins
		ptr      :=bins[cellIndex]
		if ptr==nil {
			bins[cellIndex]=&(slis[numRemainingStars])
		} else {
			for ptr.Next!=nil {
				ptr=ptr.Next
			}
			ptr.Next=&(slis[numRemainingStars])
		}

		numRemainingStars++
	}

	bins=nil
	slis=nil
	// Return shortened list of stars as result
	return stars[:numRemainingStars]
}

// Shifts each star to its floating point-valued center of mass. Modifies stars in place
func shiftToCenterOfMass(stars []Star, data []float32, width int32, threshold float32, radius int32) (sumOfShifts float32) {
	// for all stars
	sumOfShifts=float32(0)
	for i,s:=range stars {

		// until the shifts are below 0.01 pixel (i.e. 0.0001 squared error), or max rounds reached
		shiftSquared:=float32(math.MaxFloat32)
		for round:=int32(0); shiftSquared>0.0001 && round<10; round++ {
			// calculate star mass and first moments from current x,y
			xMoment, yMoment:=float32(0), float32(0)
			mass:=float32(0)
			for y:=-radius; y<=radius; y++ {
				for x:=-radius; x<=radius; x++ {
					index:=s.Index+y*int32(width)+x
					value:=float32(0)
					if index>=0 && int(index)<len(data) {
						value=data[index]-threshold
						if value<0 { value=0 }
					}
					xMoment+=float32(x)*value
					yMoment+=float32(y)*value
					mass+=value
				}
			}

			// update x and y from moments over mass
			x:=s.Index % int32(width)
			y:=s.Index / int32(width)
			if mass==0.0 { mass=1e-8 }
			deltaX:=(xMoment)/mass
			deltaY:=(yMoment)/mass
			newX:=float32(x)+deltaX
			newY:=float32(y)+deltaY

			preciseDeltaX:=newX-s.X
			preciseDeltaY:=newY-s.Y
			shiftSquared  =preciseDeltaX*preciseDeltaX + preciseDeltaY*preciseDeltaY
			index:=s.Index + width*int32(deltaY+0.5)+int32(deltaX+0.5)
			value:=float32(0)
			if index>=0 && int(index)<len(data) {
				value=float32(data[index])
			}
			s=Star{Index:index, Value:value, X:float32(newX), Y:float32(newY), Mass:float32(mass)}
			stars[i]=s
		}
		sumOfShifts+=float32(math.Sqrt(float64(shiftSquared)))
	}	
	return sumOfShifts
}

// Calculate the Half-Flux Radius of each star, and filters out implausible candidates
// Returns a new list of stars, each enriched with the HFR field and updated mass
// Based on the algorithm in https://en.wikipedia.org/wiki/Half_flux_diameter
func calcAndFilterHalfFluxRadius(stars []Star, data []float32, width int32, radius, location, starInOut float32) (res []Star, avgHFR float32) {
	numRemainingStars:=0
	avgHFR=float32(0)

	for _,s:=range stars {
		// calculate mass, moment and HFR
		moment, mass, pixels:=float32(0), float32(0), int32(0)
		rad:=int32(math.Ceil(float64(radius)))
		distSqLimit:=int32(math.Ceil(float64(radius+1e-8)*float64(radius+1e-8)))
		for y:=-rad; y<=rad; y++ {
			for x:=-rad; x<=rad; x++ {
				distSq:=x*x+y*y
				if distSq>distSqLimit { continue }
				distance:=float32(math.Sqrt(float64(distSq)))

				index:=s.Index+y*width+x
				value:=float32(0.0)
				if index>=0 && index<int32(len(data)) {
					v:=data[index]-location
					if v>0 { value=v }
				}
				moment  +=distance*value
				mass    +=value
				pixels++
			}
		}
		if mass==0.0 { mass=1e-8 }
		hfr:=float32(moment/mass)

		// sanity check results to avoid long lockups
		if hfr>radius { continue } 

		// calculate mass inside HFR and number of inner pixels
		innerMass, innerPixels:=float32(0), int32(0)
		innerRad:=int32(math.Ceil(float64(hfr)))
		distSqLimit=int32(math.Ceil(float64(hfr*hfr)))
		for y:=-innerRad; y<=innerRad; y++ {
			for x:=-innerRad; x<=innerRad; x++ {
				distSq:=x*x+y*y
				if distSq>distSqLimit { continue }

				index:=s.Index+y*width+x
				value:=float32(0.0)
				if index>=0 && index<int32(len(data)) {
					v:=data[index]-location
					if v>0 { value=v }
				}
				innerMass  +=value
				innerPixels++
			}
		}

		// plausibility check: is average inner brightness significantly higher than outside? 
		outerMass  :=mass  -innerMass
		outerPixels:=pixels-innerPixels
		if innerMass*float32(outerPixels) <= starInOut*outerMass*float32(innerPixels) { continue }
		// this is equivalent to innerMass/innerPixels <= outerMass/outerPixels + threshold, but avoids divide by zero issues
		// if innerMass*float32(outerPixels) <= outerMass*float32(innerPixels)+ threshold*float32(innerPixels)*float32(outerPixels) { continue }

		// keep star, enrich with HFR and mass information, and update average
		s.HFR=hfr
		s.Mass=mass
		stars[numRemainingStars]=s
		numRemainingStars++

		avgHFR+=float32(hfr)
	}
	avgHFR/=float32(numRemainingStars)
	return stars[:numRemainingStars], avgHFR
}
