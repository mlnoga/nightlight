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
	"sort"
)

// A kd-Tree with k=2 dimensions.
// Inspired by https://en.wikipedia.org/wiki/K-d_tree 
// Pointerless idea came up by itself ;)
type KDTree2 []Point2D


// Builds a pointerless k-dimensional tree with k=2 from the points by resorting the array.
// Function for even depths which pivots on the X dimension.
func (points KDTree2) Make() {
	sort.Slice(points, func(i, j int) bool {
		return points[i].X <= points[j].X
	} )

	l:=len(points)
	if l>1 { // descend left
		points[ :l/2].makeY()
		if l>2 { // descend right 
			points[l/2+1: ].makeY()
		}
	}
}

// Builds a pointerless k-dimensional tree with k=2 from the points by resorting the array.
// Helper function for odd depths which pivots on the Y dimension.
func (points KDTree2) makeY() {
	sort.Slice(points, func(i, j int) bool {
		return points[i].Y <= points[j].Y
	} )

	l:=len(points)
	if l>1 { // descend left
		points[ :l/2].Make()
		if l>2 { // descend right
			points[l/2+1: ].Make()
		}
	}
}

// Performs a nearest neighbor search on the points, which must have been previously transformed
// to a k-dimensional tree using NewKDTree2()
func (kdt KDTree2) NearestNeighbor(p Point2D) (closestPt Point2D, closestDsq float32) {
	l:=len(kdt)
	midpoint:=kdt[l/2]
	closestPt,closestDsq=midpoint, Dist2DSquared(p, midpoint)
	if p.X <= midpoint.X {
		if l>1 { // descend left
			pt, dsq := kdt[ :l/2].nearestNeighborY(p)
			if dsq<closestDsq { closestPt, closestDsq = pt, dsq }
			if l>2 { // descend right
				distToPlane:=p.X-midpoint.X
				if distToPlane*distToPlane<=closestDsq {
					pt, dsq := kdt[l/2+1:].nearestNeighborY(p)
					if dsq<closestDsq { closestPt, closestDsq = pt, dsq }
				}
			}
		}
	} else {
		if l>2 { // descend right
			pt, dsq := kdt[l/2+1:].nearestNeighborY(p)
			if dsq<closestDsq { closestPt, closestDsq = pt, dsq }
		}
		if l>1 { // descend left
			distToPlane:=p.X-midpoint.X
			if distToPlane*distToPlane<=closestDsq {
				pt, dsq := kdt[ :l/2].nearestNeighborY(p)
				if dsq<closestDsq { closestPt, closestDsq = pt, dsq }
			}
		}
	}
	return closestPt, closestDsq
}

func (kdt KDTree2) nearestNeighborY(p Point2D) (closestPt Point2D, closestDsq float32) {
	l:=len(kdt)
	midpoint:=kdt[l/2]
	closestPt,closestDsq=midpoint, Dist2DSquared(p, midpoint)
	if p.Y <= midpoint.Y {
		if l>1 { // descend left
			pt, dsq := kdt[ :l/2].NearestNeighbor(p)
			if dsq<closestDsq { closestPt, closestDsq = pt, dsq }
			if l>2 { // descend right
				distToPlane:=p.Y-midpoint.Y
				if distToPlane*distToPlane<=closestDsq {
					pt, dsq := kdt[l/2+1:].NearestNeighbor(p)
					if dsq<closestDsq { closestPt, closestDsq = pt, dsq }
				}
			}
		}
	} else {
		if l>2 { // descend right
			pt, dsq := kdt[l/2+1:].NearestNeighbor(p)
			if dsq<closestDsq { closestPt, closestDsq = pt, dsq }
		}
		if l>1 { // descend left
			distToPlane:=p.Y-midpoint.Y
			if distToPlane*distToPlane<=closestDsq {
				pt, dsq := kdt[ :l/2].NearestNeighbor(p)
				if dsq<closestDsq { closestPt, closestDsq = pt, dsq }
			}
		}
	}
	return closestPt, closestDsq
}
