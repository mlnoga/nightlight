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
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"math" 
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var reParser *regexp.Regexp=compileRE() // Regexp parser for FITS header lines

// Read FITS data from the file with the given name. Decompresses gzip if .gz or gzip suffix is present
func (fits *FITSImage) ReadFile(fileName string) error {
	//LogPrintln("Reading from " + fileName + "..." )
	f, err:=os.Open(fileName)
	if err!=nil { return err }
	defer f.Close()

	var r io.Reader=f

	// Decompress gzip if .gz or .gzip suffix is present
	ext:=path.Ext(fileName)
	lExt:=strings.ToLower(ext)
	if lExt==".gz" || lExt==".gzip" {
		r, err=gzip.NewReader(f)
		if err!=nil { return err }
	} 

	fits.FileName=fileName
	return fits.Read(r)
}


func (fits *FITSImage) Read(f io.Reader) error {
	err:=fits.Header.read(f)
	if err!=nil { return err }
	if(!fits.Header.Bools["SIMPLE"]) { return errors.New("Not a valid FITS file; SIMPLE=T missing in header.") }

	fits.Bitpix=fits.Header.Ints["BITPIX"]
	fits.Bzero =float32(0)
	if val, ok:=fits.Header.Ints["BZERO"] ; ok {
		fits.Bzero=float32(val)
	} else if val, ok:=fits.Header.Floats["BZERO"] ; ok {
		fits.Bzero=val
	}
	naxis     :=fits.Header.Ints["NAXIS"]
	fits.Naxisn=make([]int32, naxis)
	fits.Pixels=int32(1)
	for i:=int32(1); i<=naxis; i++ {
		name:="NAXIS"+strconv.FormatInt(int64(i),10)
		nai:=fits.Header.Ints[name]
		fits.Naxisn[i-1]=nai
		fits.Pixels*=int32(nai)
	}
	if val, ok:=fits.Header.Ints["EXPOSURE"] ; ok {
		fits.Exposure=float32(val)
	} else if val, ok:=fits.Header.Floats["EXPOSURE"] ; ok {
		fits.Exposure=val
	} else 	if val, ok:=fits.Header.Ints["EXPTIME"] ; ok {
		fits.Exposure=float32(val)
	} else if val, ok:=fits.Header.Floats["EXPTIME"] ; ok {
		fits.Exposure=val
	}

	//LogPrintf("Found %dbpp image in %dD with dimensions %v, total %d pixels.\n", 
	//		   fits.Bitpix, len(fits.Naxisn), fits.Naxisn, fits.Pixels)
	return fits.readData(f)
}


// Read image data from file, convert to float32 data type, apply BZero offset and set BZero to 0 afterwards.
func (fits *FITSImage) readData(f io.Reader) (err error) {
	switch fits.Bitpix {
	case 8: 
		return fits.readInt8Data(f)

	case 16:
		return fits.readInt16Data(f)

	case 32:
		LogPrintf("Warning: loss of precision converting int%d to float32 values\n", fits.Bitpix)
		return fits.readInt32Data(f)

	case 64: 
		LogPrintf("Warning: loss of precision converting int%d to float32 values\n", fits.Bitpix)
		return fits.readInt64Data(f)

	case -32:
		return fits.readFloat32Data(f)

	case -64:
		LogPrintf("Warning: loss of precision converting float%d to float32 values\n", -fits.Bitpix)
		return fits.readFloat64Data(f)

	default:
		return errors.New("Unknown BITPIX value "+strconv.FormatInt(int64(fits.Bitpix),10))
	}

	return nil
}


const bufLen int=16*1024  // input buffer length for reading from file

// Batched read of data of the given size and type from the file, converting from network byte order and adjusting for Bzero
func (fits *FITSImage) readInt8Data(r io.Reader) error {
	fits.Data=make([]float32,int(fits.Pixels))
	buf:=make([]byte,bufLen)

	dataIndex:=0	
	for ; dataIndex<len(fits.Data) ; {
		bytesToRead:=(len(fits.Data)-dataIndex)*1
		if bytesToRead>bufLen {
			bytesToRead=bufLen
		}
		bytesRead, err:=r.Read(buf[:bytesToRead])
		if err!=nil { return err }

		for i, val:=range(buf[:bytesRead]) { 
			fits.Data[dataIndex+i]=float32(val)+fits.Bzero
		}
		dataIndex+=bytesRead
	}
	fits.Bzero=0 // offset has been adjusted on data values
	return nil
}

