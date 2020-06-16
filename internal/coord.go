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
 	"errors"
 	"fmt"
 	"math"
 )


// A 2-dimensional point with floating point coordinates.
type Point2D struct {
	X float32
	Y float32
}

// A 2-dimensional rectangle with floating point coordinates
type Rect2D struct {
	A Point2D
	B Point2D
}

// A 3-dimensional point with floating point coordinates.
type Point3D struct {
	X float32
	Y float32
	Z float32
}

// A 3-dimensional point with floating point coordinates and payload
type Point3DPayload struct {
	Point3D
	Payload interface{}
}

// A 2D coordinate transformation.
type Transform2D struct {
	A float32
	B float32
	C float32
	D float32
	E float32
	F float32
}

func (p Point2D) String() string {
	return fmt.Sprintf("(%.2f, %.2f)", p.X, p.Y)
}

func (r Rect2D) String() string {
	return fmt.Sprintf("(%v, %v)", r.A, r.B)
}

func (p Point3D) String() string {
	return fmt.Sprintf("(%.2f, %.2f, %.2f)", p.X, p.Y, p.Z)
}

func (t Transform2D) String() string {
	return fmt.Sprintf("x'=%.5gx %+.5gy %+.2g, y'=%.5gx %+.5gy %+.2g", 
		t.A, t.B, t.C, t.D, t.E, t.F)
}

// Returns the euclidian distance between the two given points
func Dist2D(a,b Point2D) float32 {
	dSquared:=Dist2DSquared(a,b)
	return float32(math.Sqrt(float64(dSquared)))
}

// Returns the squared euclididian distance between the two given points
func Dist2DSquared(a,b Point2D) float32 {
	dx, dy:=a.X-b.X, a.Y-b.Y
	return dx*dx + dy*dy
}

func Add2D(a,b Point2D) Point2D {
	return Point2D{a.X+b.X, a.Y+b.Y}
}

func Sub2D(a,b Point2D) Point2D {
	return Point2D{a.X-b.X, a.Y-b.Y}
}

// Returns the euclidian distance between the two given points
func Dist3D(a,b Point3D) float32 {
	dSquared:=Dist3DSquared(a,b)
	return float32(math.Sqrt(float64(dSquared)))
}

// Returns the squared euclididian distance between the two given points
func Dist3DSquared(a,b Point3D) float32 {
	dx, dy, dz:=a.X-b.X, a.Y-b.Y, a.Z-b.Z
	return dx*dx + dy*dy + dz*dz
}


func IdentityTransform2D() Transform2D {
	return Transform2D{1,0,0, 0,1,0}
}

// Calculate 2D transformation matrix from three given points in first coordinate
// system, and corresponding reference points in second coordinate system.
// p1, p2, p3 are in the first system. p1p, p2p, p3p are in the second. 
func NewTransform2D(p1, p2, p3, p1p, p2p, p3p Point2D) (Transform2D, error) {
	a:=( (p3p.X-p1p.X)*(p2.Y-p1.Y) - (p2p.X-p1p.X)*(p3.Y-p1.Y) ) /
	   ( (p2 .Y-p1 .Y)*(p3.X-p1.X) - (p2 .X-p1 .X)*(p3.Y-p1.Y) )

	b:=( (p2p.X-p1p.X) - a*(p2.X-p1.X) ) / (p2.Y-p1.Y)

	c:=p1p.X - a*p1.X - b*p1.Y

	d:=( (p3p.Y-p1p.Y)*(p2.Y-p1.Y) - (p2p.Y-p1p.Y)*(p3.Y-p1.Y) ) /
	   ( (p2 .Y-p1 .Y)*(p3.X-p1.X) - (p2 .X-p1 .X)*(p3.Y-p1.Y) )

	e:=( (p2p.Y-p1p.Y) - d*(p2.X-p1.X) ) / (p2.Y-p1.Y)

	f:=p1p.Y - d*p1.X - e*p1.Y   

	if math.IsInf(float64(a),0) || math.IsInf(float64(b),0) || math.IsInf(float64(d),0) || math.IsInf(float64(e),0) {
		return Transform2D{}, errors.New("divide by zero")
	} 
	return Transform2D{a,b,c,d,e,f}, nil
}


// Apply given 2D transformation to the given coordinates
func (t *Transform2D) Apply(p Point2D) (pP Point2D) {
	xP:=t.A*p.X + t.B*p.Y + t.C
	yP:=t.D*p.X + t.E*p.Y + t.F
	return Point2D{xP, yP}
}


// Apply given 2D transformation to many given coordinates
func (t *Transform2D) ApplySlice(ps []Point2D) (pPs []Point2D) {
	pPs=make([]Point2D, len(ps))
	for i, p := range(ps) {
		pPs[i]=t.Apply(p)
	}
	return pPs
}


// Invert a given 2D transformation. Returns error in the case of divid
func (t* Transform2D) Invert() (inv Transform2D, err error) {
	if epsilon:=t.B*t.D-t.A*t.E; epsilon<1e-8 && -epsilon<1e-8 {
		msg:=fmt.Sprintf("Matrix has no inverse, epsilon=%g", epsilon)
		return Transform2D{}, errors.New(msg)
	}
	return Transform2D{
		/*	x'    = a*x + b*y +c
			y'    = d*x + e*y +f

		    b*y   = x' - a*x - c
		    e*y   = y' - d*x - f

		    b*e*y = e*x' - a*e*x - c*e
		    b*e*y = b*y' - b*d*x - b*f

		    e*x' - a*e*x - c*e = b*y' - b*d*x - b*f
		    b*d*x - a*e*x      = b*y' - e*x' + c*e - b*f
		    x = -e/(b*d-a*e)*x' + b/(b*d-a*e)*y' + (c*e-b*f)/(b*d-a*e)

		    x = a'*x' + b'*y' + c'
		      with a'=-e/(b*d-a*e), b'=b/(b*d-a*e) and c'=(c*e-b*f)/(b*d-a*e)  */
		A: -t.E/(t.B*t.D-t.A*t.E),
		B:  t.B/(t.B*t.D-t.A*t.E),
		C: (t.C*t.E-t.B*t.F)/(t.B*t.D-t.A*t.E),
		/*	x'    = a*x + b*y +c
			y'    = d*x + e*y +f

		    a*x   = x' - b*y - c
		    d*x   = y' - e*y - f

		    a*d*x = d*x' - b*d*y - c*d
		    a*d*x = a*y' - a*e*y - a*f

		    d*x' - b*d*y - c*d = a*y' - a*e*y - a*f
		    a*e*y - b*d*y      = -d*x' + a*y' + c*d - a*f
		    y                  = -d/(a*e-b*d)*x' + a/(a*e-b*d)*y' + (c*d-a*f)/(a*e-b*d)
		    y =  d'*x' + e'+y* + f' 
		      with d'=-d/(a*e-b*d),  e'=a/(a*e-b*d) and f'=(c*d-a*f)/(a*e-b*d) */
		D: -t.D/(t.A*t.E-t.B*t.D),
		E:  t.A/(t.A*t.E-t.B*t.D),
		F: (t.C*t.D-t.A*t.F)/(t.A*t.E-t.B*t.D),
	}, nil
}