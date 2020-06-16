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


#include "textflag.h"

// Mask for VPERMPS to shift each SIMD value right by one
DATA shiftRConstant<>+0x00(SB)/4, $0x00000001
DATA shiftRConstant<>+0x04(SB)/4, $0x00000002
DATA shiftRConstant<>+0x08(SB)/4, $0x00000003
DATA shiftRConstant<>+0x0C(SB)/4, $0x00000004
DATA shiftRConstant<>+0x10(SB)/4, $0x00000005
DATA shiftRConstant<>+0x14(SB)/4, $0x00000006
DATA shiftRConstant<>+0x18(SB)/4, $0x00000007
DATA shiftRConstant<>+0x1C(SB)/4, $0x00000008
GLOBL shiftRConstant<>(SB), RODATA|NOPTR, $64

// Mask for VPERMPS to shift each SIMD value left by one
DATA shiftLConstant<>+0x00(SB)/4, $0x00000008
DATA shiftLConstant<>+0x04(SB)/4, $0x00000000
DATA shiftLConstant<>+0x08(SB)/4, $0x00000001
DATA shiftLConstant<>+0x0C(SB)/4, $0x00000002
DATA shiftLConstant<>+0x10(SB)/4, $0x00000003
DATA shiftLConstant<>+0x14(SB)/4, $0x00000004
DATA shiftLConstant<>+0x18(SB)/4, $0x00000005
DATA shiftLConstant<>+0x1C(SB)/4, $0x00000006
GLOBL shiftLConstant<>(SB), RODATA|NOPTR, $64

// Mask for VPMASKMOV to store only middle six SIMD values
DATA storeMask<>+0x00(SB)/4, $0x00000000
DATA storeMask<>+0x04(SB)/4, $0x80000000
DATA storeMask<>+0x08(SB)/4, $0x80000000
DATA storeMask<>+0x0C(SB)/4, $0x80000000
DATA storeMask<>+0x10(SB)/4, $0x80000000
DATA storeMask<>+0x14(SB)/4, $0x80000000
DATA storeMask<>+0x18(SB)/4, $0x80000000
DATA storeMask<>+0x1C(SB)/4, $0x00000000
GLOBL storeMask<>(SB), RODATA|NOPTR, $64

// func MedianFilterLine3x3AVX2(dest, source []float32, width int64)
//    0(FP) 8 byte dest   pointer
//    8(FP) 8 byte dest   length,   in float32s
//   16(FP) 8 byte dest   capacity, in float32s
//   24(FP) 8 byte source pointer
//   32(FP) 8 byte source length,   in float32s
//   40(FP) 8 byte source capacity, in float32s
//   48(FP) 8 byte line width,      in float32s
TEXT ·MedianFilterLine3x3AVX2(SB),(NOSPLIT|NOFRAME),$0-56
    MOVQ source_data+24(FP),SI              // load source pointer in SI
    
    MOVQ width+48(FP),DX                    // load line width into DX and adjust for size in bytes, not float32s
    SHLQ $2,DX

    MOVQ SI,BP                              // calculate end of first source line in BP, correcting for 8-way SIMD   
    ADDQ DX,BP
    SUBQ $7*4,BP

    MOVQ dest_ptr+0(FP),DI                  // load destination pointer in DI and adjust it to point into the second line //, second column
    ADDQ DX, DI
    //ADDQ $1*4, DI

    VMOVUPS shiftRConstant<>+0x00(SB), Y13  // load right shift constant into Y13, left into Y14 and mask for storing results into Y15 
    VMOVUPS shiftLConstant<>+0x00(SB), Y14
    VMOVUPS storeMask<>+0x00(SB), Y15

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

    VPERMPS Y0, Y13, Y1                 // Shift Y0/Y3/Y6 left by one entry into Y1/Y4/Y7
    VPERMPS Y3, Y13, Y4
    VPERMPS Y6, Y13, Y7
    VPERMPS Y1, Y13, Y2                 // Shift Y1/Y4/Y7 left by one entry into Y2/Y5/Y8
    VPERMPS Y4, Y13, Y5
    VPERMPS Y7, Y13, Y8

    // Apply perfect median-of-9 sorting network 8x in parallel, computing 6 medians 
    // The last 2 are missing data and are hence idle and ignored. possible optimization by filling them 