// Batched read of data of the given size and type from the file, converting from network byte order and adjusting for Bzero
func (fits *FITSImage) readInt16Data(r io.Reader) error {
	fits.Data=make([]float32,int(fits.Pixels))
	buf     :=make([]byte,bufLen)

	bytesPerValueShift:=uint(1)
	bytesPerValue:=1<<bytesPerValueShift
	bytesPerValueMask:=bytesPerValue-1
	dataIndex:=0	
	leftoverBytes:=0
	for ; dataIndex<len(fits.Data) ; {
		bytesToRead:=(len(fits.Data)-dataIndex)*bytesPerValue-leftoverBytes
		if bytesToRead>bufLen {
			bytesToRead=bufLen
		}
		bytesRead, err:=r.Read(buf[leftoverBytes:leftoverBytes+bytesToRead])
		if err!=nil { return err }

		availableBytes:=leftoverBytes+bytesRead
		for i:=0; i<(availableBytes&^bytesPerValueMask); i+=bytesPerValue { 
			val:=int16((uint16(buf[i])<<8) | uint16(buf[i+1]))
			fits.Data[dataIndex+(i>>bytesPerValueShift)]=float32(val)+fits.Bzero
		}
		dataIndex   += availableBytes>>bytesPerValueShift
		leftoverBytes= availableBytes& bytesPerValueMask
		for i:=0; i<leftoverBytes; i++ {
			buf[i]=buf[availableBytes-leftoverBytes+i]
		}
	}
	fits.Bzero=0 // offset has been adjusted on data values
	return nil
}

// Batched read of data of the given size and type from the file, converting from network byte order and adjusting for Bzero
func (fits *FITSImage) readInt32Data(r io.Reader) error {
	fits.Data=make([]float32,int(fits.Pixels))
	buf     :=make([]byte,bufLen)

	bytesPerValueShift:=uint(2)
	bytesPerValue:=1<<bytesPerValueShift
	bytesPerValueMask:=bytesPerValue-1
	dataIndex:=0	
	leftoverBytes:=0
	for ; dataIndex<len(fits.Data) ; {
		bytesToRead:=(len(fits.Data)-dataIndex)*bytesPerValue-leftoverBytes
		if bytesToRead>bufLen {
			bytesToRead=bufLen
		}
		bytesRead, err:=r.Read(buf[leftoverBytes:leftoverBytes+bytesToRead])
		if err!=nil { return err }

		availableBytes:=leftoverBytes+bytesRead
		for i:=0; i<(availableBytes&^bytesPerValueMask); i+=bytesPerValue { 
			val:=int32((uint32(buf[i])<<24) | (uint32(buf[i+1])<<16) | (uint32(buf[i+2])<<8) | (uint32(buf[i+3])))
			fits.Data[dataIndex+(i>>bytesPerValueShift)]=float32(val)+fits.Bzero
		}
		dataIndex   += availableBytes>>bytesPerValueShift
		leftoverBytes= availableBytes& bytesPerValueMask
		for i:=0; i<leftoverBytes; i++ {
			buf[i]=buf[availableBytes-leftoverBytes+i]
		}
	}
	fits.Bzero=0 // offset has been adjusted on data values
	return nil
}

// Batched read of data of the given size and type from the file, converting from network byte order and adjusting for Bzero
func (fits *FITSImage) readInt64Data(r io.Reader) error {
	fits.Data=make([]float32,int(fits.Pixels))
	buf     :=make([]byte,bufLen)

	bytesPerValueShift:=uint(3)
	bytesPerValue:=1<<bytesPerValueShift
	bytesPerValueMask:=bytesPerValue-1
	dataIndex:=0	
	leftoverBytes:=0
	for ; dataIndex<len(fits.Data) ; {
		bytesToRead:=(len(fits.Data)-dataIndex)*bytesPerValue-leftoverBytes
		if bytesToRead>bufLen {
			bytesToRead=bufLen
		}
		bytesRead, err:=r.Read(buf[leftoverBytes:leftoverBytes+bytesToRead])
		if err!=nil { return err }

		availableBytes:=leftoverBytes+bytesRead
		for i:=0; i<(availableBytes&^bytesPerValueMask); i+=bytesPerValue { 
			val:=int64((uint64(buf[i  ])<<56) | (uint64(buf[i+1])<<48) | (uint64(buf[i+2])<<40) | (uint64(buf[i+3])<<32) |
			           (uint64(buf[i+4])<<24) | (uint64(buf[i+5])<<16) | (uint64(buf[i+6])<< 8) | (uint64(buf[i+7])    )   )
			fits.Data[dataIndex+(i>>bytesPerValueShift)]=float32(val)+fits.Bzero
		}
		dataIndex   += availableBytes>>bytesPerValueShift
		leftoverBytes= availableBytes& bytesPerValueMask
		for i:=0; i<leftoverBytes; i++ {
			buf[i]=buf[availableBytes-leftoverBytes+i]
		}
	}
	fits.Bzero=0 // offset has been adjusted on data values
	return nil
}

