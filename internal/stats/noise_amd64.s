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

// 3x3 convolution weights for noise estimation
DATA convW1<>+0x00(SB)/4, $1.0
GLOBL convW1<>(SB), RODATA|NOPTR, $4

DATA convWM2<>+0x00(SB)/4, $-2.0
GLOBL convWM2<>(SB), RODATA|NOPTR, $4

DATA convW4<>+0x00(SB)/4, $4.0
GLOBL convW4<>(SB), RODATA|NOPTR, $4

// Mask for VPERMPS to shift each SIMD value right by one
DATA shiftRConst<>+0x00(SB)/4, $0x00000001
DATA shiftRConst<>+0x04(SB)/4, $0x00000002
DATA shiftRConst<>+0x08(SB)/4, $0x00000003
DATA shiftRConst<>+0x0C(SB)/4, $0x00000004
DATA shiftRConst<>+0x10(SB)/4, $0x00000005
DATA shiftRConst<>+0x14(SB)/4, $0x00000006
DATA shiftRConst<>+0x18(SB)/4, $0x00000007
DATA shiftRConst<>+0x1C(SB)/4, $0x00000008
GLOBL shiftRConst<>(SB), RODATA|NOPTR, $64

// Mask to calculate absolute value of a float AND simultaneously zero the two uppermost SIMD lanes, which hold only partial results, with VANDPS
DATA absFilterMask<>+0x00(SB)/4, $0x7fffffff
DATA absFilterMask<>+0x04(SB)/4, $0x7fffffff
DATA absFilterMask<>+0x08(SB)/4, $0x7fffffff
DATA absFilterMask<>+0x0C(SB)/4, $0x7fffffff
DATA absFilterMask<>+0x10(SB)/4, $0x7fffffff
DATA absFilterMask<>+0x14(SB)/4, $0x7fffffff
DATA absFilterMask<>+0x18(SB)/4, $0x00000000
DATA absFilterMask<>+0x1C(SB)/4, $0x00000000
GLOBL absFilterMask<>(SB), RODATA|NOPTR, $64

// Mask to adjust SIMD lane for last, possibly incomplete flight of each line, with VANDPS
DATA filterMask2<>+0x00(SB)/4, $0x00000000
DATA filterMask2<>+0x04(SB)/4, $0x00000000
DATA filterMask2<>+0x08(SB)/4, $0x00000000
DATA filterMask2<>+0x0C(SB)/4, $0x00000000
DATA filterMask2<>+0x10(SB)/4, $0x00000000
DATA filterMask2<>+0x14(SB)/4, $0x00000000
DATA filterMask2<>+0x18(SB)/4, $0x00000000
DATA filterMask2<>+0x1C(SB)/4, $0x00000000
DATA filterMask2<>+0x20(SB)/4, $0xffffffff
DATA filterMask2<>+0x24(SB)/4, $0xffffffff
DATA filterMask2<>+0x28(SB)/4, $0xffffffff
DATA filterMask2<>+0x2C(SB)/4, $0xffffffff
DATA filterMask2<>+0x30(SB)/4, $0xffffffff
DATA filterMask2<>+0x34(SB)/4, $0xffffffff
DATA filterMask2<>+0x38(SB)/4, $0x00000000
DATA filterMask2<>+0x3C(SB)/4, $0x00000000
GLOBL filterMask2<>(SB), RODATA|NOPTR, $64

// func estimateNoiseLineAVX2(source []float32, width int64) float32
//    0(FP) 8 byte source pointer
//    8(FP) 8 byte source length,   in float32s
//   16(FP) 8 byte source capacity, in float32s
//   24(FP) 8 byte line width,      in float32s
//   32(FP) 4 byte result float32
TEXT ·estimateNoiseLineAVX2(SB),(NOSPLIT|NOFRAME),$0-36
    MOVQ source_data+0(FP),SI               // load source pointer in SI
    
    MOVQ width+24(FP),DX                    // load line width into DX and adjust for size in bytes, not float32s
    SHLQ $2,DX

    MOVQ SI,BP                              // calculate end of first source line in BP, correcting for 8-way SIMD   
    ADDQ DX,BP
    SUBQ $7*4,BP

    VXORPS Y9, Y9, Y9                       // initialize running sum in Y9 with zero

    VMOVUPS shiftRConst<>+0x00(SB),  Y10    // load right shift constant into Y10

    VBROADCASTSS convW1<>+0x00(SB),  Y11    // load matrix weights and broadcast into all lanes
    VBROADCASTSS convWM2<>+0x00(SB), Y12
    VBROADCASTSS convW4<>+0x00(SB),  Y13

    VMOVUPS absFilterMask<>+0x00(SB),Y14    // load mask for taking absolute value and filtering out invalid lanes from results into Y15

    JMP lineLoopCondition
