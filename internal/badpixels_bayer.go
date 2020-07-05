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
)


// Apply cosmetic correction to CFA data
func CosmeticCorrectionBayer(data []float32, width int32, debayer, cfa string, sigmaLow, sigmaHigh float32) (numRemoved int32, err error) {
	// translate CFA type to offsets in standard RGGB array type
	xOffset, yOffset, err:=getOffsets(cfa)
	if err!=nil { return 0, err }

	median:=make([]float32, len(data))

	// select operation based on desired color channel
	switch(debayer) {
	case "R","r": 
		return CosmeticCorrectionBayerRedOrBlue(median, data, width, xOffset+0, yOffset+0, sigmaLow, sigmaHigh), nil
	case "G","g": 
		return CosmeticCorrectionBayerGreen(median, data, width, xOffset, yOffset, sigmaLow, sigmaHigh), nil
	case "B","b":
		return CosmeticCorrectionBayerRedOrBlue(median, data, width, xOffset+1, yOffset+1, sigmaLow, sigmaHigh), nil
	default:      
		return 0, errors.New("Unknown debayering value " + debayer)
	}
}


// Apply cosmetic correction to CFA data red or blue channels
func CosmeticCorrectionBayerRedOrBlue(median, data []float32, width int32, xOffset, yOffset int32, sigmaLow, sigmaHigh float32) (numRemoved int32) {
	MedianFilterBayerRedOrBlue(median, data, width, xOffset, yOffset)
	_, stdDev:=DeltaStatsBayerRedOrBlue(median, data, width, xOffset, yOffset)
	return ReplaceOutliersBayerRedOrBlue(median, data, width, xOffset, yOffset, -sigmaLow*stdDev, sigmaHigh*stdDev)
}


// Apply cosmetic correction to CFA data green channels
func CosmeticCorrectionBayerGreen(median, data []float32, width int32, xOffset, yOffset int32, sigmaLow, sigmaHigh float32) (numRemoved int32) {
	MedianFilterBayerGreen(median, data, width, xOffset, yOffset)
	_, stdDev:=DeltaStatsBayerGreen(median, data, width, xOffset, yOffset)
	//LogPrintf("mean %f stdDev %f\n", mean, stdDev)
	return ReplaceOutliersBayerGreen(median, data, width, xOffset, yOffset, -sigmaLow*stdDev, sigmaHigh*stdDev)
}


// Apply median filter to CFA data red or blue channels 
func MedianFilterBayerRedOrBlue(res, data []float32, width, xOffset, yOffset int32) {
	height   :=int32(len(data))/width
	tmp:=[]float32{0,0,0,0,0,0,0,0,0}

	/* LogPrintln("Input data")
	for y:=yOffset; y<height; y+=2 {
		LogPrintf("%2d:", y)
		for x:=xOffset; x<width; x+=2 {
			LogPrintf(" %03.1f", data[y*width+x])
		}
		LogPrintln("")
	} */

	// for all RGGB boxes
	for y:=yOffset; y<height; y+=2 {
		for x:=xOffset; x<width; x+=2 {
			numGathered:=0
			// for the local neighborhood
			for nOffsY:=int32(-2); nOffsY<=2; nOffsY+=2 {
				neighborY:=y+nOffsY
				if neighborY<0 || neighborY>=height { continue }

				for nOffsX:=int32(-2); nOffsX<=2; nOffsX+=2 {
					neighborX:=x+nOffsX
					if neighborX<0 || neighborX>=width { continue }

					index:=neighborY*width + neighborX
					tmp[numGathered]=data[index]
					numGathered++
				}
			}
			median:=MedianFloat32(tmp[:numGathered])
			res[y*width + x]=median
		}
	}

	/* LogPrintln("Median")
	for y:=yOffset; y<height; y+=2 {
		LogPrintf("%2d:", y)
		for x:=xOffset; x<width; x+=2 {
			LogPrintf(" %03.1f", res[y*width+x])
		}
		LogPrintln("")
	} */

}


