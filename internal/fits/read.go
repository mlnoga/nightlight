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
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/mlnoga/nightlight/internal/stats"
)

var reParser *regexp.Regexp = compileRE() // Regexp parser for FITS header lines

func NewImageFromFile(fileName string, id int, logWriter io.Writer) (i *Image, err error) {
	i = NewImage()
	i.ID = id
	return i, i.ReadFile(fileName, true, logWriter)
}

func NewImageMasterDataFromFile(fileName string, id int, logWriter io.Writer) (i *Image, err error) {
	i = NewImage()
	i.ID = id
	return i, i.ReadFile(fileName, false, logWriter)
}

// Read FITS data from the file with the given name. Decompresses gzip if .gz or gzip suffix is present.
// Reads metadata only (fast) if readData is false.
func (fits *Image) ReadFile(fileName string, readData bool, logWriter io.Writer) error {
	//LogPrintln("Reading from " + fileName + "..." )
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	var r io.Reader = f

	fits.FileName = fileName
	ext := path.Ext(fileName)
	lExt := strings.ToLower(ext)

	if lExt == ".tif" || lExt == ".tiff" {
		return fits.ReadTIFF(fileName)
	} else if lExt == ".gz" || lExt == ".gzip" {
		// Decompress gzip if .gz or .gzip suffix is present
		r, err = gzip.NewReader(f)
		if err != nil {
			return err
		}
	}

	return fits.Read(r, readData, logWriter)
}

func (fits *Image) PopHeaderInt32(key string) (res int32, err error) {
	if val, ok := fits.Header.Ints[key]; ok {
		delete(fits.Header.Ints, key)
		return val, nil
	}
	return 0, fmt.Errorf("%d: FITS header does not contain key %s", fits.ID, key)
}

func (fits *Image) PopHeaderInt32OrFloat(key string) (res float32, err error) {
	if val, ok := fits.Header.Ints[key]; ok {
		delete(fits.Header.Ints, key)
		return float32(val), nil
	} else if val, ok := fits.Header.Floats[key]; ok {
		delete(fits.Header.Floats, key)
		return val, nil
	}
	return 0, fmt.Errorf("%d: FITS header does not contain key %s", fits.ID, key)
}

func (fits *Image) Read(f io.Reader, readData bool, logWriter io.Writer) (err error) {
	err = fits.Header.read(f, fits.ID, logWriter)
	if err != nil {
		return err
	}

	// check mandatory fields as per standard
	if !fits.Header.Bools["SIMPLE"] {
		return fmt.Errorf("%d: Not a valid FITS file; SIMPLE=T missing in header", fits.ID)
	}
	delete(fits.Header.Bools, "SIMPLE")

	if fits.Bitpix, err = fits.PopHeaderInt32("BITPIX"); err != nil {
		return err
	}
	var naxis int32
	if naxis, err = fits.PopHeaderInt32("NAXIS"); err != nil {
		return err
	}
	fits.Naxisn = make([]int32, naxis)
	fits.Pixels = int32(1)
	for i := int32(1); i <= naxis; i++ {
		name := "NAXIS" + strconv.FormatInt(int64(i), 10)
		var nai int32
		if nai, err = fits.PopHeaderInt32(name); err != nil {
			return err
		}
		fits.Naxisn[i-1] = nai
		fits.Pixels *= int32(nai)
	}

	// check key optional fields relevant for stacking and image processing
	if fits.Bzero, err = fits.PopHeaderInt32OrFloat("BZERO"); err != nil {
		fits.Bzero = 0
	}
	if fits.Bscale, err = fits.PopHeaderInt32OrFloat("BSCALE"); err != nil {
		fits.Bscale = 1
	}
	if fits.Exposure, err = fits.PopHeaderInt32OrFloat("EXPOSURE"); err != nil {
		if fits.Exposure, err = fits.PopHeaderInt32OrFloat("EXPTIME"); err != nil {
			fits.Exposure = 0
		}
	}

	if !readData {
		return nil
	}
	return fits.readData(f, logWriter)
}

