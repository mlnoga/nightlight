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
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
)

var reParser *regexp.Regexp=compileRE() // Regexp parser for FITS header lines

func (fits *FITSImage) ReadFile(fileName string) error {
	//LogPrintln("Reading from " + fileName + "..." )
	f, err:=os.Open(fileName)
	if err!=nil { return err }
	defer f.Close()
	fits.FileName=fileName
	return fits.Read(f)
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
	//LogPrintf("Found %dbpp image in %dD with dimensions %v, total %d pixels.\n", 
	//		   fits.Bitpix, len(fits.Naxisn), fits.Naxisn, fits.Pixels)
	return fits.readData(f)
}


// Read image data from file, convert to float32 data type, apply BZero offset and set BZero to 0 afterwards.
func (fits *FITSImage) readData(f io.Reader) (err error) {
	switch fits.Bitpix {
	case 8: 
		raw:=GetArrayOfInt8FromPool(int(fits.Pixels))
		err=binary.Read(f, binary.BigEndian, raw)
		if err==nil {
			fits.Data=GetArrayOfFloat32FromPool(int(fits.Pixels))
			for i, val:=range(raw) { 
				fits.Data[i]=float32(val)+fits.Bzero
			}
		}
		PutArrayOfInt8IntoPool(raw)
		raw=nil

	case 16:
		raw:=GetArrayOfInt16FromPool(int(fits.Pixels))
		err=binary.Read(f, binary.BigEndian, raw)
		if err==nil {
			fits.Data=GetArrayOfFloat32FromPool(int(fits.Pixels))
			for i, val:=range(raw) { 
				fits.Data[i]=float32(val)+fits.Bzero
			}
		}
		PutArrayOfInt16IntoPool(raw)
		raw=nil

	case 32:
		LogPrintf("Warning: loss of precision converting int%d to float32 values\n", fits.Bitpix)
		raw:=GetArrayOfInt32FromPool(int(fits.Pixels))
		err=binary.Read(f, binary.BigEndian, raw)
		if err==nil {
			fits.Data=GetArrayOfFloat32FromPool(int(fits.Pixels))
			for i, val:=range(raw) { 
				fits.Data[i]=float32(val)+fits.Bzero
			}
		}
		PutArrayOfInt32IntoPool(raw)
		raw=nil

	case 64: 
		LogPrintf("Warning: loss of precision converting int%d to float32 values\n", fits.Bitpix)
		raw:=GetArrayOfInt64FromPool(int(fits.Pixels))
		err=binary.Read(f, binary.BigEndian, raw)
		if err==nil {
			fits.Data=GetArrayOfFloat32FromPool(int(fits.Pixels))
			for i, val:=range(raw) { 
				fits.Data[i]=float32(val)+fits.Bzero
			}
		}
		PutArrayOfInt64IntoPool(raw)
		raw=nil

	case -32:
		raw:=GetArrayOfFloat32FromPool(int(fits.Pixels))
		err=binary.Read(f, binary.BigEndian, raw)
		if err==nil {
			fits.Data=raw
			for i, _:=range(raw) { 
				fits.Data[i]+=fits.Bzero
			}
		}
		// do not return to pool, data used directly!

	case -64:
		LogPrintf("Warning: loss of precision converting float%d to float32 values\n", -fits.Bitpix)
		raw:=GetArrayOfFloat64FromPool(int(fits.Pixels))
		err=binary.Read(f, binary.BigEndian, raw)
		if err==nil {
			fits.Data=GetArrayOfFloat32FromPool(int(fits.Pixels))
			for i, val:=range(raw) { 
				fits.Data[i]=float32(val)+fits.Bzero
			}
		}
		PutArrayOfFloat64IntoPool(raw)
		raw=nil

	default:
		return errors.New("Unknown BITPIX value "+strconv.FormatInt(int64(fits.Bitpix),10))
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
				LogPrintf("%d:Warning:Cannot parse '%s', ignoring\n",lineNo, string(line))
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
