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


package pre

import (
	"errors"
	"io"
	"fmt"
	"math"
	"strings"
	"github.com/mlnoga/nightlight/internal/qsort"
	"github.com/mlnoga/nightlight/internal/median"
)

// A piecewise linear background, for automated background extraction (ABE)
type Background struct {
	Width int32           // original image width
	Height int32          // original image height
	GridSpacing  int32    // approximate grid spacing as given by user
	GridSpacingX float32  // fine grid spacing for evenly sized cells, X direction
	GridSpacingY float32  // fine grid spacing for evenly sized cells, Y direction
	GridCellsX   int32    // number of grid cells, X direction
	GridCellsY   int32    // number of grid cells, Y direction
	GridCells    int32	  // number of grid cells, total = X * Y
 	Cells []float32       // grid cell values
 	OutlierCells int32    // number of outlier cells replaced with interpolation of neighboring cells
 	Max float32           // maximum alpha, beta, gamma values
 	Min float32           // minimum alpha, beta, gamma values
}

func (b *Background) String() string {
	return fmt.Sprintf("Background grid %d cells %dx%d outliers %d range [%f...%f]",
		b.GridSpacing, b.GridCellsX, b.GridCellsY, b.OutlierCells, 
		b.Min, b.Max )
}

func (b *Background) CellsString() string {
	sb:=&strings.Builder{}

	for y:=int32(0); y<b.GridCellsY; y++ {
		fmt.Fprintf(sb, "%2d:", y)
		for x:=int32(0); x<b.GridCellsX; x++ {
			c:=y*b.GridCellsX + x
			fmt.Fprintf(sb, " %4.0f", b.Cells[c])
		}	
		sb.WriteString("\n")
	} 
	return sb.String()
}

// Creates new background by fitting linear gradients to grid cells of the given image, masking out areas in given mask
func NewBackground(src []float32, width int32, gridSpacing int32, sigma float32, backClip int32, logWriter io.Writer) (b *Background) {
	// Allocate space for gradient cells
	height:=int32(len(src)/int(width))

	gridCellsX  :=(width+  gridSpacing/2) / gridSpacing
	gridCellsY  :=(height+ gridSpacing/2) / gridSpacing
	gridCells   :=gridCellsX*gridCellsY
	gridSpacingX:=float32(width )/float32(gridCellsX)
	gridSpacingY:=float32(height)/float32(gridCellsY)
	cells       :=make([]float32, gridCells)

	//LogPrintf("GridCells x %d y %d total %d GridSpacing x %.2f y %.2f\n", gridCellsX, gridCellsY, gridCells, gridSpacingX, gridSpacingY)
	b=&Background{Width:width, Height:height, GridSpacing:gridSpacing, 
	              GridSpacingX:gridSpacingX, GridSpacingY:gridSpacingY,
	              GridCellsX:gridCellsX, GridCellsY:gridCellsY, GridCells:gridCells, Cells:cells}

	b.init(src, sigma)
	//LogPrintf("Sigma %f\n", sigma)
	//LogPrintln(b.CellsString())

	if backClip>0 {
		b.clip(backClip, logWriter)
		//LogPrintf("Clip %d\n", backClip)
		//LogPrintln(b.CellsString())
	}

	b.smoothe()
	//LogPrintln("Smooth")
	//LogPrintln(b.CellsString())

    b.calculateStats()

	return b
}

// Initialize background by approximating each grid cell with a linear gradient
func (b *Background) init(src []float32, sigma float32) {
	buffer:=make([]float32, int32(b.GridSpacingX+1.5)*int32(b.GridSpacingY+1.5)) // reuse for all grid cells to ease GC pressure

	// For all grid cells
	for y:=int32(0); y<b.GridCellsY; y++ {
		yStart:=int32( float32(y)   *b.GridSpacingY +0.5)
		yEnd  :=int32((float32(y)+1)*b.GridSpacingY +0.5)
		if yEnd>b.Height { yEnd=b.Height }

		for x:=int32(0); x<b.GridCellsX; x++ {
			xStart:=int32( float32(x)   *b.GridSpacingX +0.5)
			xEnd  :=int32((float32(x)+1)*b.GridSpacingX +0.5)
			if xEnd>b.Width { xEnd=b.Width }

			//LogPrintf("y %d yS %d yE %d x %d xS %d xE %d \n", y, yStart, yEnd, x, xStart, xEnd)
			// Fit linear gradient to masked source image within that cell
			c:=y*b.GridCellsX + x
			b.Cells[c]=FitCell(src, b.Width, sigma, xStart, xEnd, yStart, yEnd, buffer)
		}	
	}	

	buffer=nil
}

