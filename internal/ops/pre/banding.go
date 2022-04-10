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
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/qsort"
	"github.com/mlnoga/nightlight/internal/ops"
)


type OpDebandHoriz struct {
	ops.OpUnaryBase
	Percentile      float32     `json:"percentile"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpDebandHorizDefaults() })} // register the operator for JSON decoding

func NewOpDebandHorizDefaults() *OpDebandHoriz { return NewOpDebandHoriz(50) }

func NewOpDebandHoriz(percentile float32) *OpDebandHoriz {
	op:=&OpDebandHoriz{
		OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "debandHoriz"}},
		Percentile  : percentile,
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
	if op.Percentile<=0 || op.Percentile>=100 { return f, nil }

	// allocate space
	numCols, numRows   :=f.Naxisn[0], f.Naxisn[1]
	rowPercentiles     :=make([]float32, numRows)
	rowPercentilesClone:=make([]float32, numRows)
	rowBuffer          :=make([]float32, numCols)

	// calculate desired percentile of each row
	k:=int(float32(numCols)*op.Percentile*0.01)
	for row:=int32(0); row<numRows; row++ {
		copy(rowBuffer, f.Data[row*numCols:row*numCols+numCols])
		rowPercentiles[row]=qsort.QSelectFloat32(rowBuffer,int(k))
	}

	// calculate median of percentiles
	copy(rowPercentilesClone, rowPercentiles)
	medianOfRowPercentiles:=qsort.QSelectMedianFloat32(rowPercentilesClone)

	// apply correction to each row
	lowest, highest:=float32(1), float32(0)
	for row:=int32(0); row<numRows; row++ {
		factor:=medianOfRowPercentiles / rowPercentiles[row]
		if factor<lowest { lowest = factor }
		if factor>highest { highest = factor }
		theRow:=f.Data[row*numCols:row*numCols+numCols]
		for col, v:=range(theRow) {
			theRow[col]=v*factor
		}
	}
	f.Stats.Clear()
	fmt.Fprintf(c.Log, "%d: De-banded horizontally with %.3fth percentile, factors in [%.3f, %.3f]\n", f.ID, op.Percentile, lowest, highest)
	return f, nil
}



type OpDebandVert struct {
	ops.OpUnaryBase
	Percentile      float32     `json:"percentile"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpDebandVertDefaults() })} // register the operator for JSON decoding

func NewOpDebandVertDefaults() *OpDebandVert { return NewOpDebandVert(50) }

func NewOpDebandVert(percentile float32) *OpDebandVert {
	op:=&OpDebandVert{
		OpUnaryBase : ops.OpUnaryBase{OpBase : ops.OpBase{Type: "debandVert"}},
		Percentile  : percentile,
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

	// allocate space
	numCols, numRows:=f.Naxisn[0], f.Naxisn[1]
	colPercentiles      :=make([]float32, numCols)
	colPercentilesClone :=make([]float32, numCols)
	colBuffer       :=make([]float32, numRows)

	// calculate desired percentile of each column
	k:=int(float32(numRows)*op.Percentile*0.01)
	for col:=int32(0); col<numCols; col++ {
		for row:=int32(0); row<numRows; row++ {
			colBuffer[row]=f.Data[row*numCols + col]
		}
		colPercentiles[col]=qsort.QSelectFloat32(colBuffer,int(k))
	}

	// calculate median of percentiles
	copy(colPercentilesClone, colPercentiles)
	medianOfColPercentiles:=qsort.QSelectMedianFloat32(colPercentilesClone)

	// apply correction to each column
	lowest, highest:=float32(1), float32(0)
	for col:=int32(0); col<numCols; col++ {
		factor:=medianOfColPercentiles / colPercentiles[col]
		if factor<lowest { lowest = factor }
		if factor>highest { highest = factor }
		for row:=int32(0); row<numRows; row++ {
			f.Data[row*numCols + col] *= factor			
		}
	}
	f.Stats.Clear()
	fmt.Fprintf(c.Log, "%d: De-banded vertically with %.3fth percentile, factors in [%.3f, %.3f]\n", f.ID, op.Percentile, lowest, highest)
	return f, nil
}
