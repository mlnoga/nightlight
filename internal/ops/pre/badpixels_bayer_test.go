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

func TestCosmeticCorrectionBayerRedOrBlue(t *testing.T) {
	width, height:=int32(13), int32(11) 
	data:=make([]float32, width*height)
	tmp :=make([]float32, width*height)

	for i:=0; i<len(data); i++ {
		data[i]=100+float32(i&3)
	}
	data[2*width+2]=500
	count:=CosmeticCorrectionBayerRedOrBlue(tmp, data, width, 0, 0, 3.0, 5.0)
	if count!=1 { t.Errorf("count=%d; want %d", count, 1) }
	if data[2*width+2]==500 { t.Errorf("data=%f; want not %f", data[2*width+2], float32(500)) }

	for i:=0; i<len(data); i++ {
		data[i]=100+float32(i&3)
	}
	data[2*width+2]=0
	count=CosmeticCorrectionBayerRedOrBlue(tmp, data, width, 0, 0, 3.0, 5.0)
	if count!=1 { t.Errorf("count=%d; want %d", count, 1) }
	if data[2*width+2]==0 { t.Errorf("data=%f; want not %f", data[2*width+2], float32(0)) }

	for i:=0; i<len(data); i++ {
		data[i]=100+float32(i&3)
	}
	data[4*width+4]=500
	count=CosmeticCorrectionBayerRedOrBlue(tmp, data, width, 0, 0, 3.0, 5.0)
	if count!=1 { t.Errorf("count=%d; want %d", count, 1) }
	if data[4*width+4]==500 { t.Errorf("data=%f; want not %f", data[4*width+4], float32(500)) }

	for i:=0; i<len(data); i++ {
		data[i]=100+float32(i&3)
	}
	data[4*width+4]=0
	count=CosmeticCorrectionBayerRedOrBlue(tmp, data, width, 0, 0, 3.0, 5.0)
	if count!=1 { t.Errorf("count=%d; want %d", count, 1) }
	if data[4*width+4]==0 { t.Errorf("data=%f; want not %f", data[4*width+4], float32(0)) }

	for i:=0; i<len(data); i++ {
		data[i]=100+float32(i&3)
	}
	data[3*width+2]=500
	count=CosmeticCorrectionBayerRedOrBlue(tmp, data, width, 0, 0, 3.0, 5.0)
	if count!=0 { t.Errorf("count=%d; want %d", count, 0) }
	if data[3*width+2]!=500 { t.Errorf("data=%f; want %f", data[3*width+2], float32(500)) }
}


func TestCosmeticCorrectionBayerGreen(t *testing.T) {
	width, height:=int32(13), int32(11) 
	data:=make([]float32, width*height)
	tmp :=make([]float32, width*height)

	for i:=0; i<len(data); i++ {
		data[i]=100+float32(i&3)
	}
	data[2*width+3]=500
	count:=CosmeticCorrectionBayerGreen(tmp, data, width, 0, 0, 3.0, 5.0)
	if count!=1 { t.Errorf("count=%d; want %d", count, 1) }
	if data[2*width+3]==500 { t.Errorf("data=%f; want not %f", data[2*width+3], float32(500)) }

	for i:=0; i<len(data); i++ {
		data[i]=100+float32(i&3)
	}
	data[2*width+3]=0
	count=CosmeticCorrectionBayerGreen(tmp, data, width, 0, 0, 3.0, 5.0)
	if count!=1 { t.Errorf("count=%d; want %d", count, 1) }
	if data[2*width+3]==0 { t.Errorf("data=%f; want not %f", data[2*width+3], float32(0)) }

	for i:=0; i<len(data); i++ {
		data[i]=100+float32(i&3)
	}
	data[3*width+2]=500
	count=CosmeticCorrectionBayerGreen(tmp, data, width, 0, 0, 3.0, 5.0)
	if count!=1 { t.Errorf("count=%d; want %d", count, 1) }
	if data[3*width+2]==500 { t.Errorf("data=%f; want not %f", data[3*width+2], float32(500)) }

	for i:=0; i<len(data); i++ {
		data[i]=100+float32(i&3)
	}
	data[3*width+2]=0
	count=CosmeticCorrectionBayerGreen(tmp, data, width, 0, 0, 3.0, 5.0)
	if count!=1 { t.Errorf("count=%d; want %d", count, 1) }
	if data[3*width+2]==0 { t.Errorf("data=%f; want not %f", data[3*width+2], float32(0)) }

	for i:=0; i<len(data); i++ {
		data[i]=100+float32(i&3)
	}
	data[2*width+2]=500
	count=CosmeticCorrectionBayerGreen(tmp, data, width, 0, 0, 3.0, 5.0)
	if count!=0 { t.Errorf("count=%d; want %d", count, 0) }
	if data[2*width+2]!=500 { t.Errorf("data=%f; want %f", data[2*width+2], float32(500)) }
}
