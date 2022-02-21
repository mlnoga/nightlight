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
	"math"
	"sort"
	"gonum.org/v1/gonum/optimize"   // for alignment. source via "go get gonum.org/v1/gonum"
									// also needs "go get golang.org/x/exp/rand"
									// also needs "go get golang.org/x/tools/container/intsets"
)

// A star aligner
type Aligner struct {
	Naxisn		 []int32      // Size of the destination image we are aligning to
	RefStars     []Star       // The reference stars this aligner uses
	Stars2DT     KDTree2      // Pointerless 2-dimensional tree  for fast lookup of reference stars
	RefTriangles []Triangle   // Reference triangles built from the above, using the k constant
	RefTri3DT    KDTree3P     // Pointerless 3-dimensional tree for fast lookup of reference triangles
	K            int32        // Consider top k brightest stars for building triangles
}

// A triangle representing the distances between three stars, which are translation and rotation invariant.
// Also stores their indices into the Stars[] array for later processing steps.
type Triangle struct {
	DistAB	float32
	DistAC  float32
	DistBC	float32
	A       int32
	B       int32
	C       int32   
}

// A candidate match between a triangle and a reference triangle, with distance between them
type Match struct {
	Dist        float32
	TriIndex    int32
	RefTriIndex int32
} 

const minDistanceForAlignmentStars float32 = 1.0/20.0

// Creates a new star aligner from the given reference stars and priming constant k
func NewAligner(naxisn []int32, refStars []Star, k int32) *Aligner {
	var kdt2 KDTree2 =make([]Point2D, len(refStars))
	for i,s:=range refStars { kdt2[i]=Point2D{s.X, s.Y} }
	kdt2.Make()

	minLength:=float32(naxisn[1])*minDistanceForAlignmentStars
	indices:=pickBrightestDistant(refStars, minLength, k)
	tris:=generateTriangles(refStars, indices, 1.0)	
	var trisKDT3 KDTree3P = make([]Point3DPayload, len(tris))
	for i,s:=range tris { trisKDT3[i]=Point3DPayload{Point3D{s.DistAB, s.DistAC, s.DistBC}, interface{}(int32(i)) } }
	trisKDT3.Make()

	return &Aligner{naxisn, refStars, kdt2, tris, trisKDT3, k}
}

// Calculates image alignments based on their respective star positions
func (a *Aligner) Align(naxisn []int32, stars []Star, id int) (trans Transform2D, residual float32) {
	minLength:=float32(a.Naxisn[1])*minDistanceForAlignmentStars
	indices:=pickBrightestDistant(stars, minLength, a.K)
	//LogPrintf("%d: Picked the %d brightest stars with distance greater %f.\n", id, len(indices), minLength)
	triangles:=generateTriangles(stars, indices, float32(a.Naxisn[0])/float32(naxisn[0]))
	//LogPrintf("%d: Built %d triangles from the %d brightest stars of the %d overall.\n", id, len(triangles), a.K, len(stars))
	matches:=a.closestTriangleMatches(triangles)
	trans, residual=a.findBestMatch(matches, triangles, stars, id)
	return trans, residual
}

// Selects the k brightest stars, skipping those closer than limit to an already selected star. Returns indices into stars
func pickBrightestDistant(stars []Star, minLength float32, k int32) (indices []int) {
	indices=make([]int, k)
	i:=0
	s:=0
	outer:
	for ; i<len(indices) && s<len(stars); s++ {
		starA:=stars[s]
		for j:=0; j<i; j++ {
			starB:=stars[indices[j]]
			dAB:=Dist2D(Point2D{starA.X, starA.Y}, Point2D{starB.X, starB.Y})
			if dAB<minLength {
					continue outer
			}
		}
		indices[i]=s
		i++
	}
	return indices[0:i]
}

