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
	"gonum.org/v1/gonum/optimize"
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
func NewBackground(src, mask []float32, width int32, gridSpacing int32) (b *Background, err error) {
	// Allocate space for gradient cells
	height:=int32(len(src)/int(width))
	numCellCols:=(width+gridSpacing-1)/gridSpacing
	numCellRows:=(height+gridSpacing-1)/gridSpacing
	numCells   :=numCellCols*numCellRows
	cells      :=make([]BGCell, numCells)

	// For all grid cells
	c:=0
	for yStart:=int32(0); yStart<height; yStart+=gridSpacing {
		yEnd:=yStart+gridSpacing
		if yEnd>height { yEnd=height }

		for xStart:=int32(0); xStart<width; xStart+=gridSpacing {
			xEnd:=xStart+gridSpacing
			if xEnd>width { xEnd=width }

			// Fit linear gradient to masked source image within that cell
			err=cells[c].Fit(src, mask, width, xStart, xEnd, yStart, yEnd)
			if err!= nil { return nil, err }			
			c++
		}	
	}	

	return &Background{width, height, gridSpacing, cells}, nil
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


// Fit background cell to given source image, except where masked out
func (cell *BGCell) Fit(src, mask []float32, width int32, xStart, xEnd, yStart, yEnd int32) (err error) {
  	x0:=[]float64{float64(0), float64(0), float64(0)}
    problem := optimize.Problem{
		Func:func(x []float64) float64 {
			cell:=BGCell{float32(x[0]), float32(x[1]), float32(x[2])}	
			delta:=cell.Delta(src, mask, width, xStart, xEnd, yStart, yEnd)
	        return delta
		},			
	}
	result, err := optimize.Minimize(problem, x0, nil, &optimize.NelderMead{})
	cell.Alpha, cell.Beta, cell.Gamma=float32(result.X[0]), float32(result.X[1]), float32(result.X[2])
	return err
}


// Calculate delta between source image and background, except where masked out
func (c *BGCell) Delta(src, mask []float32, width int32, xStart, xEnd, yStart, yEnd int32) float64 {
	totalDeltas:=float64(0)
	for y:=yStart; y<yEnd; y++ {
		rowDeltas:=float64(0)
		for x:=xStart; x<xEnd; x++ {
			if mask!=nil && mask[x+y*width]!=0 { continue }
			value:=src[x+y*width]
			bgValue:=c.EvalAt(x, y)
			delta:=value-bgValue
			//if delta<0 { delta=-delta }
			rowDeltas+=float64(delta)*float64(delta)
		}
		totalDeltas+=rowDeltas
	}
	// FIXME: what to do when entire cell has been masked out? -> somehow interpolate from neighboring cells
	return totalDeltas
}


// Render background cell into given window of the given destination image
func (c *BGCell) Render(dest []float32, width int32, xStart, xEnd, yStart, yEnd int32) {
	for y:=yStart; y<yEnd; y++ {
		for x:=xStart; x<xEnd; x++ {
			dest[x+y*width]=c.EvalAt(x,y)
		}
	}
}


// Evaluate background cell at given position
func (c *BGCell) EvalAt(x, y int32) float32 {
	return c.Alpha*float32(x) + c.Beta*float32(y) + c.Gamma
}