lineLoopStart:                              // for all data elements

    VMOVUPS (SI),Y0                         // load first row d[0][0:8] into Y0, second row d[1][0:8] into Y3 and third row d[2][0:8] into Y6
    ADDQ DX,SI
    VMOVUPS (SI),Y3
    ADDQ DX,SI
    VMOVUPS (SI),Y6
    SUBQ DX,SI
    SUBQ DX,SI

    ADDQ $6*4,SI                            // move right by 6 float32s. possible optimization by reusing data, moving by 8 exists 

    // Put the 9-neighborhood of d[1][1] into the lowest float32 of Y0..Y8, that is,
    // d[0][0], d[0][1], d[0][2], d[1][0], d[1][1], d[1][2], d[2][0], d[2][1], d[2][2]
    // Put the 9-neighborhood of d[1][2] into the next float32 of Y0..Y8, and so on.
    // This pattern holds up to d[1][6], the last two are missing data and result in nil calculations

    VPERMPS Y0, Y10, Y1                 // Shift Y0/Y3/Y6 left by one entry into Y1/Y4/Y7
    VPERMPS Y3, Y10, Y4
    VPERMPS Y6, Y10, Y7
    VPERMPS Y1, Y10, Y2                 // Shift Y1/Y4/Y7 left by one entry into Y2/Y5/Y8
    VPERMPS Y4, Y10, Y5
    VPERMPS Y7, Y10, Y8

    // Perform the 3x3 convolution and sum elements in Y9
    // Trying to avoid data dependencies with successive VFMADD231PS into same register

    VMULPS Y0, Y11, Y0                    // multiply d[0][0] with 1
    //VADDPS Y0, Y9, Y9

    VMULPS Y1, Y12, Y1                    // multiply d[0][1] with -2
    //VADDPS Y1, Y9, Y9

    VMULPS Y2, Y11, Y2                    // multiply d[0][2] with 1
    //VADDPS Y2, Y9, Y9

    VMULPS Y3, Y12, Y3                    // multiply d[1][0] with -2
    //VADDPS Y3, Y9, Y9

    //VMULPS Y4, Y13, Y4                  // multiply d[1][1] with 4
    //VADDPS Y4, Y0, Y0
    VFMADD231PS Y4, Y13, Y0

    //VMULPS Y5, Y12, Y5                  // multiply d[1][2] with -2
    //VADDPS Y5, Y1, Y1
    VFMADD231PS Y5, Y12, Y1

    //VMULPS Y6, Y11, Y6                  // multiply d[2][0] with 1
    //VADDPS Y6, Y2, Y2
    VFMADD231PS Y6, Y11, Y2

    //VMULPS Y7, Y12, Y7                  // multiply d[2][1] with -2
    //VADDPS Y7, Y3, Y3
    VFMADD231PS Y7, Y12, Y3

    //VMULPS Y8, Y11, Y8                  // multiply d[2][2] with 1
    //VADDPS Y8, Y0, Y0
    VFMADD231PS Y8, Y11, Y0

    VADDPS Y2, Y3, Y2                     // Butterfly reduction into Y0
    VADDPS Y0, Y1, Y0
    VADDPS Y0, Y2, Y0

    VANDPS Y0, Y14, Y0                    // Zero out the invalid lanes of Y0 and take absolute value in one go
    VADDPS Y0, Y9,  Y9                    // update running sum in Y9

lineLoopCondition:      // continue while inside the body of the line
    CMPQ SI, BP
    JL   lineLoopStart

    MOVQ SI, AX         // does a partial processing window remain at the end of the line, due to stride 6? 
    SUBQ BP, AX
    CMPQ AX, $5*4         
    JGE  dontFixUpLine

    ADDQ $1*4, AX       // if yes, position source and destination pointer at end of the line for one more processing step
    SUBQ AX, SI
    SUBQ AX, DI

    LEAQ filterMask2<>+0x20(SB), BX  // load address of lane map adjusting table
    SUBQ AX, BX                      // step backwards AX times in that map
    VMOVUPS (BX), Y0                 // load adjustment for invalid lane map into Y0
    VANDPS  Y0, Y14, Y14             // adjust invalid lane map in Y15 with Y0

    JMP  lineLoopStart

dontFixUpLine:
                                                            // Perform butterfly reduction 
    VPERM2F128 $((1<<0) + (0<<2)), Y9, Y9, Y0               // swap upper/lower halves of Y9 into Y0
    VADDPS Y0, Y9, Y9                                       // add, lower 128 bit now hold 0+4, 1+5, 2+6, 3+7, repeated
    VPERMILPS $((2<<0) + (3<<2) + (0<<4) + (1<<6)), Y9, Y0  // swap upper/lower quarters
    VADDPS Y0, Y9, Y9                                       // add, lower 64 bit now hold 0+2+4+6, 1+3+5+7, repeated 
    VPERMILPS $((1<<0) + (0<<2) + (3<<4) + (2<<6)), Y9, Y0  // swap even/odd cells
    VADDPS Y0, Y9, Y9                                       // add. reduction is now complete and broadcast to all positions
    MOVSS X9, res+32(FP)                                    // store lowest scalar from Y9 (lower half is X9) to memory

    RET