// Pair of int32
type pairOfint32 struct {
	X int32
	Y int32
}


// Offsets for median filtering green elements of the bayer array
var gOffsets =[]pairOfint32{
	pairOfint32{ 0,-2}, 
	pairOfint32{-1,-1}, 
	pairOfint32{ 1,-1}, 
	pairOfint32{-2, 0}, 
	pairOfint32{ 0, 0}, 
	pairOfint32{ 2, 0}, 
	pairOfint32{-1, 1}, 
	pairOfint32{ 1, 1}, 
	pairOfint32{ 0, 2}, 
}


// Apply median filter to CFA data red or blue channels 
func MedianFilterBayerGreen(res, data []float32, width, xOffset, yOffset int32) {
	height   :=int32(len(data))/width
	tmp:=[]float32{0,0,0,0,0,0,0,0,0}

	/*LogPrintln("Input data")
	colorOffsetX:=int32(0)
	for y:=yOffset; y<height; y+=1 {
		colorOffsetX=1-colorOffsetX
		LogPrintf("%2d:", y)
		for x:=xOffset+colorOffsetX; x<width; x+=2 {
			LogPrintf(" %03.1f", data[y*width+x])
		}
		LogPrintln("")
	}*/

	// for all RGGB boxes
	colorOffsetX:=int32(0)
	for y:=yOffset; y<height; y+=1 {
		colorOffsetX=1-colorOffsetX
		for x:=xOffset+colorOffsetX; x<width; x+=2 {
			numGathered:=0
			// for the local neighborhood
			for _,nOffsets:=range gOffsets {
				neighborY:=y+nOffsets.Y
				if neighborY<0 || neighborY>=height { continue }

				neighborX:=x+nOffsets.X
				if neighborX<0 || neighborX>=width { continue }

				index:=neighborY*width + neighborX
				tmp[numGathered]=data[index]
				numGathered++
			}
			median:=MedianFloat32(tmp[:numGathered])
			res[y*width + x]=median
		}
	}

	/*LogPrintln("Median")
	colorOffsetX=int32(0)
	for y:=yOffset; y<height; y+=1 {
		colorOffsetX=1-colorOffsetX
		LogPrintf("%2d:", y)
		for x:=xOffset+colorOffsetX; x<width; x+=2 {
			LogPrintf(" %03.1f", res[y*width+x])
		}
		LogPrintln("")
	}*/
}


// Calculate statistics for data - median, on red or blue channel
func DeltaStatsBayerRedOrBlue(median, data []float32, width, xOffset, yOffset int32) (mean, stdDev float32) {
	height   :=int32(len(data))/width

	// for all rows
	deltaSum:=float32(0)
	deltaNum:=int32(0)
	for y:=yOffset; y<height; y+=2 {

		// for all pixels in the row
		deltaRowSum:=float32(0)
		deltaRowNum:=int32(0)
		for x:=xOffset; x<width; x+=2 {
			index:=y*width + x
			delta:=data[index]-median[index]
			deltaRowSum+=delta
			deltaRowNum++
		}

		deltaSum+=deltaRowSum
		deltaNum+=deltaRowNum
	}
	mean=deltaSum/float32(deltaNum)

	// for all rows
	deltaSum=float32(0)
	deltaNum=int32(0)
	for y:=yOffset; y<height; y+=2 {

		// for all pixels in the row
		deltaRowSum:=float32(0)
		deltaRowNum:=int32(0)
		for x:=xOffset; x<width; x+=2 {
			index:=y*width + x
			delta:=data[index]-median[index]
			deltaSq:=(delta-mean)*(delta-mean)
			deltaRowSum+=deltaSq
			deltaRowNum++
		}

		deltaSum+=deltaRowSum
		deltaNum+=deltaRowNum
	}
	var variance float32=0
	if deltaNum>0 {
		variance=deltaSum/float32(deltaNum)
	}
	stdDev=float32(math.Sqrt(float64(variance)))
	//LogPrintf("deltaSum %f deltaNum %d variance %f stdDev %f\n", deltaSum, deltaNum, variance, stdDev)
	return mean, stdDev
}


