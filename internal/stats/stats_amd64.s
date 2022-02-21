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

// +build amd64


#include "textflag.h"

// func calcMinMeanMaxAVX2(data []float32) (min, mean, max float64)
//    0(FP) 8 byte data pointer
//    8(FP) 8 byte data length
//   16(FP) 8 byte data capacity
//   24(FP) 4 byte return value min
//   28(FP) 4 byte return value mean
//   32(FP) 4 byte return value max
TEXT ·calcMinMeanMaxAVX2(SB),(NOSPLIT|NOFRAME),$0-36
    // initialize data pointer in DI and end pointer in SI
    MOVQ data_ptr+0(FP),DI  
    MOVQ data_len+8(FP),SI
    SHLQ $2,SI
    ADDQ DI, SI

    // initialize min in xmm2 register (4 floats each)
    MOVUPS (DI), X2
    // initialize max in xmm3 register (4 floats each)
    MOVAPS X2, X3

    // initialize running sum in ymm0 register (4 doubles)
    XORPD X0, X0
    VBROADCASTSS X0, X0
    VCVTPS2PD X0, Y0

    JMP mmmLoopCond
mmmLoopStart:
    // load next data elements as vector
    MOVUPS (DI), X1
    ADDQ $16,DI

    // update min and max
    VMINPS X1, X2, X2
    VMAXPS X1, X3, X3

    // update running sum vectors
    VCVTPS2PD X1, Y1
    VADDPD Y1,Y0,Y0


mmmLoopCond:
    CMPQ DI, SI
    JL   mmmLoopStart

    // reduce float vector X2 with minima into a float scalar and store return value on stack
    VPERMILPS $((1<<0) + (0<<2) + (3<<4) + (2<<6)), X2, X4 
    VMINPS X4, X2, X2
    VPERMILPS $((2<<0) + (3<<2) + (0<<4) + (1<<6)), X2, X4 
    VMINPS X4, X2, X2
    MOVSS X2, res+24(FP)

    // reduce float vector X3 with maxima into a float scalar and store return value on stack
    VPERMILPS $((1<<0) + (0<<2) + (3<<4) + (2<<6)), X3, X4 
    VMAXPS X4, X3, X3
    VPERMILPS $((2<<0) + (3<<2) + (0<<4) + (1<<6)), X3, X4 
    VMAXPS X4, X3, X3
    MOVSS X3, res+32(FP)

    // reduce double vector Y0 with horizontal sum into a double scalar in X0
    VPERMILPD $5, Y0, Y1 
    VADDPD Y0, Y1, Y1
    VEXTRACTF128 $0, Y1, X0
    VEXTRACTF128 $1, Y1, X1
    VADDPD X0, X1, X0

    // divide by number of elements, convert to float and store return value on stack
    MOVQ data_len+8(FP),AX
    CVTSQ2SD AX,X5
    DIVSD X5,X0
    CVTSD2SS X0,X0
    MOVSS X0, res+28(FP)

    RET


// func calcVarianceAVX2(data []float32, mean float32) (res float64)
//    0(FP) 8 byte data pointer
//    8(FP) 8 byte data length
//   16(FP) 8 byte data capacity
//   24(FP) 4 byte mean
//   28(FP) 4 byte __padding__
//   32(FP) 8 byte return value
TEXT ·calcVarianceAVX2(SB),(NOSPLIT|NOFRAME),$0-36
    // initialize data pointer in DI and end pointer in SI
    MOVQ data_ptr+0(FP),DI  
    MOVQ data_len+8(FP),SI
    SHLQ $2,SI
    ADDQ DI, SI

    // initialize mean in xmm2 register (4 floats)
    VBROADCASTSS mean+24(FP),X2

    // initialize running sum in ymm0 register (4 doubles)
    XORPD X0, X0
    VBROADCASTSS X0, X0
    VCVTPS2PD X0, Y0

    JMP vLoopCond
vLoopStart:
    // sum+=(*ptr-mean)*(*ptr-mean) in parallel 
    MOVUPS (DI), X1
    ADDQ $16,DI
    SUBPS X2,X1
    VCVTPS2PD X1, Y1
    VMULPD Y1,Y1,Y1
    VADDPD Y1,Y0,Y0

vLoopCond:
    CMPQ DI, SI
    JL   vLoopStart

    // reduce double vector Y0 with horizontal sum into a double scalar in X0
    VPERMILPD $5, Y0, Y1 
    VADDPD Y0, Y1, Y1
    VEXTRACTF128 $0, Y1, X0
    VEXTRACTF128 $1, Y1, X1
    VADDPD X0, X1, X0

    // divide by number of elements and return
    MOVQ data_len+8(FP),AX
    CVTSQ2SD AX,X3
    DIVSD X3,X0
    MOVSD X0, res+32(FP)
    RET