// Clips the top n entries from the background gradient
func (b *Background) clip(n int32, logWriter io.Writer) {
	buffer:=make([]float32, b.GridCells)
	for i,cell:=range b.Cells { buffer[i]=cell }
	threshold:=qsort.QSelectFloat32(buffer, len(buffer)-int(n)+1)
	buffer=nil

	ignoredCells:=int32(0)
	for i,cell:=range b.Cells { 
		if cell>=threshold {
			b.Cells[i]=float32(math.NaN())
			ignoredCells++
		}
	}

	fmt.Fprintf(logWriter, "n=%d: %d ignored cells based on threshold %f\n", n, ignoredCells, threshold)
	//LogPrintln(b.CellsString())

	b.OutlierCells=ignoredCells

	// Then replace cells with interpolations
	for neighbors:=8; neighbors>=0; neighbors-- {
		numChanged:=1
		for numChanged>0 {
			numChanged=interpolate(b.Cells, b.GridCellsX, b.GridCellsY, neighbors)
		}
	}
	buffer=nil
}

func (b *Background) smoothe() {
	tmp:=make([]float32, len(b.Cells))
	gauss3x3(tmp, b.Cells, b.GridCellsX)
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
func MedianInterpolation(params []float32, width, height, x,y int32, temp []float32) (med float32, numGathered int) {
	numGathered=0

	for _,off:=range interpolOffsets {
		x2, y2:=x+off.X, y+off.Y
		if x2>=0 && x2<width && y2>=0 && y2<height {
			index:=x2+y2*width
			p:=params[index]
			if !math.IsNaN(float64(p)) {
				temp[numGathered]=p
				numGathered++
			}			
		}
	}

	med=median.MedianFloat32(temp[:numGathered])
	return med, numGathered
}	


// Render full background into a data array, returning the array
func (b Background) Render() (dest []float32) {
	dest=make([]float32, b.Width*b.Height)

	srcYl    :=int32(-1)
	srcYh    :=int32(0)
	destYl   :=int32(-0.5*b.GridSpacingY-0.5)
	destYh   :=int32( 0.5*b.GridSpacingY+0.5)
	destYSpan:=1.0/float32(destYh-destYl)

	for destY:=int32(0); destY<b.Height; destY++ {
		if destY>=destYh {
			srcYl    =srcYh
			srcYh    =srcYh+1
			destYl   =destYh
			destYh   =int32((float32(srcYh)+0.5)*b.GridSpacingY+0.5)
			destYSpan=1.0/float32(destYh-destYl)
		}
		srcY:=float32(srcYl)+float32(destY-destYl)*destYSpan

		//LogPrintf("dest yl %d y %d yh %d  src yl %d y %f yh %d\n", destYl, destY, destYh, srcYl, srcY, srcYh)

		srcXl    :=int32(-1)
		srcXh    :=int32(0)
		destXl   :=int32(-0.5*b.GridSpacingX-0.5)
		destXh   :=int32( 0.5*b.GridSpacingX+0.5)
		destXSpan:=1.0/float32(destXh-destXl)

		for destX:=int32(0); destX<b.Width; destX++ {
			if destX>=destXh {
				srcXl    =srcXh
				srcXh    =srcXh+1
				destXl   =destXh
				destXh   =int32((float32(srcXh)+0.5)*b.GridSpacingX+0.5)
				destXSpan=1.0/float32(destXh-destXl)
			}
			srcX:=float32(srcXl)+float32(destX-destXl)*destXSpan

			// perform bilinear interpolation
			xl, yl, xh, yh:=srcXl, srcYl, srcXh, srcYh

			if xl<0 {
				xl++
				xh++
			}
			if xh>=b.GridCellsX {
				xl--
				xh--
			}
			if yl<0 {
				yl++
				yh++
			}
			if yh>=b.GridCellsY {
				yl--
				yh--
			}
			xr, yr:=srcX-float32(xl), srcY-float32(yl)

			xlyl:=xl+yl*b.GridCellsX
			xhyl:=xlyl+1         // xh+yl*origWidth
			xlyh:=xlyl+b.GridCellsX // xl+yh*origWidth
			xhyh:=xhyl+b.GridCellsX // xh+yh*origWidth

			vyl  :=b.Cells[xlyl]*(1-xr) + b.Cells[xhyl]*xr
			vyh  :=b.Cells[xlyh]*(1-xr) + b.Cells[xhyh]*xr
			v    :=vyl    *(1-yr) + vyh    *yr

			//LogPrintf("x%d y%d xSrc%f ySrc%f xl%d yl%d xh%d yh%d v%f\n",
			//	x,y,xSrc,ySrc,xl,yl,xh,yh,v)
			dest[destX + destY*b.Width]=v
		}	
	}	

	return dest
}


// Subtract full background from given data array, changing it in place.
func (b Background) Subtract(dest []float32) error {
	if int(b.Width)*int(b.Height)!=len(dest) { 
		return errors.New(fmt.Sprintf("Background size %dx%d does not match destination image size %d\n", b.Width, b.Height, len(dest)))
	}

	srcYl    :=int32(-1)
	srcYh    :=int32(0)
	destYl   :=int32(-0.5*b.GridSpacingY-0.5)
	destYh   :=int32( 0.5*b.GridSpacingY+0.5)
	destYSpan:=1.0/float32(destYh-destYl)

	for destY:=int32(0); destY<b.Height; destY++ {
		if destY>=destYh {
			srcYl    =srcYh
			srcYh    =srcYh+1
			destYl   =destYh
			destYh   =int32((float32(srcYh)+0.5)*b.GridSpacingY+0.5)
			destYSpan=1.0/float32(destYh-destYl)
		}
		srcY:=float32(srcYl)+float32(destY-destYl)*destYSpan

		//LogPrintf("dest yl %d y %d yh %d  src yl %d y %f yh %d\n", destYl, destY, destYh, srcYl, srcY, srcYh)

		srcXl    :=int32(-1)
		srcXh    :=int32(0)
		destXl   :=int32(-0.5*b.GridSpacingX-0.5)
		destXh   :=int32( 0.5*b.GridSpacingX+0.5)
		destXSpan:=1.0/float32(destXh-destXl)

		for destX:=int32(0); destX<b.Width; destX++ {
			if destX>=destXh {
				srcXl    =srcXh
				srcXh    =srcXh+1
				destXl   =destXh
				destXh   =int32((float32(srcXh)+0.5)*b.GridSpacingX+0.5)
				destXSpan=1.0/float32(destXh-destXl)
			}
			srcX:=float32(srcXl)+float32(destX-destXl)*destXSpan

			// perform bilinear interpolation
			xl, yl, xh, yh:=srcXl, srcYl, srcXh, srcYh

			if xl<0 {
				xl++
				xh++
			}
			if xh>=b.GridCellsX {
				xl--
				xh--
			}
			if yl<0 {
				yl++
				yh++
			}
			if yh>=b.GridCellsY {
				yl--
				yh--
			}
			xr, yr:=srcX-float32(xl), srcY-float32(yl)

			xlyl:=xl+yl*b.GridCellsX
			xhyl:=xlyl+1         // xh+yl*origWidth
			xlyh:=xlyl+b.GridCellsX // xl+yh*origWidth
			xhyh:=xhyl+b.GridCellsX // xh+yh*origWidth

			vyl  :=b.Cells[xlyl]*(1-xr) + b.Cells[xhyl]*xr
			vyh  :=b.Cells[xlyh]*(1-xr) + b.Cells[xhyh]*xr
			v    :=vyl    *(1-yr) + vyh    *yr

			//LogPrintf("x%d y%d xSrc%f ySrc%f xl%d yl%d xh%d yh%d v%f\n",
			//	x,y,xSrc,ySrc,xl,yl,xh,yh,v)
			dest[destX + destY*b.Width]-=v
		}	
	}	
	return nil
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
	median=qsort.QSelectMedianFloat32(buffer)
	for i, b:=range buffer { buffer[i]=float32(math.Abs(float64(b - median))) }
	mad=qsort.QSelectMedianFloat32(buffer)*1.4826 // factor normalizes MAD to Gaussian standard deviation
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
	return qsort.QSelectMedianFloat32(buffer[:numSamples])	
}