// Read image data from file, convert to float32 data type, apply BZero offset and set BZero to 0 afterwards.
func (fits *Image) readData(f io.Reader, logWriter io.Writer) (err error) {
	switch fits.Bitpix {
	case 8:
		return fits.readInt8Data(f)

	case 16:
		return fits.readInt16Data(f)

	case 32:
		fmt.Fprintf(logWriter, "%d: Warning: loss of precision converting int%d to float32 values\n", fits.ID, fits.Bitpix)
		return fits.readInt32Data(f)

	case 64:
		fmt.Fprintf(logWriter, "%d: Warning: loss of precision converting int%d to float32 values\n", fits.ID, fits.Bitpix)
		return fits.readInt64Data(f)

	case -32:
		return fits.readFloat32Data(f)

	case -64:
		fmt.Fprintf(logWriter, "%d: Warning: loss of precision converting float%d to float32 values\n", fits.ID, -fits.Bitpix)
		return fits.readFloat64Data(f)

	default:
		return fmt.Errorf("%d: Unknown BITPIX value %d", fits.ID, fits.Bitpix)
	}
}

const bufLen int = 16 * 1024 // input buffer length for reading from file

// Batched read of data of the given size and type from the file, converting from network byte order and adjusting for Bzero
func (fits *Image) readInt8Data(r io.Reader) error {
	min, max, sum := float32(math.MaxFloat32), float32(-math.MaxFloat32), float64(0)
	fits.Data = make([]float32, int(fits.Pixels))
	buf := make([]byte, bufLen)

	dataIndex := 0
	for dataIndex < len(fits.Data) {
		bytesToRead := (len(fits.Data) - dataIndex) * 1
		if bytesToRead > bufLen {
			bytesToRead = bufLen
		}
		bytesRead, err := r.Read(buf[:bytesToRead])
		if err != nil {
			return fmt.Errorf("%d: %s", fits.ID, err.Error())
		}

		for i, val := range buf[:bytesRead] {
			v := float32(val)*fits.Bscale + fits.Bzero
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
			sum += float64(v)
			fits.Data[dataIndex+i] = v
		}
		dataIndex += bytesRead
	}
	fits.Bzero, fits.Bscale = 0, 1 // reflect that data values incorporate these now
	mean := float32(sum / float64(len(fits.Data)))
	fits.Stats = stats.NewStatsWithMMM(fits.Data, fits.Naxisn[0], min, max, mean)
	return nil
}

// Batched read of data of the given size and type from the file, converting from network byte order and adjusting for Bzero
func (fits *Image) readInt16Data(r io.Reader) error {
	min, max, sum := float32(math.MaxFloat32), float32(-math.MaxFloat32), float64(0)
	fits.Data = make([]float32, int(fits.Pixels))
	buf := make([]byte, bufLen)

	bytesPerValueShift := uint(1)
	bytesPerValue := 1 << bytesPerValueShift
	bytesPerValueMask := bytesPerValue - 1
	dataIndex := 0
	leftoverBytes := 0
	for dataIndex < len(fits.Data) {
		bytesToRead := (len(fits.Data)-dataIndex)*bytesPerValue - leftoverBytes
		if bytesToRead > bufLen {
			bytesToRead = bufLen
		}
		bytesRead, err := r.Read(buf[leftoverBytes : leftoverBytes+bytesToRead])
		if err != nil {
			return fmt.Errorf("%d: %s", fits.ID, err.Error())
		}

		availableBytes := leftoverBytes + bytesRead
		for i := 0; i < (availableBytes &^ bytesPerValueMask); i += bytesPerValue {
			val := int16((uint16(buf[i]) << 8) | uint16(buf[i+1]))
			v := float32(val)*fits.Bscale + fits.Bzero
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
			sum += float64(v)
			fits.Data[dataIndex+(i>>bytesPerValueShift)] = v
		}
		dataIndex += availableBytes >> bytesPerValueShift
		leftoverBytes = availableBytes & bytesPerValueMask
		for i := 0; i < leftoverBytes; i++ {
			buf[i] = buf[availableBytes-leftoverBytes+i]
		}
	}
	fits.Bzero, fits.Bscale = 0, 1 // reflect that data values incorporate these now
	mean := float32(sum / float64(len(fits.Data)))
	fits.Stats = stats.NewStatsWithMMM(fits.Data, fits.Naxisn[0], min, max, mean)
	return nil
}