#define Data0   Y0
#define Data1   Y1
#define Data2   Y2
#define Data3   Y3
#define Data4   Y4
#define Data5   Y5
#define Data6   Y6
#define Data7   Y7
#define Data8   Y8
#define DFree   Y9

    // D0 D1 D2 D3 D4 D5 D6 D7 D8 DFree
    // Y0 Y1 Y2 Y3 Y4 Y5 Y6 Y7 Y8 Y9

    VMINPS Y0,Y1,Y9    // swap(a,0,1)
    VMAXPS Y0,Y1,Y1

    // Swap D0 and DFree
    // D0 D1 D2 D3 D4 D5 D6 D7 D8 DFree
    // Y9 Y1 Y2 Y3 Y4 Y5 Y6 Y7 Y8 Y0

    VMINPS Y3,Y4,Y0    // swap(a,3,4)
    VMAXPS Y3,Y4,Y4

    // Swap D3 and DFree
    // D0 D1 D2 D3 D4 D5 D6 D7 D8 DFree
    // Y9 Y1 Y2 Y0 Y4 Y5 Y6 Y7 Y8 Y3

    VMINPS Y6,Y7,Y3    // swap(a,6,7)
    VMAXPS Y6,Y7,Y7

    // Swap D6 and DFree
    // D0 D1 D2 D3 D4 D5 D6 D7 D8 DFree
    // Y9 Y1 Y2 Y0 Y4 Y5 Y3 Y7 Y8 Y6

    VMINPS Y1,Y2,Y6    // swap(a,1,2)
    VMAXPS Y1,Y2,Y2

    // Swap D1 and DFree
    // D0 D1 D2 D3 D4 D5 D6 D7 D8 DFree
    // Y9 Y6 Y2 Y0 Y4 Y5 Y3 Y7 Y8 Y1

    VMINPS Y4,Y5,Y1    // swap(a,4,5)
    VMAXPS Y4,Y5,Y5

    // Swap D4 and DFree
    // D0 D1 D2 D3 D4 D5 D6 D7 D8 DFree
    // Y9 Y6 Y2 Y0 Y1 Y5 Y3 Y7 Y8 Y4

    VMINPS Y7,Y8,Y4    // swap(a,7,8)
    VMAXPS Y7,Y8,Y8

    // Swap D7 and DFree
    // D0 D1 D2 D3 D4 D5 D6 D7 D8 DFree
    // Y9 Y6 Y2 Y0 Y1 Y5 Y3 Y4 Y8 Y7

    VMINPS Y9,Y6,Y7    // swap(a,0,1)
    VMAXPS Y9,Y6,Y6

    // Swap D0 and DFree
    // D0 D1 D2 D3 D4 D5 D6 D7 D8 DFree
    // Y7 Y6 Y2 Y0 Y1 Y5 Y3 Y4 Y8 Y9

    VMINPS Y0,Y1,Y9    // swap(a,3,4)
    VMAXPS Y0,Y1,Y1

    // Swap D3 and DFree
    // D0 D1 D2 D3 D4 D5 D6 D7 D8 DFree
    // Y7 Y6 Y2 Y9 Y1 Y5 Y3 Y4 Y8 Y0

    VMINPS Y3,Y4,Y0    // swap(a,6,7)
    VMAXPS Y3,Y4,Y4

    // Swap D6 and DFree
    // D0 D1 D2 D3 D4 D5 D6 D7 D8 DFree
    // Y7 Y6 Y2 Y9 Y1 Y5 Y0 Y4 Y8 Y3

    VMAXPS Y7,Y9,Y9    // max (a,0,3)

    VMAXPS Y9,Y0,Y0    // max (a,3,6)

    VMINPS Y6,Y1,Y3    // swap(a,1,4)
    VMAXPS Y6,Y1,Y1

    // Swap D1 and DFree
    // D0 D1 D2 D3 D4 D5 D6 D7 D8 DFree
    // Y7 Y3 Y2 Y9 Y1 Y5 Y0 Y4 Y8 Y6

    VMINPS Y1,Y4,Y1    // min (a,4,7)

    VMAXPS Y3,Y1,Y1    // max (a,1,4)

    VMINPS Y5,Y8,Y5    // min (a,5,8)

    VMINPS Y2,Y5,Y2    // min (a,2,5)

    VMINPS Y2,Y1,Y6    // swap(a,2,4)
    VMAXPS Y2,Y1,Y1

    // Swap D2 and DFree
    // D0 D1 D2 D3 D4 D5 D6 D7 D8 DFree
    // Y7 Y3 Y6 Y9 Y1 Y5 Y0 Y4 Y8 Y2

    VMINPS Y1,Y0,Y1    // min (a,4,6)

    VMAXPS Y6,Y1,Y1    // max (a,2,4)

    VPERMPS Y1, Y14, Y1                  // shift results left by one
    VMASKMOVPS Y1, Y15, (DI)             // store into middle six elements of the destination array
    //VMOVUPS Y1, (DI)                   // store into middle six elements of the destination array
    ADDQ $6*4, DI                        // increment output pointer bData 6 float32s

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
    JMP  lineLoopStart

dontFixUpLine:

    RET