// Generates all possible triangles from the list of brightest star indices provided.
// This is O(K^3), pretty wasteful, generates all n=6 permutation of the triangle points as separate triangles.
func generateTriangles(stars []Star, indices []int, scaleFactor float32) []Triangle {
	tris:=[]Triangle{}
	for _, a := range indices {
		starA:=stars[a]
		for _, b :=range indices {
			if a==b { continue }
			starB:=stars[b]
			dAB:=Dist2D(Point2D{starA.X*scaleFactor, starA.Y*scaleFactor}, Point2D{starB.X*scaleFactor, starB.Y*scaleFactor})
			for _, c :=range indices {
				if a==c || b==c { continue }
				starC:=stars[c]
				dAC:=Dist2D(Point2D{starA.X*scaleFactor, starA.Y*scaleFactor}, Point2D{starC.X*scaleFactor, starC.Y*scaleFactor})
				dBC:=Dist2D(Point2D{starB.X*scaleFactor, starB.Y*scaleFactor}, Point2D{starC.X*scaleFactor, starC.Y*scaleFactor})

				if(dAB<dAC && dAC<dBC) {
					tri:=Triangle{dAB, dAC, dBC, int32(a), int32(b), int32(c) }	
					tris=append(tris, tri)
				}
			}
		}
	}
	return tris
}

// Finds the closest matches between the given triangles and the reference triangles
func (a *Aligner) closestTriangleMatches(triangles []Triangle) (matches []Match) {
	kdt:=a.RefTri3DT
	matches=make([]Match, len(triangles))

	for i, tri := range triangles {
		pt   :=Point3D{tri   .DistAB, tri   .DistAC, tri   .DistBC}
		closest, distSquared:=kdt.NearestNeighbor(pt)
		matches[i]=Match{distSquared, int32(i), closest.Payload.(int32) }
	}

	// FIXME: wasteful, should use extract to find the k-th smallest element
	sort.Slice(matches[:], func(i, j int) bool {
		return matches[i].Dist < matches[j].Dist
	} )

	k:=a.K
	if k>int32(len(matches)) { k=int32(len(matches)) }
	shortlist:=make([]Match, k)
	for i, _:=range(shortlist) {
		m:=matches[i]
		shortlist[i]=Match{m.Dist, m.TriIndex, m.RefTriIndex}
	}
	return shortlist
}