// Batched read of data of the given size and type from the file, converting from network byte order and adjusting for Bzero
func (fits *Image) readInt32Data(r io.Reader) error {
	min, max, sum := float32(math.MaxFloat32), float32(-math.MaxFloat32), float64(0)
	fits.Data = make([]float32, int(fits.Pixels))
	buf := make([]byte, bufLen)

	bytesPerValueShift := uint(2)
	bytesPerValue := 1 << bytesPerValueShift
	bytesPerValueMask := bytesPerValue - 1
	dataIndex := 0
	leftoverBytes := 0
	for dataIndex < len(fits.Data) {
		bytesToRead := (len(fits.Data)-dataIndex)*bytesPerValue - leftoverBytes
		if bytesToRead > bufLen {
			bytesToRead = bufLen
		}
		bytesRead, err := r.Read(buf[leftoverBytes : leftoverBytes+bytesToRead])
		if err != nil {
			return fmt.Errorf("%d: %s", fits.ID, err.Error())
		}

		availableBytes := leftoverBytes + bytesRead
		for i := 0; i < (availableBytes &^ bytesPerValueMask); i += bytesPerValue {
			val := int32((uint32(buf[i]) << 24) | (uint32(buf[i+1]) << 16) | (uint32(buf[i+2]) << 8) | (uint32(buf[i+3])))
			v := float32(val)*fits.Bscale + fits.Bzero
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
			sum += float64(v)
			fits.Data[dataIndex+(i>>bytesPerValueShift)] = v
		}
		dataIndex += availableBytes >> bytesPerValueShift
		leftoverBytes = availableBytes & bytesPerValueMask
		for i := 0; i < leftoverBytes; i++ {
			buf[i] = buf[availableBytes-leftoverBytes+i]
		}
	}
	fits.Bzero, fits.Bscale = 0, 1 // reflect that data values incorporate these now
	mean := float32(sum / float64(len(fits.Data)))
	fits.Stats = stats.NewStatsWithMMM(fits.Data, fits.Naxisn[0], min, max, mean)
	return nil
}

// Batched read of data of the given size and type from the file, converting from network byte order and adjusting for Bzero
func (fits *Image) readInt64Data(r io.Reader) error {
	min, max, sum := float32(math.MaxFloat32), float32(-math.MaxFloat32), float64(0)
	fits.Data = make([]float32, int(fits.Pixels))
	buf := make([]byte, bufLen)

	bytesPerValueShift := uint(3)
	bytesPerValue := 1 << bytesPerValueShift
	bytesPerValueMask := bytesPerValue - 1
	dataIndex := 0
	leftoverBytes := 0
	for dataIndex < len(fits.Data) {
		bytesToRead := (len(fits.Data)-dataIndex)*bytesPerValue - leftoverBytes
		if bytesToRead > bufLen {
			bytesToRead = bufLen
		}
		bytesRead, err := r.Read(buf[leftoverBytes : leftoverBytes+bytesToRead])
		if err != nil {
			return fmt.Errorf("%d: %s", fits.ID, err.Error())
		}

		availableBytes := leftoverBytes + bytesRead
		for i := 0; i < (availableBytes &^ bytesPerValueMask); i += bytesPerValue {
			val := int64((uint64(buf[i]) << 56) | (uint64(buf[i+1]) << 48) | (uint64(buf[i+2]) << 40) | (uint64(buf[i+3]) << 32) |
				(uint64(buf[i+4]) << 24) | (uint64(buf[i+5]) << 16) | (uint64(buf[i+6]) << 8) | (uint64(buf[i+7])))
			v := float32(val)*fits.Bscale + fits.Bzero
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
			sum += float64(v)
			fits.Data[dataIndex+(i>>bytesPerValueShift)] = v
		}
		dataIndex += availableBytes >> bytesPerValueShift
		leftoverBytes = availableBytes & bytesPerValueMask
		for i := 0; i < leftoverBytes; i++ {
			buf[i] = buf[availableBytes-leftoverBytes+i]
		}
	}
	fits.Bzero, fits.Bscale = 0, 1 // reflect that data values incorporate these now
	mean := float32(sum / float64(len(fits.Data)))
	fits.Stats = stats.NewStatsWithMMM(fits.Data, fits.Naxisn[0], min, max, mean)
	return nil
}

