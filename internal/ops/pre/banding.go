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
	"encoding/json"
	"fmt"
	"math"
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/qsort"
	"github.com/mlnoga/nightlight/internal/ops"
)


type OpDebandHoriz struct {
	ops.OpUnaryBase
	Percentile      float32     `json:"percentile"`
	Window          int32       `json:"window"`
	Sigma           float32     `json:"sigma"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpDebandHorizDefaults() })} // register the operator for JSON decoding

func NewOpDebandHorizDefaults() *OpDebandHoriz { return NewOpDebandHoriz(50, 128, 3.0) }

func NewOpDebandHoriz(percentile float32, window int32, sigma float32) *OpDebandHoriz {
	op:=&OpDebandHoriz{
		OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "debandHoriz"}},
		Percentile  : percentile,
		Window      : window,
		Sigma       : sigma,
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpDebandHoriz) UnmarshalJSON(data []byte) error {
	type defaults OpDebandHoriz
	def:=defaults( *NewOpDebandHorizDefaults() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpDebandHoriz(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpDebandHoriz) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.Percentile<=0 || op.Percentile>=100 || op.Window<=0 { return f, nil }

	// obtain dimensions
	numCols, numRows   :=f.Naxisn[0], f.Naxisn[1]
	windowRows:=op.Window
	if windowRows>numRows { windowRows=numRows }

	// allocate space
	rowPercentiles     :=make([]float32, numRows)
	rowPercentilesClone:=make([]float32, windowRows)
	rowBuffer          :=make([]float32, numCols)

	// calculate threshold from global location and scale, if sigma is present
	threshold:=float32(math.MaxFloat32)
	if op.Sigma!=0 {
		loc, scale:=f.Stats.Location(), f.Stats.Scale()
		threshold=loc + op.Sigma*scale
	}

	// calculate desired percentile of each row, excluding values above the threshold
	for row:=int32(0); row<numRows; row++ {
		ds:=f.Data[row*numCols:row*numCols+numCols]
		numSamples:=0
		for _,d:=range(ds) {
			if d<=threshold {
				rowBuffer[numSamples]=d
				numSamples++
			}
		}
		k:=int(float32(numSamples)*op.Percentile*0.01)
		rowPercentiles[row]=qsort.QSelectFloat32(rowBuffer[:numSamples],int(k))
	}

	// correct each row
	lowest, highest:=float32(1), float32(0)
	for row:=int32(0); row<numRows; row++ {
		// determine local window and calculate local median of percentiles
		startRow:=row-(windowRows>>1)
		missing:=int32(0)
		if startRow<0 { 
			missing = startRow
			startRow = 0 
		}
		endRow:=startRow+windowRows
		if endRow>numRows {
			missing = endRow - numRows
			endRow=numRows
			startRow=endRow-windowRows
		}
		copy(rowPercentilesClone, rowPercentiles[startRow:endRow])
		if missing!=0 {
			fixWindowEdge(rowPercentilesClone, missing)
		}
		medianOfRowPercentiles:=qsort.QSelectMedianFloat32(rowPercentilesClone)

		// calculate local correction factor
		factor:=medianOfRowPercentiles / rowPercentiles[row]
		if factor<lowest { lowest = factor }
		if factor>highest { highest = factor }

		// apply local correction factor
		theRow:=f.Data[row*numCols:row*numCols+numCols]
		for col, v:=range(theRow) {
			theRow[col]=v*factor
		}
	}
	f.Stats.Clear()
	fmt.Fprintf(c.Log, "%d: De-banded horizontally with %.3fth percentile, window %d, sigma %.2f, threshold %.2f, factors in [%.3f, %.3f]\n", 
		        f.ID, op.Percentile, op.Window, op.Sigma, threshold, lowest, highest)
	return f, nil
}

func fixWindowEdge(window []float32, missing int32) {
  // calculate medians of the left and right halves of the window
  left:=make([]float32, len(window)/2)
  copy(left, window[:len(left)])
  leftMedian:=qsort.QSelectMedianFloat32(left)

  right:=make([]float32, len(window)-len(left))
  copy(right, window[len(left):])
  rightMedian:=qsort.QSelectMedianFloat32(right)

  // linearly approximate the gradient via mean-of-medians and slope-of-medians  
  meanOfMedians:=0.5*(leftMedian+rightMedian)
  center:=0.5*(float32(len(left))+float32(len(right)))
  slopeOfMedians:=(rightMedian-leftMedian)/center

  if missing<0 { 
  	// replace values on the right of the buffer with interpolated values left of buffer
  	for i:=int32(len(window))+missing; i<int32(len(window)); i++ {
  		offset:=float32(i-int32(len(window)))-center
  		window[i]=meanOfMedians+slopeOfMedians*offset
  	}
  } else {
  	// replace values on the left of the buffer with interpolated values right of buffer
  	for i:=int32(0); i<missing; i++ {
  		offset:=float32(i+int32(len(window)))-center
  		window[i]=meanOfMedians+slopeOfMedians*offset
  	}
  }
}

type OpDebandVert struct {
	ops.OpUnaryBase
	Percentile      float32     `json:"percentile"`
	Window          int32       `json:"window"`
	Sigma           float32     `json:"sigma"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpDebandVertDefaults() })} // register the operator for JSON decoding