// Batched read of data of the given size and type from the file, converting from network byte order and adjusting for Bzero
func (fits *FITSImage) readFloat32Data(r io.Reader) error {
	fits.Data=make([]float32,int(fits.Pixels))
	buf     :=make([]byte,bufLen)

	bytesPerValueShift:=uint(2)
	bytesPerValue:=1<<bytesPerValueShift
	bytesPerValueMask:=bytesPerValue-1
	dataIndex:=0	
	leftoverBytes:=0
	for ; dataIndex<len(fits.Data) ; {
		bytesToRead:=(len(fits.Data)-dataIndex)*bytesPerValue-leftoverBytes
		if bytesToRead>bufLen {
			bytesToRead=bufLen
		}
		//LogPrintf("dataIndex %d bytesToRead %d\n", dataIndex, bytesToRead)
		bytesRead, err:=r.Read(buf[leftoverBytes:leftoverBytes+bytesToRead])
		//LogPrintf("bytesRead %d err %d\n", bytesRead, err)
		if err!=nil { return err }

		availableBytes:=leftoverBytes+bytesRead
		//LogPrintf("availableBytes %d\n", availableBytes)
		for i:=0; i<(availableBytes&^bytesPerValueMask); i+=bytesPerValue { 
			bits:=((uint32(buf[i]))<<24) | (uint32(buf[i+1])<<16) | (uint32(buf[i+2])<<8) | (uint32(buf[i+3]))
			val:=math.Float32frombits(bits)
			//LogPrintf("%d: %02x %02x %02x %02x = %08x =%f\n", i, buf[i], buf[i+1], buf[i+2], buf[i+3], bits, val)
			fits.Data[dataIndex+(i>>bytesPerValueShift)]=float32(val)+fits.Bzero
		}
		dataIndex   += availableBytes>>bytesPerValueShift
		leftoverBytes= availableBytes& bytesPerValueMask
		for i:=0; i<leftoverBytes; i++ {
			buf[i]=buf[availableBytes-leftoverBytes+i]
		}
	}
	fits.Bzero=0 // offset has been adjusted on data values
	return nil
}

// Batched read of data of the given size and type from the file, converting from network byte order and adjusting for Bzero
func (fits *FITSImage) readFloat64Data(r io.Reader) error {
	fits.Data=make([]float32,int(fits.Pixels))
	buf     :=make([]byte,bufLen)

	bytesPerValueShift:=uint(3)
	bytesPerValue:=1<<bytesPerValueShift
	bytesPerValueMask:=bytesPerValue-1
	dataIndex:=0	
	leftoverBytes:=0
	for ; dataIndex<len(fits.Data) ; {
		bytesToRead:=(len(fits.Data)-dataIndex)*bytesPerValue-leftoverBytes
		if bytesToRead>bufLen {
			bytesToRead=bufLen
		}
		bytesRead, err:=r.Read(buf[leftoverBytes:leftoverBytes+bytesToRead])
		if err!=nil { return err }

		availableBytes:=leftoverBytes+bytesRead
		for i:=0; i<(availableBytes&^bytesPerValueMask); i+=bytesPerValue { 
			bits:=((uint64(buf[i  ])<<56) | (uint64(buf[i+1])<<48) | (uint64(buf[i+2])<<40) | (uint64(buf[i+3])<<32) |
			       (uint64(buf[i+4])<<24) | (uint64(buf[i+5])<<16) | (uint64(buf[i+6])<< 8) | (uint64(buf[i+7])    )   )
			val:=math.Float64frombits(bits)
			fits.Data[dataIndex+(i>>bytesPerValueShift)]=float32(val)+fits.Bzero
		}
		dataIndex   += availableBytes>>bytesPerValueShift
		leftoverBytes= availableBytes& bytesPerValueMask
		for i:=0; i<leftoverBytes; i++ {
			buf[i]=buf[availableBytes-leftoverBytes+i]
		}
	}
	fits.Bzero=0 // offset has been adjusted on data values
	return nil
}