// Batched read of data of the given size and type from the file, converting from network byte order and adjusting for Bzero
func (fits *Image) readFloat32Data(r io.Reader) error {
	min, max, sum := float32(math.MaxFloat32), float32(-math.MaxFloat32), float64(0)
	fits.Data = make([]float32, int(fits.Pixels))
	buf := make([]byte, bufLen)

	bytesPerValueShift := uint(2)
	bytesPerValue := 1 << bytesPerValueShift
	bytesPerValueMask := bytesPerValue - 1
	dataIndex := 0
	leftoverBytes := 0
	for dataIndex < len(fits.Data) {
		bytesToRead := (len(fits.Data)-dataIndex)*bytesPerValue - leftoverBytes
		if bytesToRead > bufLen {
			bytesToRead = bufLen
		}
		bytesRead, err := r.Read(buf[leftoverBytes : leftoverBytes+bytesToRead])
		if err != nil {
			return fmt.Errorf("%d: %s", fits.ID, err.Error())
		}

		availableBytes := leftoverBytes + bytesRead
		for i := 0; i < (availableBytes &^ bytesPerValueMask); i += bytesPerValue {
			bits := ((uint32(buf[i])) << 24) | (uint32(buf[i+1]) << 16) | (uint32(buf[i+2]) << 8) | (uint32(buf[i+3]))
			val := math.Float32frombits(bits)
			v := float32(val)*fits.Bscale + fits.Bzero
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
			sum += float64(v)
			fits.Data[dataIndex+(i>>bytesPerValueShift)] = v
		}
		dataIndex += availableBytes >> bytesPerValueShift
		leftoverBytes = availableBytes & bytesPerValueMask
		for i := 0; i < leftoverBytes; i++ {
			buf[i] = buf[availableBytes-leftoverBytes+i]
		}
	}
	fits.Bzero, fits.Bscale = 0, 1 // reflect that data values incorporate these now
	mean := float32(sum / float64(len(fits.Data)))
	fits.Stats = stats.NewStatsWithMMM(fits.Data, fits.Naxisn[0], min, max, mean)
	return nil
}

// Batched read of data of the given size and type from the file, converting from network byte order and adjusting for Bzero
func (fits *Image) readFloat64Data(r io.Reader) error {
	min, max, sum := float32(math.MaxFloat32), float32(-math.MaxFloat32), float64(0)
	fits.Data = make([]float32, int(fits.Pixels))
	buf := make([]byte, bufLen)

	bytesPerValueShift := uint(3)
	bytesPerValue := 1 << bytesPerValueShift
	bytesPerValueMask := bytesPerValue - 1
	dataIndex := 0
	leftoverBytes := 0
	for dataIndex < len(fits.Data) {
		bytesToRead := (len(fits.Data)-dataIndex)*bytesPerValue - leftoverBytes
		if bytesToRead > bufLen {
			bytesToRead = bufLen
		}
		bytesRead, err := r.Read(buf[leftoverBytes : leftoverBytes+bytesToRead])
		if err != nil {
			return fmt.Errorf("%d: %s", fits.ID, err.Error())
		}

		availableBytes := leftoverBytes + bytesRead
		for i := 0; i < (availableBytes &^ bytesPerValueMask); i += bytesPerValue {
			bits := ((uint64(buf[i]) << 56) | (uint64(buf[i+1]) << 48) | (uint64(buf[i+2]) << 40) | (uint64(buf[i+3]) << 32) |
				(uint64(buf[i+4]) << 24) | (uint64(buf[i+5]) << 16) | (uint64(buf[i+6]) << 8) | (uint64(buf[i+7])))
			val := math.Float64frombits(bits)
			v := float32(val)*fits.Bscale + fits.Bzero
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
			sum += float64(v)
			fits.Data[dataIndex+(i>>bytesPerValueShift)] = v
		}
		dataIndex += availableBytes >> bytesPerValueShift
		leftoverBytes = availableBytes & bytesPerValueMask
		for i := 0; i < leftoverBytes; i++ {
			buf[i] = buf[availableBytes-leftoverBytes+i]
		}
	}
	fits.Bzero, fits.Bscale = 0, 1 // reflect that data values incorporate these now
	mean := float32(sum / float64(len(fits.Data)))
	fits.Stats = stats.NewStatsWithMMM(fits.Data, fits.Naxisn[0], min, max, mean)
	return nil
}