func (a *Aligner) findBestMatch(matches []Match, triangles []Triangle, stars []Star, id int) (trans Transform2D, residual float32) {
	bestTrans:=Transform2D{}
	bestResidualError:=float32(math.MaxFloat32)
	refTriangles, refStars:=a.RefTriangles, a.RefStars

	distSquaredLimit:=float32(8.0*8.0)         // Distance limit to consider a star a match
	earlyAbortForResidualError:=float32(0.01)  // Stop further search if a global match closer than this is found

	for _, match:=range(matches) {
		// Build initial transformation based on the triples of stars in the match
		tri:=triangles[match.TriIndex]
		p1:=Point2D{stars[tri.A].X, stars[tri.A].Y}
		p2:=Point2D{stars[tri.B].X, stars[tri.B].Y}
		p3:=Point2D{stars[tri.C].X, stars[tri.C].Y}
		refTri:=refTriangles[match.RefTriIndex]
		p1p:=Point2D{refStars[refTri.A].X, refStars[refTri.A].Y}
		p2p:=Point2D{refStars[refTri.B].X, refStars[refTri.B].Y}
		p3p:=Point2D{refStars[refTri.C].X, refStars[refTri.C].Y}
		trans, err:=NewTransform2D(p1, p2, p3, p1p, p2p, p3p)
		if err!=nil { continue }

		// Print some stats about the transformation candidate found
		//if id==0 {
		//	LogPrintf("Match %d dist %.6g: Based on tri %d [%d,%d,%d] -> refTri %d [%d,%d,%d]:\n", 
		//		i, match.Dist, match.TriIndex, tri.A, tri.B, tri.C, match.RefTriIndex, refTri.A, refTri.B, refTri.C)
		//	LogPrintf("Coords [%v, %v, %v] -> [%v, %v, %v]\n", p1, p2, p3, p1p, p2p, p3p) 
		//	LogPrintf("Deltas [%v, %v, %v]\n", Sub2D(p1,p1p), Sub2D(p2,p2p), Sub2D(p3,p3p))
		//	p1t:=trans.Apply(p1)
		//	p2t:=trans.Apply(p2)
		//	p3t:=trans.Apply(p3)
		//	LogPrintf("Proj   [%v, %v, %v] Deltas [%v, %v, %v]\n", p1t, p2t, p3t, Sub2D(p1t,p1p), Sub2D(p2t,p2p), Sub2D(p3t,p3p))
		//	LogPrintf("Trans  %s\n", trans)
		//}

		// Identify all projected stars which have reasonably close matches to reference stars
		numMatches:=0
		refPoints:=make([]Point2D, len(stars))
		for id, star:=range stars {
			p:=Point2D{star.X, star.Y}
			proj:=trans.Apply(p)
			refPoint, distSquared:=a.Stars2DT.NearestNeighbor(proj)
			if distSquared<distSquaredLimit {
				refPoints[id]=refPoint
				numMatches++
			} else {
				refPoints[id]=Point2D{float32(math.NaN()), float32(math.NaN())}
			}
		}
		//if id==0 {
		//	LogPrintf("Match %d numStarsMatched %d totalStarsMatched %d\n", i, numMatches, len(stars))
		//}
		if numMatches<len(stars)/3 { // abort if fewer than a third of the stars matched
			continue;
		}

        // Minimize the distance between projected stars and their reference counterparts 
        x0:=[]float64{float64(trans.A), float64(trans.B), float64(trans.C), float64(trans.D), float64(trans.E), float64(trans.F)}
        problem := optimize.Problem{
			Func:func(x []float64) float64 {
				tr:=Transform2D{float32(x[0]), float32(x[1]), float32(x[2]), float32(x[3]), float32(x[4]), float32(x[5])}

				starsMatched    :=int32(0)      
				distSquaredSum  :=float32(0)
				for id,star:=range stars {
					p:=Point2D{star.X, star.Y}
					proj:=tr.Apply(p)

					refPoint:=refPoints[id]
					if !math.IsNaN(float64(refPoint.X)) {
						distSquared:=Dist2DSquared(proj, refPoint)
						distSquaredSum+=distSquared
						starsMatched++
					}
		        }
		        return math.Sqrt(float64(distSquaredSum))/float64(starsMatched)
			},			
		}
		result, err := optimize.Minimize(problem, x0, nil, &optimize.NelderMead{})
		if err!= nil {
			//LogPrintf("optimizer error: %s\n", err.Error())
			continue
		}

		x:=result.X
		trans=Transform2D{float32(x[0]), float32(x[1]), float32(x[2]), float32(x[3]), float32(x[4]), float32(x[5])}
		residualError:=float32(result.F)
		// Update best solution found, if applicable
		if residualError<bestResidualError {
			bestTrans=trans
			bestResidualError=residualError

			if bestResidualError<earlyAbortForResidualError { 
				return bestTrans, bestResidualError
			}
		}
	}

	return bestTrans, bestResidualError
}


func (a *Aligner) calcDist(stars []Star, tr Transform2D) (starsMatched int32, dist float32) {
	distSquaredLimit:=float32(8.0*8.0)  // Distance limit to consider this a match. FIXME: arbitrary!!
	starsMatched=int32(0)
	distSquaredSum:=float32(0)
	for _,star:=range stars {
		p:=Point2D{star.X, star.Y}
		proj:=tr.Apply(p)

		_, distSquared:=a.Stars2DT.NearestNeighbor(proj)

		if distSquared<distSquaredLimit {
			starsMatched++
			distSquaredSum+=distSquared
		} 
    }
    return starsMatched, float32(math.Sqrt(float64(distSquaredSum)/float64(starsMatched)))
}