func NewOpDebandVertDefaults() *OpDebandVert { return NewOpDebandVert(50, 128, 3.0) }

func NewOpDebandVert(percentile float32, window int32, sigma float32) *OpDebandVert {
	op:=&OpDebandVert{
		OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "debandVert"}},
		Percentile  : percentile,
		Window      : window,
		Sigma       : sigma,
	}
	op.OpUnaryBase.Apply=op.Apply // assign class method to superclass abstract method
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpDebandVert) UnmarshalJSON(data []byte) error {
	type defaults OpDebandVert
	def:=defaults( *NewOpDebandVertDefaults() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpDebandVert(def)
	op.OpUnaryBase.Apply=op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpDebandVert) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.Percentile<=0 || op.Percentile>=100 { return f, nil }

	// obtain dimensions
	numCols, numRows   :=f.Naxisn[0], f.Naxisn[1]
	windowCols:=op.Window
	if windowCols>numCols { windowCols=numCols }

	// allocate space
	colPercentiles     :=make([]float32, numCols)
	colPercentilesClone:=make([]float32, windowCols)
	colBuffer          :=make([]float32, numRows)

	// calculate threshold from global location and scale, if sigma is present
	threshold:=float32(math.MaxFloat32)
	if op.Sigma!=0 {
		loc, scale:=f.Stats.Location(), f.Stats.Scale()
		threshold=loc + op.Sigma*scale
	}

	// calculate desired percentile of each column
	for col:=int32(0); col<numCols; col++ {
		numSamples:=0
		for row:=int32(0); row<numRows; row++ {
			d:=f.Data[row*numCols + col]
			if d<=threshold {
				colBuffer[numSamples]=d
				numSamples++
			}
		}
		k:=int(float32(numSamples)*op.Percentile*0.01)
		colPercentiles[col]=qsort.QSelectFloat32(colBuffer[:numSamples],int(k))
	}

	// calculate median of percentiles
	copy(colPercentilesClone, colPercentiles)

	// apply correction to each column
	lowest, highest:=float32(1), float32(0)
	for col:=int32(0); col<numCols; col++ {
		// determine local window and calculate local median of percentiles
		startCol:=col-(windowCols>>1)
		missing:=int32(0)
		if startCol<0 { 
			missing = startCol
			startCol = 0 
		}
		endCol:=startCol+windowCols
		if endCol>numCols {
			missing = endCol - numCols
			endCol=numCols
			startCol=endCol-windowCols
		}
		copy(colPercentilesClone, colPercentiles[startCol:endCol])
		if missing!=0 {
			fixWindowEdge(colPercentilesClone, missing)
		}
		medianOfColPercentiles:=qsort.QSelectMedianFloat32(colPercentilesClone)

		// calculate local correction factor
		factor:=medianOfColPercentiles / colPercentiles[col]
		if factor<lowest { lowest = factor }
		if factor>highest { highest = factor }

		// apply local correction factor
		for row:=int32(0); row<numRows; row++ {
			f.Data[row*numCols + col] *= factor			
		}
	}
	f.Stats.Clear()
	fmt.Fprintf(c.Log, "%d: De-banded vertically with %.3fth percentile, window %d and sigma %.2f, threshold %.2f, factors in [%.3f, %.3f]\n", 
		        f.ID, op.Percentile, op.Window, op.Sigma, threshold, lowest, highest)
	return f, nil
}
