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
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/star"
)

// Calculate inner and outer bounding boxes for a set of lights.
func BoundingBoxes(lights []*fits.Image) (outer, inner star.Rect2D) {
	outer.A.X=float32( math.MaxFloat32)
	inner.A.X=float32(-math.MaxFloat32)
	inner.B.X=float32( math.MaxFloat32)
	outer.B.X=float32(-math.MaxFloat32)

	outer.A.Y=float32( math.MaxFloat32)
	inner.A.Y=float32(-math.MaxFloat32)
	inner.B.Y=float32( math.MaxFloat32)
	outer.B.Y=float32(-math.MaxFloat32)

	for id,lp := range lights {
		if lp==nil { continue }

		// Transform image coordiates into reference image coordinates
		localP1:=star.Point2D{0,0}
		localP2:=star.Point2D{float32(lp.Naxisn[0]), float32(lp.Naxisn[1])}
		p1, p2:=lp.Trans.Apply(localP1), lp.Trans.Apply(localP2)

		// Fix mirrors/rotations: ensure p1 has lower X and Y than p2
		if p1.X>p2.X { p1.X, p2.X = p2.X, p1.X }
		if p1.Y>p2.Y { p1.Y, p2.Y = p2.Y, p1.Y }

		LogPrintf("%d:bbox %v %v\n", id, p1, p2)

		// Update outer and inner bounding boxes
		if p1.X<outer.A.X { outer.A.X=p1.X }
		if p1.X>inner.A.X { inner.A.X=p1.X }
		if p2.X<inner.B.X { inner.B.X=p2.X }
		if p2.X>outer.B.X { outer.B.X=p2.X }

		if p1.Y<outer.A.Y { outer.A.Y=p1.Y }
		if p1.Y>inner.A.Y { inner.A.Y=p1.Y }
		if p2.Y<inner.B.Y { inner.B.Y=p2.Y }
		if p2.Y>outer.B.Y { outer.B.Y=p2.Y }
	}
	return outer, inner
}