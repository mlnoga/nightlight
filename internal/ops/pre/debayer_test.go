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
	"testing"
)

func TestDebayerBilinearRGGBToRed(t *testing.T) {
	width, height:=int32(7), int32(11) 
	data:=make([]float32, width*height)
	sum:=0
	for i:=0; i<len(data); i++ {
		sum+=i
		data[i]=float32(sum)
	}

	rs, adjWidth:=DebayerBilinearRGGBToRed(data, width, 0, 0)
	if adjWidth!=(width&^1)  { t.Errorf("adjWidth=%d; want %d", adjWidth, (width&^1)) }
	if int32(len(rs))!=(width&^1)*(height&^1) { t.Errorf("len(rs)=%d; want %d", len(rs), (width&^1)*(height&^1)) }
	adjHeight:=int32(len(rs))/adjWidth
	for row:=int32(0); row<adjHeight; row+=2 {
		for col:=int32(0); col<adjWidth; col+=2 {
			if rs[row*adjWidth+col]!=data[row*width+col] { t.Errorf("rs[%d]=%f; want %f", row*adjWidth+col, rs[row*adjWidth+col], data[row*width+col]) }
			// FIXME: add other checks
		}
	}
}

func TestDebayerBilinearRGGBToGreen(t *testing.T) {
	width, height:=int32(11), int32(13) 
	data:=make([]float32, width*height)
	sum:=0
	for i:=0; i<len(data); i++ {
		sum+=i
		data[i]=float32(sum)
	}

	rs, adjWidth:=DebayerBilinearRGGBToGreen(data, width, 0, 0)
	if adjWidth!=(width&^1)  { t.Errorf("adjWidth=%d; want %d", adjWidth, (width&^1)) }
	if int32(len(rs))!=(width&^1)*(height&^1) { t.Errorf("len(rs)=%d; want %d", len(rs), (width&^1)*(height&^1)) }
	adjHeight:=int32(len(rs))/adjWidth
	for row:=int32(0); row<adjHeight; row+=2 {
		for col:=int32(0); col<adjWidth; col+=2 {
			if rs[row*adjWidth+col+1]!=data[row*width+col+1] { t.Errorf("rs[%d]=%f; want %f", row*adjWidth+col+1, rs[row*adjWidth+col+1], data[row*width+col+1]) }
			if rs[row*adjWidth+col+adjWidth]!=data[row*width+col+width] { t.Errorf("rs[%d]=%f; want %f", row*adjWidth+col+adjWidth, rs[row*adjWidth+col+adjWidth], data[row*width+col+width]) }
			// FIXME: add other checks
		}
	}
}

func TestDebayerBilinearRGGBToBlue(t *testing.T) {
	width, height:=int32(13), int32(7) 
	data:=make([]float32, width*height)
	sum:=0
	for i:=0; i<len(data); i++ {
		sum+=i
		data[i]=float32(sum)
	}

	rs, adjWidth:=DebayerBilinearRGGBToBlue(data, width, 0, 0)
	if adjWidth!=(width&^1)  { t.Errorf("adjWidth=%d; want %d", adjWidth, (width&^1)) }
	if int32(len(rs))!=(width&^1)*(height&^1) { t.Errorf("len(rs)=%d; want %d", len(rs), (width&^1)*(height&^1)) }
	adjHeight:=int32(len(rs))/adjWidth
	for row:=int32(0); row<adjHeight; row+=2 {
		for col:=int32(0); col<adjWidth; col+=2 {
			if rs[row*adjWidth+col+adjWidth+1]!=data[row*width+col+width+1] { t.Errorf("rs[%d]=%f; want %f", row*adjWidth+col+adjWidth+1, rs[row*adjWidth+col+adjWidth+1], data[row*width+col+width+1]) }
			// FIXME: add other checks
		}
	}
}