func (h *FITSHeader) read(r io.Reader) error {
	buf:=make([]byte, fitsBlockSize)

	myParser:=reParser.Copy() // better (thread-)safe for SubexpNames() than sorry

	for h.Length=0; !h.End ; {
		// read next header unit
		bytesRead, err:=io.ReadFull(r, buf)
		if err!=nil || bytesRead!=fitsBlockSize { return err }
		h.Length+=int32(bytesRead)

		// parse all lines in this header unit
		for lineNo:=0; lineNo<fitsBlockSize/fitsHeaderLineSize && !h.End; lineNo++ {
			line:=buf[lineNo*fitsHeaderLineSize:(lineNo+1)*fitsHeaderLineSize]
			subValues:=myParser.FindSubmatch(line)
			if subValues==nil {
				LogPrintf("Warning:Cannot parse '%s', ignoring\n",string(line))
			} else {
				subNames:=myParser.SubexpNames()
				h.readLine(subNames, subValues, lineNo)
			}
		}
	}
	return nil
}


func (h *FITSHeader) readLine(subNames []string, subValues [][]byte, lineNo int) {
	key:=""
	// ignore index 0 which is the whole line
	for i:=1; i<len(subNames); i++ {
		if subValues[i]!=nil && len(subNames[i])==1 {
			switch c:=subNames[i][0]; c {
			case byte('E'): // end line
				h.End=true 
			case byte('H'): // history line
				h.History=append(h.History, string(subValues[i]))
			case byte('C'): // comment line
				h.Comments=append(h.History, string(subValues[i]))
			case byte('k'): // key
				key=string(subValues[i])
			case byte('b'): // boolean
				if len(subValues[i])>0 {
					v:=subValues[i][0]
					h.Bools[key]=v==byte('t') || v==byte('T')
				}
			case byte('i'): // int
				val, err:=strconv.ParseInt(string(subValues[i]),10,64)
				if err==nil {
					h.Ints[key]=int32(val)
				}
			case byte('f'): // float
				val, err:=strconv.ParseFloat(string(subValues[i]),64)
				if err==nil {
					h.Floats[key]=float32(val)
				}
			case byte('s'): // string
				h.Strings[key]=string(subValues[i])
			case byte('d'): // date
				h.Dates[key]=string(subValues[i])
			case byte('c'): // comment
				// ignore value comments
			default:
				LogPrintf("%d:Warning:Unknown token '%s'\n", lineNo, string(c))
			}
		}
	}
}

func (h *FITSHeader) Print() {
	fmt.Printf("Bools   : %v\n", h.Bools)
	fmt.Printf("Ints    : %v\n", h.Ints)
	fmt.Printf("Floats  : %v\n", h.Floats)
	fmt.Printf("Strings : %v\n", h.Strings)
	fmt.Printf("Dates   : %v\n", h.Dates)
	fmt.Printf("History : %v\n", h.History)
	fmt.Printf("Comments: %v\n", h.Comments)
	fmt.Printf("End     : %v\n", h.End)
}


// Build regexp parser for FITS header lines
func compileRE() *regexp.Regexp {
	white   :="\\s+"
	whiteOpt:="\\s*"
	whiteLine:=white

	hist    :="HISTORY"
	rest    :=".*"
	histLine:=hist + white +"(?P<H>"+ rest +")"

	commKey :="COMMENT"
	commLine:=commKey + white + "(?P<C>"+ rest +")" 

	end     :="(?P<E>END)"
	endLine :=end + whiteOpt

	key     :="(?P<k>[A-Z0-9_-]+)"
	equals  :="="

	boo     :="(?P<b>[TF])"
	inte    :="(?P<i>[+-]?[0-9]+)"
    floa    :="(?P<f>[+-]?[0-9]*\\.[0-9]*(?:[ED]-?[0-9]+)?)"
    stri    :="'(?P<s>[^']*)'"
    date    :="(?P<d>[0-9]{1,4}-?[012][0-9]-?[0123][0-9]T[012][0-9]:?[0-5][0-9]:?[0-5][0-9].?[0-9]*)" // FIXME: other variants possible, see ISO8601
    val     :="(?:"+ boo +"|"+ inte +"|"+ floa +"|"+ stri +"|"+ date +")"

    // missing: CONTINUE for strings
    // missing: complex int: (nr, nr)
    // missing: complex float: (nr, nr)

    commOpt :="(?:/(?P<c>.*))?"
    keyLine :=key + whiteOpt + equals + whiteOpt + val + whiteOpt + commOpt

    lineRe  :="^(?:" + whiteLine +"|"+ histLine +"|"+ commLine +"|"+ keyLine +"|"+ endLine +")$"
    return regexp.MustCompile(lineRe)
}
