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
	"fmt"
	"io"
	"math"
	"os"
	"strings"
)

// Writes an in-memory FITS image to a file with given filename.
// Creates/overwrites the file if necessary 
func (fits *FITSImage) WriteFile(fileName string) error {
	//fmt.Println("Reading from " + fileName + "..." )
	f, err:=os.OpenFile(fileName, os.O_WRONLY |os.O_CREATE, 0644)
	if err!=nil { return err }
	defer f.Close()
	return fits.Write(f)
}


// Writes an in-memory FITS image to an io.Writer.
func (fits *FITSImage) Write(f io.Writer) error {
	// Build header in string buffer
	sb:=strings.Builder{}
	writeBool(&sb, "SIMPLE", true, "    FITS standard 4.0")
	writeInt32(&sb, "BITPIX", -32, "    32-bit floating point")
	writeInt32(&sb, "NAXIS",  int32(len(fits.Naxisn)), "[1] Number of axis")
	for i:=0; i<len(fits.Naxisn); i++ {
		writeInt32(&sb, fmt.Sprintf("NAXIS%d",i+1), fits.Naxisn[i], "[1] Axis size")
	}
	writeFloat32(&sb, "BZERO", fits.Bzero, "[1] Zero offset")
	// FIXME: currently omitting all other FITS header entries
	writeEnd(&sb)

	// Pad current header block with spaces if necessary
	bytesInHeaderBlock:=(sb.Len() % 2880)
	if bytesInHeaderBlock>0 {
		for i:=bytesInHeaderBlock; i<2880; i++ {
			sb.WriteRune(' ')
		} 
	}

	// Write header block(s)
	_, err:=f.Write([]byte(sb.String()))
	if err!=nil { return err }

	// Write payload data, replacing NaNs with zeros for compatibility
	return writeFloat32Array(f, fits.Data, true)
}


// Writes a FITS header boolean value 
func writeBool(w io.Writer, key string, value bool, comment string) {
	if len(key)>8 { key=key[0:8] }
	if len(comment)>47 { comment=comment[0:47] }
	v:="F"
	if value { v="T" }
	fmt.Fprintf(w, "%-8s= %20s / %-47s", key, v, comment)
}


// Writes a FITS header integer value 
func writeInt(w io.Writer, key string, value int, comment string) {
	if len(key)>8 { key=key[0:8] }
	if len(comment)>47 { comment=comment[0:47] }
	fmt.Fprintf(w, "%-8s= %20d / %-47s", key, value, comment)
}


// Writes a FITS header int32 value 
func writeInt32(w io.Writer, key string, value int32, comment string) {
	if len(key)>8 { key=key[0:8] }
	if len(comment)>47 { comment=comment[0:47] }
	fmt.Fprintf(w, "%-8s= %20d / %-47s", key, value, comment)
}


// Writes a FITS header int64 value 
func writeInt64(w io.Writer, key string, value int64, comment string) {
	if len(key)>8 { key=key[0:8] }
	if len(comment)>47 { comment=comment[0:47] }
	fmt.Fprintf(w, "%-8s= %20d / %-47s", key, value, comment)
}


// Writes a FITS header float32 value 
func writeFloat32(w io.Writer, key string, value float32, comment string) {
	if len(key)>8 { key=key[0:8] }
	if len(comment)>47 { comment=comment[0:47] }
	fmt.Fprintf(w, "%-8s= %20g / %-47s", key, value, comment)
}


// Writes a FITS header float64 value 
func writeFloat64(w io.Writer, key string, value float64, comment string) {
	if len(key)>8 { key=key[0:8] }
	if len(comment)>47 { comment=comment[0:47] }
	fmt.Fprintf(w, "%-8s= %20g / %-47s", key, value, comment)
}


// Writes a FITS header string value, with escaping and continuations if necessary. 
func writeString(w io.Writer, key, value, comment string) {
	if len(key)>8 { key=key[0:8] }
	if len(comment)>47 { comment=comment[0:47] }

	// escape ' characters
	value=strings.Join(strings.Split(value, "'"), "''")
	// FIXME: Malformatted output ERROR if printing breaks the '' characters!

	if len(value)<=18 {
		fmt.Fprintf(w, "%-8s= '%s'%s / %-47s", key, value, strings.Repeat(" ", 18-len(value)), comment)
	} else {
		fmt.Fprintf(w, "%-8s= '%s&' / %-47s", key, value[0:17], comment)
		value=value[17:]
		for ; len(value)>66 ; {
			fmt.Fprintf(w, "CONTINUE  '%s&' ", value[0:66])
			value=value[66:]
		}
		fmt.Fprintf(w, "CONTINUE  '%s'%s", value, strings.Repeat(" ", 50+(18-len(value))))
	}	
}


// Writes a FITS header end record 
func writeEnd(w io.Writer) {
	fmt.Fprintf(w, "END%s", strings.Repeat(" ", 80-3))
}

// Writes FITS binary body data in network byte order. 
// Optionally replaces NaNs with zeros for compatibility with other software
func writeFloat32Array(w io.Writer, data []float32, replaceNaNs bool) error {
	buf:=make([]byte,bufLen)

	for block:=0; block<len(data); block+=(bufLen>>2) {
		size:=len(data)-block
		if size>(bufLen>>2) { size=(bufLen>>2) }

		for offset:=0; offset<size; offset++ {
			d:=data[block+offset]
			if replaceNaNs && math.IsNaN(float64(d)) { d=0 }
			val:=math.Float32bits(d)
			buf[(offset<<2)+0]=byte(val>>24)
			buf[(offset<<2)+1]=byte(val>>16)
			buf[(offset<<2)+2]=byte(val>> 8)
			buf[(offset<<2)+3]=byte(val    )
		}
		_, err:=w.Write(buf[:(size<<2)])
		if err!=nil { return err }
	}
	return nil
}
