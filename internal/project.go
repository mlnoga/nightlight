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

// Projects an image into a new coordinate system with the given transformation.
// Fills in missing pixels with the given out of bounds value. Uses bilinear interpolation for now.
func (img *FITSImage) Project(destNaxisn []int32, trans Transform2D, outOfBounds float32) (res *FITSImage, err error) {
	// Invert transformation so we can sample from the target coordinate system PoV
	invTrans,err:=trans.Invert()
	if err!=nil { return nil, err }

	// Create new FITS image for the result
	destWidth:=destNaxisn[0]
	destPixels:=destNaxisn[0]*destNaxisn[1]
	res=&FITSImage{
		ID    : img.ID,
		Header: NewFITSHeader(),
		Bitpix: -32,
		Bzero : 0,
		Naxisn: []int32{destNaxisn[0], destNaxisn[1]},
		Pixels: destPixels,
		Data:   make([]float32,int(destPixels)),
		Trans:  IdentityTransform2D(),
	}

	// Resample image from the target coordinate system PoV
	d:=img.Data
	origWidth:=img.Naxisn[0]

	for row:=int32(0); row<destNaxisn[1]; row++ {
		for col:=int32(0); col<destWidth; col++ {
			pt:=Point2D{float32(col), float32(row)}
			proj:=invTrans.Apply(pt)

			// perform bilinear interpolation
			xl, yl:=int32(math.Floor(float64(proj.X))), int32(math.Floor(float64(proj.Y)))
			xh, yh:=xl+1,               yl+1
			xr, yr:=proj.X-float32(xl), proj.Y-float32(yl)

			if xl<0 || xh>=origWidth || yl<0 || yh>=img.Naxisn[1] {
   				// Replace out of bounds values with not a number.
   				// Stacking will exclude NaNs. Note, however, that
   				// other operations will fail miserably. Including
   				// all partitioning and sorting-based operations 
   				// like median, because IEEE NaN does not compare
   				// equal to itself.  
   				res.Data[col + row*destWidth]=outOfBounds
   				continue 
			}

			xlyl:=xl+yl*origWidth
			xhyl:=xlyl+1         // xh+yl*origWidth
			xlyh:=xlyl+origWidth // xl+yh*origWidth
			xhyh:=xhyl+origWidth // xh+yh*origWidth

			vyl  :=d[xlyl]*(1-xr) + d[xhyl]*xr
			vyh  :=d[xlyh]*(1-xr) + d[xhyh]*xr
			v    :=vyl    *(1-yr) + vyh    *yr

			res.Data[col + row*destWidth]=v
		}
	}
	res.Stats=CalcBasicStats(res.Data)
	return res, nil
}