// Calculate statistics for data - median, on green channel
func DeltaStatsBayerGreen(median, data []float32, width, xOffset, yOffset int32) (mean, stdDev float32) {
	height   :=int32(len(data))/width

	// for all rows
	deltaSum:=float32(0)
	deltaNum:=int32(0)
	colorOffsetX:=int32(0)
	for y:=yOffset; y<height; y+=1 {
		colorOffsetX=1-colorOffsetX

		// for all pixels in the row
		deltaRowSum:=float32(0)
		deltaRowNum:=int32(0)
		for x:=xOffset+colorOffsetX; x<width; x+=2 {
			index:=y*width + x
			delta:=data[index]-median[index]
			deltaRowSum+=delta
			deltaRowNum++
		}

		deltaSum+=deltaRowSum
		deltaNum+=deltaRowNum
	}
	mean=deltaSum/float32(deltaNum)

	// for all rows
	deltaSum=float32(0)
	deltaNum=int32(0)
	colorOffsetX=int32(0)
	for y:=yOffset; y<height; y+=1 {
		colorOffsetX=1-colorOffsetX

		// for all pixels in the row
		deltaRowSum:=float32(0)
		deltaRowNum:=int32(0)
		for x:=xOffset+colorOffsetX; x<width; x+=2 {
			index:=y*width + x
			delta:=data[index]-median[index]
			deltaSq:=(delta-mean)*(delta-mean)
			deltaRowSum+=deltaSq
			deltaRowNum++
		}

		deltaSum+=deltaRowSum
		deltaNum+=deltaRowNum
	}

	var variance float32=0
	if deltaNum>0 {
		variance=deltaSum/float32(deltaNum)
	}
	stdDev=float32(math.Sqrt(float64(variance)))
	//LogPrintf("deltaSum %f deltaNum %d variance %f stdDev %f\n", deltaSum, deltaNum, variance, stdDev)
	return mean, stdDev
}


// Replace outliers in data, which are lower than threshLow less than the median, or higher than treshHigh more than the median, with median
func ReplaceOutliersBayerRedOrBlue(median, data []float32, width, xOffset, yOffset int32, threshLow, threshHigh float32) (numRemoved int32) {
	height   :=int32(len(data))/width
	numRemoved=0

	//LogPrintf("Replacing outliers with data-median < %f or > %f\n", threshLow, threshHigh)

	// for all rows
	for y:=yOffset; y<height; y+=2 {
		// for all pixels in the row
		for x:=xOffset; x<width; x+=2 {
			index:=y*width + x
			delta:=data[index]-median[index]
			if delta<threshLow || delta>threshHigh {
				data[index]=median[index]
				numRemoved++
			}
		}
	}

	/* LogPrintln("Corrected")
	for y:=yOffset; y<height; y+=2 {
		LogPrintf("%2d:", y)
		for x:=xOffset; x<width; x+=2 {
			LogPrintf(" %03.1f", data[y*width+x])
		}
		LogPrintln("")
	} */

	return numRemoved
}


// Replace outliers in data, which are lower than threshLow less than the median, or higher than treshHigh more than the median, with median
func ReplaceOutliersBayerGreen(median, data []float32, width, xOffset, yOffset int32, threshLow, threshHigh float32) (numRemoved int32) {
	height   :=int32(len(data))/width
	numRemoved=0

	// for all rows
	colorOffsetX:=int32(0)
	for y:=yOffset; y<height; y+=1 {
		colorOffsetX=1-colorOffsetX

		// for all pixels in the row
		for x:=xOffset+colorOffsetX; x<width; x+=2 {
			index:=y*width + x
			delta:=data[index]-median[index]
			if delta<threshLow || delta>threshHigh {
				data[index]=median[index]
				numRemoved++
			}
		}
	}
	return numRemoved
}