func (h *Header) read(r io.Reader, id int, logWriter io.Writer) error {
	buf := make([]byte, fitsBlockSize)

	for h.Length = 0; !h.End; {
		// read next header unit
		bytesRead, err := io.ReadFull(r, buf)
		if err != nil || bytesRead != fitsBlockSize {
			return fmt.Errorf("%d: %s", id, err.Error())
		}
		h.Length += int32(bytesRead)

		// parse all lines in this header unit
		for lineNo := 0; lineNo < fitsBlockSize/HeaderLineSize && !h.End; lineNo++ {
			line := buf[lineNo*HeaderLineSize : (lineNo+1)*HeaderLineSize]
			subValues := reParser.FindSubmatch(line)
			if subValues == nil {
				fmt.Fprintf(logWriter, "%d: Warning:Cannot parse '%s', ignoring\n", id, string(line))
			} else {
				subNames := reParser.SubexpNames()
				h.readLine(subNames, subValues, id, lineNo, logWriter)
			}
		}
	}
	return nil
}

func (h *Header) readLine(subNames []string, subValues [][]byte, id, lineNo int, logWriter io.Writer) {
	key := ""
	// ignore index 0 which is the whole line
	for i := 1; i < len(subNames); i++ {
		if subValues[i] != nil && len(subNames[i]) == 1 {
			switch c := subNames[i][0]; c {
			case byte('E'): // end line
				h.End = true
			case byte('H'): // history line
				h.History = append(h.History, string(subValues[i]))
			case byte('C'): // comment line
				h.Comments = append(h.History, string(subValues[i]))
			case byte('k'): // key
				key = string(subValues[i])
			case byte('b'): // boolean
				if len(subValues[i]) > 0 {
					v := subValues[i][0]
					h.Bools[key] = v == byte('t') || v == byte('T')
				}
			case byte('i'): // int
				val, err := strconv.ParseInt(string(subValues[i]), 10, 64)
				if err == nil {
					h.Ints[key] = int32(val)
				}
			case byte('f'): // float
				val, err := strconv.ParseFloat(string(subValues[i]), 64)
				if err == nil {
					h.Floats[key] = float32(val)
				}
			case byte('s'): // string
				h.Strings[key] = string(subValues[i])
			case byte('d'): // date
				h.Dates[key] = string(subValues[i])
			case byte('c'): // comment
				// ignore value comments
			default:
				fmt.Fprintf(logWriter, "%d:%d:Warning:Unknown token '%s'\n", id, lineNo, string(c))
			}
		}
	}
}

func (h *Header) Print() {
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
	white := "\\s+"
	whiteOpt := "\\s*"
	whiteLine := white

	hist := "HISTORY"
	rest := ".*"
	histLine := hist + white + "(?P<H>" + rest + ")"

	commKey := "COMMENT"
	commLine := commKey + white + "(?P<C>" + rest + ")"

	end := "(?P<E>END)"
	endLine := end + whiteOpt

	key := "(?P<k>[A-Z0-9_-]+)"
	equals := "="

	boo := "(?P<b>[TF])"
	inte := "(?P<i>[+-]?[0-9]+)"
	floa := "(?P<f>[+-]?[0-9]*\\.[0-9]*(?:[ED][-+]?[0-9]+)?)"
	stri := "'(?P<s>[^']*)'"
	date := "(?P<d>[0-9]{1,4}-?[012][0-9]-?[0123][0-9]T[012][0-9]:?[0-5][0-9]:?[0-5][0-9].?[0-9]*)" // FIXME: other variants possible, see ISO8601
	val := "(?:" + boo + "|" + inte + "|" + floa + "|" + stri + "|" + date + ")"

	// missing: CONTINUE for strings
	// missing: complex int: (nr, nr)
	// missing: complex float: (nr, nr)

	commOpt := "(?:/(?P<c>.*))?"
	keyLine := key + whiteOpt + equals + whiteOpt + val + whiteOpt + commOpt

	lineRe := "^(?:" + whiteLine + "|" + histLine + "|" + commLine + "|" + keyLine + "|" + endLine + ")$"
	return regexp.MustCompile(lineRe)
}
