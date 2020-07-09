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
)

// A piecewise linear background, for automated background extraction (ABE)
type Background struct {
	Width int32
	Height int32
	GridSpacing int32
	NumCellCols int32
	NumCellRows int32
	NumCells    int32	
 	Cells []float32
 	OutlierCells int32 // number of outlier cells replaced with interpolation of neighboring cells
 	Max float32         // maximum alpha, beta, gamma values
 	Min float32        // minimum alpha, beta, gamma values
}

func (bg *Background) String() string {
	return fmt.Sprintf("Background grid %d cells %dx%d outliers %d range [%f...%f]",
		bg.GridSpacing, bg.NumCellCols, bg.NumCellRows, bg.OutlierCells, 
		bg.Min, bg.Max )
}

// Creates new background by fitting linear gradients to grid cells of the given image, masking out areas in given mask
func NewBackground(src []float32, width int32, gridSpacing int32, sigma float32, backClip int32) (b *Background) {
	// Allocate space for gradient cells
	height:=int32(len(src)/int(width))
	numCellCols:=(width+gridSpacing-1)/gridSpacing
	numCellRows:=(height+gridSpacing-1)/gridSpacing
	numCells   :=numCellCols*numCellRows
	cells      :=make([]float32, numCells)

	b=&Background{Width:width, Height:height, GridSpacing:gridSpacing, NumCellCols:numCellCols, NumCellRows:numCellRows, NumCells:numCells, Cells:cells}

	b.init(src, sigma)
	if backClip>0 {
		b.clip(backClip)
	}
	b.smoothe()

	// For all grid cells
	/*LogPrintln("Final")
	c:=0
	for yStart:=int32(0); yStart<b.Height; yStart+=b.GridSpacing {
		yEnd:=yStart+b.GridSpacing
		if yEnd>b.Height { yEnd=b.Height }

		LogPrintf("%d:", yStart/b.GridSpacing)
		for xStart:=int32(0); xStart<b.Width; xStart+=b.GridSpacing {
			LogPrintf(" %.0f", b.Cells[c])
			c++
		}	
		LogPrintln()
	} */

    b.calculateStats()

	return b
}

// Initialize background by approximating each grid cell with a linear gradient
func (bg *Background) init(src []float32, sigma float32) {
	buffer:=make([]float32, bg.GridSpacing*bg.GridSpacing) // reuse for all grid cells to ease GC pressure

	// For all grid cells
	c:=0
	for yStart:=int32(0); yStart<bg.Height; yStart+=bg.GridSpacing {
		yEnd:=yStart+bg.GridSpacing
		if yEnd>bg.Height { yEnd=bg.Height }

		for xStart:=int32(0); xStart<bg.Width; xStart+=bg.GridSpacing {
			xEnd:=xStart+bg.GridSpacing
			if xEnd>bg.Width { xEnd=bg.Width }

			// Fit linear gradient to masked source image within that cell
			bg.Cells[c]=FitCell(src, bg.Width, sigma, xStart, xEnd, yStart, yEnd, buffer)
			c++
		}	
	}	

	/*LogPrintf("Sigma %f\n", sigma)
	// For all grid cells
	c=0
	LogPrintln("Median")
	for yStart:=int32(0); yStart<bg.Height; yStart+=bg.GridSpacing {
		LogPrintf("%d:", yStart/bg.GridSpacing)
		for xStart:=int32(0); xStart<bg.Width; xStart+=bg.GridSpacing {
			LogPrintf(" %.0f", bg.Cells[c])
			c++
		}	
		LogPrintln()
	}*/	

	buffer=nil
}

