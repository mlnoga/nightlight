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
	"math"
)

// A piecewise linear background, for automated background extraction (ABE)
type Background struct {
	Width int32
	Height int32
	GridSpacing int32	
 	Cells []BGCell
}

// A background cell, which is modeled as a linear gradient
type BGCell struct {
	Alpha float32  // x axis multiplier
	Beta  float32  // y axis multiplier
	Gamma float32  // constant offset
}

// Creates new background by fitting linear gradients to grid cells of the given image, masking out areas in given mask
func NewBackground(src []float32, width int32, gridSpacing int32, sigma float32) (b *Background) {
	// Allocate space for gradient cells
	height:=int32(len(src)/int(width))
	numCellCols:=(width+gridSpacing-1)/gridSpacing
	numCellRows:=(height+gridSpacing-1)/gridSpacing
	numCells   :=numCellCols*numCellRows
	cells      :=make([]BGCell, numCells)

	buffer:=make([]float32, gridSpacing*gridSpacing) // reuse for all grid cells to ease GC pressure

	// For all grid cells
	c:=0
	for yStart:=int32(0); yStart<height; yStart+=gridSpacing {
		yEnd:=yStart+gridSpacing
		if yEnd>height { yEnd=height }

		for xStart:=int32(0); xStart<width; xStart+=gridSpacing {
			xEnd:=xStart+gridSpacing
			if xEnd>width { xEnd=width }

			// Fit linear gradient to masked source image within that cell
			cells[c].Fit(src, width, sigma, xStart, xEnd, yStart, yEnd, buffer)
			c++
		}	
	}	

	buffer=nil

	return &Background{width, height, gridSpacing, cells}
}


// Render full background into a data array, returning the array
func (b Background) Render() (dest []float32) {
	dest=make([]float32, b.Width*b.Height)

	// For all grid cells
	c:=0
	for yStart:=int32(0); yStart<b.Height; yStart+=b.GridSpacing {
		yEnd:=yStart+b.GridSpacing
		if yEnd>b.Height { yEnd=b.Height }

		for xStart:=int32(0); xStart<b.Width; xStart+=b.GridSpacing {
			xEnd:=xStart+b.GridSpacing
			if xEnd>b.Width { xEnd=b.Width }

			// Render linear gradient cell into the destination image
			b.Cells[c].Render(dest, b.Width, xStart, xEnd, yStart, yEnd)
			c++
		}	
	}	

	return dest
}


// Subtract full background from given data array, changing it in place.
func (b Background) Subtract(dest []float32) {
	if int(b.Width)*int(b.Height)!=len(dest) { 
		LogFatalf("Background size %dx%d does not match destination image size %d\n", b.Width, b.Height, len(dest))
	}

	// For all grid cells
	c:=0
	for yStart:=int32(0); yStart<b.Height; yStart+=b.GridSpacing {
		yEnd:=yStart+b.GridSpacing
		if yEnd>b.Height { yEnd=b.Height }

		for xStart:=int32(0); xStart<b.Width; xStart+=b.GridSpacing {
			xEnd:=xStart+b.GridSpacing
			if xEnd>b.Width { xEnd=b.Width }

			// Subtract linear gradient cell from destination image
			b.Cells[c].Subtract(dest, b.Width, xStart, xEnd, yStart, yEnd)
			c++
		}	
	}	
}


// Fit background cell to given source image, except where masked out
// FIXME: what to do if entire cell masked out?
func (cell *BGCell) Fit(src []float32, width int32, sigma float32, xStart, xEnd, yStart, yEnd int32, buffer []float32) {
	// Key idea: the x scale factor alpha, the Y scale factor beta and the constant offset gamma are linearly independent
	// So we can choose optimal values for each independently
	// Let's take the median absolute difference as the error function to minimize

	// First we determine the local background location and the scale of its noise level, to filter out stars and bright nebulae
	median, mad:=medianAndMAD(src, width, xStart, xEnd, yStart, yEnd, buffer)
	upperBound:=median+sigma*mad

	// Let's calculate the median of the non-masked left half of the data, and the median of the non-masked right half
	// The difference of these two, divided by half the grid spacing, gives the x scale factor alpha
	xHalf:=(xStart+xEnd)>>1
	leftMedian :=trimmedMedian(src, width, upperBound, xStart, xHalf, yStart, yEnd, buffer)
	rightMedian:=trimmedMedian(src, width, upperBound, xHalf,  xEnd,  yStart, yEnd, buffer)
	cell.Alpha=2.0*(rightMedian-leftMedian)/float32(xEnd-xStart)

	// Analogously for beta, just using the upper and lower half
	yHalf:=(yStart+yEnd)>>1
	upperMedian:=trimmedMedian(src, width, upperBound, xStart, xEnd, yStart, yHalf, buffer)
	lowerMedian:=trimmedMedian(src, width, upperBound, xStart, xEnd, yHalf,  yEnd,  buffer)
	cell.Beta=2.0*(lowerMedian-upperMedian)/float32(yEnd-yStart)

	// Using the median of the non-masked data as gamma minimizes constant error across the cell
	overallMedian:=trimmedMedian(src, width, upperBound, xStart, xEnd, yStart, yEnd, buffer)
	cell.Gamma=overallMedian
}


// Calculates the median and the MAD of the given grid cell of the image
func medianAndMAD(src []float32, width int32, xStart, xEnd, yStart, yEnd int32, buffer []float32) (median, mad float32) {
	numSamples:=0
	for y:=yStart; y<yEnd; y++ {
		for x:=xStart; x<xEnd; x++ {
			offset:=x+y*width
			buffer[numSamples]=src[offset]
			numSamples++
		}
	}
	buffer=buffer[:numSamples]
	median=QSelectMedianFloat32(buffer)
	for i, b:=range buffer { buffer[i]=float32(math.Abs(float64(b - median))) }
	mad=QSelectMedianFloat32(buffer)*1.4826 // factor normalizes MAD to Gaussian standard deviation
	return median, mad	
}


// Calculates the median of all values below the upper bound in the given grid cell of the image
func trimmedMedian(src []float32, width int32, upperBound float32, xStart, xEnd, yStart, yEnd int32, buffer []float32) float32 {
	numSamples:=0
	for y:=yStart; y<yEnd; y++ {
		for x:=xStart; x<xEnd; x++ {
			value:=src[x+y*width]
			if value>=upperBound { continue }
			buffer[numSamples]=value
			numSamples++
		}
	}
	return QSelectMedianFloat32(buffer[:numSamples])	
}


// Render background cell into given window of the given destination image
func (c *BGCell) Render(dest []float32, width int32, xStart, xEnd, yStart, yEnd int32) {
	for y:=yStart; y<yEnd; y++ {
		for x:=xStart; x<xEnd; x++ {
			dest[x+y*width]=c.EvalAt(x-((xStart+xEnd)>>1), y-((yStart+yEnd)>>1))
		}
	}
}


// Subtracts background cell from given window of the given destination image, changing it in place
func (c *BGCell) Subtract(dest []float32, width int32, xStart, xEnd, yStart, yEnd int32) {
	for y:=yStart; y<yEnd; y++ {
		for x:=xStart; x<xEnd; x++ {
			dest[x+y*width]-=c.EvalAt(x-((xStart+xEnd)>>1), y-((yStart+yEnd)>>1))
		}
	}
}


// Evaluate background cell at given position
func (c *BGCell) EvalAt(x, y int32) float32 {
	return c.Alpha*float32(x) + c.Beta*float32(y) + c.Gamma
}