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

package fits

import (
	"math"
)

// From https://bottosson.github.io/posts/oklab/ (MIT License / heavily adapted)

type LinearsRGB struct {
	R float32
	G float32
	B float32
}

type OkLab struct {
	L float32
	A float32
	B float32
}

func (c LinearsRGB) ToOkLab() OkLab {
	r:=c.R
	if r<0 { r=0 }
	if r>1 { r=1 }

	g:=c.G
	if g<0 { g=0 }
	if g>1 { g=1 }

	b:=c.B
	if b<0 { b=0 }
	if b>1 { b=1 }

    l := 0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b;
	m := 0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b;
	s := 0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b;

    l3 := cbrtf(l);
    m3 := cbrtf(m);
    s3 := cbrtf(s);

    return OkLab{
        0.2104542553*l3 + 0.7936177850*m3 - 0.0040720468*s3,
        1.9779984951*l3 - 2.4285922050*m3 + 0.4505937099*s3,
        0.0259040371*l3 + 0.7827717662*m3 - 0.8086757660*s3,
    }
}

func (c OkLab) ToLinearsRGB() LinearsRGB {
    l3 := c.L + 0.3963377774 * c.A + 0.2158037573 * c.B;
    m3 := c.L - 0.1055613458 * c.A - 0.0638541728 * c.B;
    s3 := c.L - 0.0894841775 * c.A - 1.2914855480 * c.B;

    l := l3*l3*l3;
    m := m3*m3*m3;
    s := s3*s3*s3;

    return LinearsRGB{
		+4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
		-1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
		-0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s,
    };
}

type OkHcl struct {
	H float32
	C float32
	L float32
}

func (c OkLab) ToOkHcl() OkHcl {
	cc:=sqrt(c.A*c.A + c.B*c.B)
	h:=atan2(c.B, c.A)*(180.0/float32(math.Pi))
	return OkHcl{h, cc, c.L}
}

func (c OkHcl) ToOkLab() OkLab {
	h:=(float32(math.Pi)/180.0)*c.H
	a:=c.C*cos(h)
	b:=c.C*sin(h)
	return OkLab{c.L, a, b}
}


func (c LinearsRGB) ToOkHcl()      OkHcl      { return c.ToOkLab().ToOkHcl()      }
func (c OkHcl)      ToLinearsRGB() LinearsRGB { return c.ToOkLab().ToLinearsRGB() }


// square root function
func sqrt(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}

// cube root function
func cbrtf(x float32) float32 {
	return float32(math.Cbrt(float64(x)))
}

// sine function
func sin(x float32) float32 {
	return float32(math.Sin(float64(x)))
}

// cosine function
func cos(x float32) float32 {
	return float32(math.Cos(float64(x)))
}

// Atan2 function
func atan2(x,y float32) float32 {
	return float32(math.Atan2(float64(x), float64(y)))
}