// Clips the top n entries from the background gradient
func (bg *Background) clip(n int32) {
	buffer:=make([]float32, bg.NumCells)
	for i,cell:=range bg.Cells { buffer[i]=cell }
	threshold:=QSelectFloat32(buffer, len(buffer)-int(n)+1)
	buffer=nil

	ignoredCells:=int32(0)
	for i,cell:=range bg.Cells { 
		if cell>=threshold {
			bg.Cells[i]=float32(math.NaN())
			ignoredCells++
		}
	}

	/*LogPrintf("n=%d: %d ignored cells based on threshold %f\n", n, ignoredCells, threshold)

	// For all grid cells
	c:=0
	LogPrintln("Clipped")
	for yStart:=int32(0); yStart<bg.Height; yStart+=bg.GridSpacing {
		LogPrintf("%d:", yStart/bg.GridSpacing)
		for xStart:=int32(0); xStart<bg.Width; xStart+=bg.GridSpacing {
			LogPrintf(" %.0f", bg.Cells[c])
			c++
		}	
		LogPrintln()
	}*/	


	bg.OutlierCells=ignoredCells

	// Then replace cells with interpolations
	for neighbors:=8; neighbors>=0; neighbors-- {
		numChanged:=1
		for numChanged>0 {
			numChanged=interpolate(bg.Cells, bg.NumCellCols, bg.NumCellRows, neighbors)
		}
	}
	buffer=nil
}

func (b *Background) smoothe() {
	tmp:=make([]float32, len(b.Cells))
	gauss3x3(tmp, b.Cells, b.NumCellCols)
	b.Cells=tmp
}

func gauss3x3(res, data []float32, width int32) {
	height:=int32(len(data))/width
	for y:=int32(0); y<height; y++ {
		for x:=int32(0); x<width; x++ {
			res[y*width+x]=gauss3x3Point(data, width, height, x,y)
		}
	}
}

//var gauss3x3Weights=[]float32{0.195346, 0.123317, 0.077847} // sigma 1.0
var gauss3x3Weights=[]float32{0.468592, 0.107973, 0.024879} // sigma 0.5

func gauss3x3Point(data []float32, width, height, x, y int32) float32 {
	runningSum:=float32(0)
	weightSum:=float32(0)

	for offY:=int32(-1); offY<=1; offY++ {
		for offX:=int32(-1); offX<=1; offX++ {
			x2, y2:=x+offX, y+offY
			if x2>=0 && x2<width && y2>=0 && y2<height {
				index:=x2+y2*width
				d:=data[index]
				weight:=gauss3x3Weights[offX*offX+offY*offY]
				runningSum+=d*weight
				weightSum+=weight
			}
		}
	}

	return runningSum/weightSum
}


func (bg *Background) calculateStats() {
	mf32:=float32(math.MaxFloat32)
	bg.Min= mf32
	bg.Max=-mf32
	for _,c:=range bg.Cells {
		if c<bg.Min { bg.Min=c }
		if c>bg.Max { bg.Max=c }
	}
}



// Smoothes a parameter
func interpolate(params []float32, width, height int32, neighbors int) (numChanges int) {
	temp:=[]float32{0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0}
	numChanges=0

    for y:=int32(0); y<height; y++ {
    	for x:=int32(0); x<width; x++ {
    		index:=y*width+x
    		p:=params[index]
    		if math.IsNaN(float64(p)) {
	    		predict, numGathered:=MedianInterpolation(params, width, height, x,y, temp)
	    		if numGathered>=neighbors {
	    			//LogPrintf("Replacing prediction for x%d y%d of %f with %f\n", x, y, p, predict)
	    			params[index]=predict
	    			numChanges++
	    		}
    		}
    	}
    }
    return numChanges
}

var interpolOffsets=[]pairOfint32{
	pairOfint32{-1,-1}, 
	pairOfint32{ 0,-1}, 
	pairOfint32{ 1,-1}, 
	pairOfint32{-1, 0}, 
	pairOfint32{ 1, 0}, 
	pairOfint32{-1, 1}, 
	pairOfint32{ 0, 1}, 
	pairOfint32{ 1, 1}, 
}

// Interpolate parameter from valid entries in local 1-neighborhood via median
func MedianInterpolation(params []float32, width, height, x,y int32, temp []float32) (median float32, numGathered int) {
	numGathered=0

	for _,off:=range interpolOffsets {
		x2, y2:=x+off.X, y+off.Y
		if x2>=0 && x2<width && y2>=0 && y2<=height {
			index:=x2+y2*width
			p:=params[index]
			if !math.IsNaN(float64(p)) {
				temp[numGathered]=p
				numGathered++
			}			
		}
	}

	median=MedianFloat32(temp[:numGathered])
	return median, numGathered
}	


// Render full background into a data array, returning the array
func (b Background) Render() (dest []float32) {
	dest=make([]float32, b.Width*b.Height)

	// For all grid cells
	subtrahend:=float32(b.GridSpacing)*0.5
	factor    :=1.0/float32(b.GridSpacing)
	for y:=int32(0); y<b.Height; y++ {
		ySrc:=(float32(y)-subtrahend)*factor
		for x:=int32(0); x<b.Width; x++ {
			xSrc:=(float32(x)-subtrahend)*factor

			// perform bilinear interpolation
			xl, yl:=int32(math.Floor(float64(xSrc))), int32(math.Floor(float64(ySrc)))
			xh, yh:=xl+1, yl+1

			if xl<0 {
				xl++
				xh++
			}
			if xh>=b.NumCellCols {
				xl--
				xh--
			}
			if yl<0 {
				yl++
				yh++
			}
			if yh>=b.NumCellRows {
				yl--
				yh--
			}
			xr, yr:=xSrc-float32(xl), ySrc-float32(yl)

			xlyl:=xl+yl*b.NumCellCols
			xhyl:=xlyl+1         // xh+yl*origWidth
			xlyh:=xlyl+b.NumCellCols // xl+yh*origWidth
			xhyh:=xhyl+b.NumCellCols // xh+yh*origWidth

			vyl  :=b.Cells[xlyl]*(1-xr) + b.Cells[xhyl]*xr
			vyh  :=b.Cells[xlyh]*(1-xr) + b.Cells[xhyh]*xr
			v    :=vyl    *(1-yr) + vyh    *yr

			//LogPrintf("x%d y%d xSrc%f ySrc%f xl%d yl%d xh%d yh%d v%f\n",
			//	x,y,xSrc,ySrc,xl,yl,xh,yh,v)
			dest[x + y*b.Width]=v
		}	
	}	

	return dest
}


// Subtract full background from given data array, changing it in place.
func (b Background) Subtract(dest []float32) {
	if int(b.Width)*int(b.Height)!=len(dest) { 
		LogFatalf("Background size %dx%d does not match destination image size %d\n", b.Width, b.Height, len(dest))
	}

	dest=make([]float32, b.Width*b.Height)

	// For all grid cells
	subtrahend:=float32(b.GridSpacing)*0.5
	factor    :=1.0/float32(b.GridSpacing)
	for y:=int32(0); y<b.Height; y++ {
		ySrc:=(float32(y)-subtrahend)*factor
		for x:=int32(0); x<b.Width; x++ {
			xSrc:=(float32(x)-subtrahend)*factor

			// perform bilinear interpolation
			xl, yl:=int32(math.Floor(float64(xSrc))), int32(math.Floor(float64(ySrc)))
			xh, yh:=xl+1, yl+1

			if xl<0 {
				xl++
				xh++
			}
			if xh>=b.NumCellCols {
				xl--
				xh--
			}
			if yl<0 {
				yl++
				yh++
			}
			if yh>=b.NumCellRows {
				yl--
				yh--
			}
			xr, yr:=xSrc-float32(xl), ySrc-float32(yl)

			xlyl:=xl+yl*b.NumCellCols
			xhyl:=xlyl+1         // xh+yl*origWidth
			xlyh:=xlyl+b.NumCellCols // xl+yh*origWidth
			xhyh:=xhyl+b.NumCellCols // xh+yh*origWidth

			vyl  :=b.Cells[xlyl]*(1-xr) + b.Cells[xhyl]*xr
			vyh  :=b.Cells[xlyh]*(1-xr) + b.Cells[xhyh]*xr
			v    :=vyl    *(1-yr) + vyh    *yr

			//LogPrintf("x%d y%d xSrc%f ySrc%f xl%d yl%d xh%d yh%d v%f\n",
			//	x,y,xSrc,ySrc,xl,yl,xh,yh,v)
			dest[x + y*b.Width]-=v
		}	
	}	
}


// Fit background cell to given source image, except where masked out
func FitCell(src []float32, width int32, sigma float32, xStart, xEnd, yStart, yEnd int32, buffer []float32) float32 {
	// First we determine the local background location and the scale of its noise level, to filter out stars and bright nebulae
	median, mad:=medianAndMAD(src, width, xStart, xEnd, yStart, yEnd, buffer)
	upperBound:=median+sigma*mad

	// Then we determine the trimmed median to approximate the true background
	overallMedian:=trimmedMedian(src, width, upperBound, xStart, xEnd, yStart, yEnd, buffer)
	return overallMedian
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
