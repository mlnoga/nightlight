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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Load frame from FITS file and calculate basic stats and noise
func LoadAndCalcStats(fileName string, id int) (f *FITSImage, err error) {
	theF:=NewFITSImage()
	f=&theF
	f.ID=id
	err=f.ReadFile(fileName)
	if err!=nil { return nil, err }
	f.Stats=CalcBasicStats(f.Data)
	f.Stats.Noise=EstimateNoise(f.Data, f.Naxisn[0])
	return f, nil
}

// Load dark frame from FITS file
func LoadDark(fileName string) *FITSImage {
	f, err:=LoadAndCalcStats(fileName, -1)
	if err!=nil { panic(err.Error()) }

	LogPrintf("%s %s %dx%d stats: %v\n", "dark", fileName, f.Naxisn[0], f.Naxisn[1], f.Stats)
	if f.Stats.StdDev<1e-8 {
		LogPrintf("Warnining: dark file may be degenerate\n")
	}
	return f
}


// Load flat frame from FITS file
func LoadFlat(fileName string) *FITSImage {
	f, err:=LoadAndCalcStats(fileName, -2)
	if err!=nil { panic(err.Error()) }

	LogPrintf("%s %s %dx%d stats: %v\n", "flat", fileName, f.Naxisn[0], f.Naxisn[1], f.Stats)
	if (f.Stats.Min<=0 && f.Stats.Max>=0) || f.Stats.StdDev<1e-8 {
		LogPrintf("Warnining: flat file may be degenerate\n")
	}
	return f
}


// Load alignment target frame from FITS file
func LoadAlignTo(fileName string) *FITSImage {
	f, err:=LoadAndCalcStats(fileName, -3)
	if err!=nil { panic(err.Error()) }

	LogPrintf("%s %s %dx%d stats: %v\n", "align", fileName, f.Naxisn[0], f.Naxisn[1], f.Stats)
	if f.Stats.StdDev<1e-8 {
		LogPrintf("Warnining: alignment target file may be degenerate\n")
	}
	return f
}




type OpPreProcess struct {
	Calibrate   *OpCalibrate    `json:"calibrate"` 
	BadPixel    *OpBadPixel     `json:"badPixel"`
	Debayer     *OpDebayer      `json:"debayer"`
	Bin         *OpBin          `json:"bin"`
	BackExtract *OpBackExtract  `json:"backExtract"`
	StarDetect  *OpStarDetect   `json:"starDetect"` 
	Save        *OpSave         `json:"save"`
}
var _ OperatorUnary = (*OpPreProcess)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpPreProcess(dark, flat string, debayer, cfa string, binning int32, 
	bpSigLow, bpSigHigh, starSig, starBpSig, starInOut float32, starRadius int32, starsPattern string, 
	backGrid int32, backSigma float32, backClip int32, backPattern, preprocessedPattern string) *OpPreProcess {
	opDebayer:=NewOpDebayer(debayer, cfa)
	return &OpPreProcess{
		Calibrate   : NewOpCalibrate(dark, flat),
		BadPixel    : NewOpBadPixel(bpSigLow, bpSigHigh, opDebayer),
		Debayer     : opDebayer,
		Bin         : NewOpBin(binning),
		BackExtract : NewOpBackExtract(backGrid, backSigma, backClip, backPattern),
		StarDetect  : NewOpStarDetect(starRadius, starSig, starBpSig, starInOut, starsPattern),
		Save        : NewOpSave(preprocessedPattern),
	}	
}

func (op *OpPreProcess) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if op.Calibrate  !=nil { if f, err=op.Calibrate  .Apply(f, logWriter); err!=nil { return nil, err} }
	if op.BadPixel   !=nil { if f, err=op.BadPixel   .Apply(f, logWriter); err!=nil { return nil, err} }
	if op.Debayer    !=nil { if f, err=op.Debayer    .Apply(f, logWriter); err!=nil { return nil, err} }
	if op.Bin        !=nil { if f, err=op.Bin        .Apply(f, logWriter); err!=nil { return nil, err} }
	if op.BackExtract!=nil { if f, err=op.BackExtract.Apply(f, logWriter); err!=nil { return nil, err} }
	if op.StarDetect !=nil { if f, err=op.StarDetect .Apply(f, logWriter); err!=nil { return nil, err} }
	if op.Save       !=nil { if f, err=op.Save       .Apply(f, logWriter); err!=nil { return nil, err} }
	return f, nil
}

func (op *OpPreProcess) Init() (err error) { 
	if op.Calibrate  !=nil { if err=op.Calibrate  .Init(); err!=nil { return err } }
	if op.BadPixel   !=nil { if err=op.BadPixel   .Init(); err!=nil { return err } }
	if op.Debayer    !=nil { if err=op.Debayer    .Init(); err!=nil { return err } }
	if op.Bin        !=nil { if err=op.Bin        .Init(); err!=nil { return err } }
	if op.BackExtract!=nil { if err=op.BackExtract.Init(); err!=nil { return err } }
	if op.StarDetect !=nil { if err=op.StarDetect .Init(); err!=nil { return err } }
	if op.Save       !=nil { if err=op.Save       .Init(); err!=nil { return err } }
	return nil
 }




type OpCalibrate struct {
	ActiveDark        bool        `json:"activeDark"`
	Dark              string      `json:"dark"`
	DarkFrame         *FITSImage  `json:"-"`
	ActiveFlat        bool        `json:"activeFlat"`
	Flat              string      `json:"flat"`
	FlatFrame         *FITSImage  `json:"-"`
}
var _ OperatorUnary = (*OpCalibrate)(nil) // Compile time assertion: type implements the interface

func NewOpCalibrate(dark, flat string) *OpCalibrate {
	return &OpCalibrate{
		ActiveDark : dark!="",
		Dark       : dark,
		ActiveFlat : flat!="",
		Flat       : flat,
	}
}

// Load dark and flat in parallel if flagged
func (op *OpCalibrate) Init() error {
    sem    :=make(chan bool, 2) // limit parallelism to 2
    waiting:=0

    op.DarkFrame=nil
    if op.ActiveDark && op.Dark!="" { 
		waiting++ 
		go func() { 
			op.DarkFrame=LoadDark(op.Dark) 
			sem <- true
		}()
	}

	op.FlatFrame=nil
    if op.ActiveFlat && op.Flat!="" { 
		waiting++ 
    	go func() { 
    		op.FlatFrame=LoadFlat(op.Flat) 
			sem <- true
		}() 
	}

	for ; waiting>0; waiting-- {
		<- sem   // wait for goroutines to finish
	}

	if op.DarkFrame!=nil && op.FlatFrame!=nil && !EqualInt32Slice(op.DarkFrame.Naxisn, op.FlatFrame.Naxisn) {
		return errors.New(fmt.Sprintf("Error: dark dimensions %v differ from flat dimensions %v.", 
			                          op.DarkFrame.Naxisn, op.FlatFrame.Naxisn))
	}
	return nil
}


// Apply calibration frames if active and available. Must have been loaded
func (op *OpCalibrate) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if op.ActiveDark && op.DarkFrame!=nil && op.DarkFrame.Pixels>0 {
		if !EqualInt32Slice(f.Naxisn, op.DarkFrame.Naxisn) {
			return nil, errors.New(fmt.Sprintf("%d: Light dimensions %v differ from dark dimensions %v",
			                      f.ID, f.Naxisn, op.DarkFrame.Naxisn))
		}
		Subtract(f.Data, f.Data, op.DarkFrame.Data)
		f.Stats=nil // invalidate stats
	}

	if op.ActiveFlat && op.FlatFrame!=nil && op.FlatFrame.Pixels>0 {
		if !EqualInt32Slice(f.Naxisn, op.FlatFrame.Naxisn) {
			return nil, errors.New(fmt.Sprintf("%d: Light dimensions %v differ from flat dimensions %v",
			                      f.ID, f.Naxisn, op.FlatFrame.Naxisn))
		}
		Divide(f.Data, f.Data, op.FlatFrame.Data, op.FlatFrame.Stats.Max)
		f.Stats=nil // invalidate stats
	}
	return f, nil
}


type OpBadPixel struct {
	Active            bool        `json:"active"`
	SigmaLow          float32     `json:"sigmaLow"`
	SigmaHigh         float32     `json:"sigmaHigh"`
	Debayer           *OpDebayer  `json:"-"`
}
var _ OperatorUnary = (*OpBadPixel)(nil) // Compile time assertion: type implements the interface


func NewOpBadPixel(bpSigLow, bpSigHigh float32, debayer *OpDebayer) *OpBadPixel {
	return &OpBadPixel{
		Active    : bpSigLow>0 && bpSigHigh>0,
		SigmaLow  : bpSigLow, 
		SigmaHigh : bpSigHigh,
		Debayer   : debayer,
	}
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpBadPixel) UnmarshalJSON(data []byte) error {
	type defaults OpBadPixel
	def:=defaults{
		Active    : true,
		SigmaLow  : 3,
		SigmaHigh : 5,
		Debayer   : nil,
	}
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpBadPixel(def)
	return nil
}


// Apply bad pixel removal if active
func (op *OpBadPixel) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active ||  op.SigmaLow==0 || op.SigmaHigh==0 {
		return f, nil
	}
	if op.Debayer==nil || !op.Debayer.Active {
		var bpm []int32
		bpm, f.MedianDiffStats=BadPixelMap(f.Data, f.Naxisn[0], op.SigmaLow, op.SigmaHigh)
		mask:=CreateMask(f.Naxisn[0], 1.5)
		MedianFilterSparse(f.Data, bpm, mask)
		fmt.Fprintf(logWriter, "%d: Removed %d bad pixels (%.2f%%) with sigma low=%.2f high=%.2f\n", 
			        f.ID, len(bpm), 100.0*float32(len(bpm))/float32(f.Pixels), op.SigmaLow, op.SigmaHigh)
	} else {
		numRemoved,err:=CosmeticCorrectionBayer(f.Data, f.Naxisn[0], op.Debayer.Debayer, op.Debayer.ColorFilterArray, op.SigmaLow, op.SigmaHigh)
		if err!=nil { return nil, err }
		fmt.Fprintf(logWriter, "%d: Removed %d bad bayer pixels (%.2f%%) with sigma low=%.2f high=%.2f\n", 
			        f.ID, numRemoved, 100.0*float32(numRemoved)/float32(f.Pixels), op.SigmaLow, op.SigmaHigh)
	}
	return f, nil
}

func (op *OpBadPixel) Init() error { return nil }


type OpDebayer struct {
	Active            bool        `json:"active"`
	Debayer           string      `json:"debayer"`
	ColorFilterArray  string      `json:"colorFilterArray"`
}
var _ OperatorUnary = (*OpDebayer)(nil) // Compile time assertion: type implements the interface


func NewOpDebayer(debayer, cfa string) *OpDebayer {
	return &OpDebayer{
		Active           : debayer!="",
		Debayer          : debayer,
		ColorFilterArray : cfa,
	}
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpDebayer) UnmarshalJSON(data []byte) error {
	type defaults OpDebayer
	def:=defaults{
		Active    : false,
		Debayer   : "",
		ColorFilterArray : "RGGB",
	}
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpDebayer(def)
	return nil
}

// Apply debayering if active 
func (op *OpDebayer) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }

	f.Data, f.Naxisn[0], err=DebayerBilinear(f.Data, f.Naxisn[0], op.Debayer, op.ColorFilterArray)
	if err!=nil { return nil, err }
	f.Pixels=int32(len(f.Data))
	f.Naxisn[1]=f.Pixels/f.Naxisn[0]
	fmt.Fprintf(logWriter, "%d: Debayered channel %s from cfa %s, new size %dx%d\n", f.ID, op.Debayer, op.ColorFilterArray, f.Naxisn[0], f.Naxisn[1])

	return f, nil
}

func (op *OpDebayer) Init() error { return nil }


type OpBin struct {
	Active            bool        `json:"active"`
	BinSize           int32       `json:"binSize"`
}
var _ OperatorUnary = (*OpBin)(nil) // Compile time assertion: type implements the interface

func NewOpBin(binning int32) *OpBin {
	return &OpBin{
		Active  : binning>1,
		BinSize : binning,
	}
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpBin) UnmarshalJSON(data []byte) error {
	type defaults OpBin
	def:=defaults{
		Active    : false,
		BinSize   : 1,
	}
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpBin(def)
	return nil
}

// Apply binning if active
func (op *OpBin) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active || op.BinSize<1 { return f, nil }

	newF:=BinNxN(f, op.BinSize)
	fmt.Fprintf(logWriter, "%d: Applying %xx%d binning, new image size %dx%d\n", newF.ID, op.BinSize, newF.Naxisn[0], newF.Naxisn[1])

	return f, nil
}

func (op *OpBin) Init() error { return nil }


type OpBackExtract struct {
	Active            bool            `json:"active"`
    GridSize     	  int32           `json:"gridSize"`
    Sigma 		      float32         `json:"sigma"`
    NumBlocksToClip   int32           `json:"numBlocksToClip"`
    Save             *OpSave          `json:"save"`
} 
var _ OperatorUnary = (*OpBackExtract)(nil) // Compile time assertion: type implements the interface

func NewOpBackExtract(backGrid int32, backSigma float32, backClip int32, savePattern string) *OpBackExtract {
	return &OpBackExtract{
		Active          : backGrid>0,
	    GridSize     	: backGrid,
	    Sigma 		    : backSigma,
	    NumBlocksToClip : backClip,
	    Save            : NewOpSave(savePattern),
	}
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpBackExtract) UnmarshalJSON(data []byte) error {
	type defaults OpBackExtract
	def:=defaults{
		Active          : false,
	    GridSize     	: 256,
	    Sigma 		    : 1.5,
	    NumBlocksToClip : 0,
	    Save            : NewOpSave(""),
	}
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpBackExtract(def)
	return nil
}

// Apply background extraction if active
func (op *OpBackExtract) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active || op.GridSize<=0 { return f, nil }

	bg:=NewBackground(f.Data, f.Naxisn[0], op.GridSize, op.Sigma, op.NumBlocksToClip)
	fmt.Fprintf(logWriter, "%d: %s\n", f.ID, bg)

	if op.Save==nil || !op.Save.Active || op.Save.FilePattern=="" {
		// faster, does not materialize background image explicitly
		bg.Subtract(f.Data)
	} else { 
		bgData:=bg.Render()
		bgFits:=FITSImage{
			Header:NewFITSHeader(),
			Bitpix:-32,
			Bzero :0,
			Naxisn:f.Naxisn,
			Pixels:f.Pixels,
			Data  :bgData,
		}
		_,err:=op.Save.Apply(&bgFits, logWriter)
		if err!=nil { return nil, err }
		Subtract(f.Data, f.Data, bgData)
		bgFits.Data, bgData=nil, nil
	}
	f.Stats=nil // invalidate stats
	return f, nil
}

func (op *OpBackExtract) Init() error { return nil }


type OpStarDetect struct {
	Active            bool            `json:"active"`
    Radius            int32           `json:"radius"`
	Sigma             float32         `json:"sigma"`
    BadPixelSigma     float32         `json:"badPixelSigma"`
    InOutRatio        float32         `json:"inOutRatio"`
    Save             *OpSave          `json:"save"`
}
var _ OperatorUnary = (*OpStarDetect)(nil) // Compile time assertion: type implements the interface

func NewOpStarDetect(starRadius int32, starSig, starBpSig, starInOut float32, savePattern string) *OpStarDetect {  
	return &OpStarDetect{
		Active          : true,
	    Radius          : starRadius,
		Sigma           : starSig,
	    BadPixelSigma   : starBpSig,
	    InOutRatio      : starInOut,
	    Save            : NewOpSave(savePattern),
	}
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpStarDetect) UnmarshalJSON(data []byte) error {
	type defaults OpStarDetect
	def:=defaults{
		Active          : true,
	    Radius          : 16,
		Sigma           : 15,
	    BadPixelSigma   : -1,   // FIXME: auto-detect??
	    InOutRatio      : 1.4,  // FIXME: way too low??
	    Save            : NewOpSave(""),
	}
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpStarDetect(def)
	return nil
}

// Apply star detection if active. Calculates needed stats on demand if not current
func (op *OpStarDetect) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active { return f, nil }

	if f.Stats==nil {
		f.Stats, err=CalcExtendedStats(f.Data, f.Naxisn[0])
		if err!=nil { return nil, err }
	}

	f.Stars, _, f.HFR=FindStars(f.Data, f.Naxisn[0], f.Stats.Location, f.Stats.Scale, op.Sigma, op.BadPixelSigma, op.InOutRatio, op.Radius, f.MedianDiffStats)
	fmt.Fprintf(logWriter, "%d: Stars %d HFR %.3g %v\n", f.ID, len(f.Stars), f.HFR, f.Stats)

	if op.Save!=nil && op.Save.Active {
		stars:=ShowStars(f, 2.0)
		_, err=op.Save.Apply(&stars, logWriter)
		if err!=nil { return nil, err }
	}	

	return f, nil
}

func (op *OpStarDetect) Init() error { return nil }


type OpSave struct {
	Active            bool            `json:"active"`
	FilePattern       string          `json:"filePattern"`
}
var _ OperatorUnary = (*OpSave)(nil) // Compile time assertion: type implements the interface

func NewOpSave(filenamePattern string) *OpSave {
	return &OpSave{
		Active      : filenamePattern!="",
		FilePattern : filenamePattern,
	}
}

// Apply saving if active
func (op *OpSave) Apply(f *FITSImage, logWriter io.Writer) (fOut *FITSImage, err error) {
	if !op.Active || op.FilePattern=="" { return f, nil }

	fileName:=fmt.Sprintf(op.FilePattern, f.ID)
	fnLower:=strings.ToLower(fileName)

	if strings.HasSuffix(fnLower,".fits")      || strings.HasSuffix(fnLower,".fit")      || strings.HasSuffix(fnLower,".fts")     ||
	   strings.HasSuffix(fnLower,".fits.gz")   || strings.HasSuffix(fnLower,".fit.gz")   || strings.HasSuffix(fnLower,".fts.gz")  ||
	   strings.HasSuffix(fnLower,".fits.gzip") || strings.HasSuffix(fnLower,".fit.gzip") || strings.HasSuffix(fnLower,".fts.gzip") {     
		fmt.Fprintf(logWriter,"%d: Writing FITS to %s", f.ID, fileName)
		err=f.WriteFile(fileName)
	} else if strings.HasSuffix(fnLower,".jpeg") || strings.HasSuffix(fnLower,".jpg") {
		if len(f.Naxisn)==1 {
			fmt.Fprintf(logWriter, "%d: Writing %s pixel mono JPEG to %s ...\n", f.ID, f.DimensionsToString(), fileName)
			f.WriteMonoJPGToFile(fileName, 95)
		} else if len(f.Naxisn)==3 {
			fmt.Fprintf(logWriter, "%d: Writing %s pixel color JPEG to %s ...\n", f.ID, f.DimensionsToString(), fileName)
			f.WriteJPGToFile(fileName, 95)
		} else {
			return nil, errors.New(fmt.Sprintf("%d: Unable to write %s pixel image as JPEG to %s\n", f.ID, f.DimensionsToString(), fileName))
		}
	}
	if err!=nil { return nil, errors.New(fmt.Sprintf("%d: Error writing to file %s: %s\n", f.ID, fileName, err)) }
	return f, nil;
}

func (op *OpSave) Init() error { return nil }



type OpParallel struct {
	Operator    OperatorUnary  `json:"operator"`
	MaxThreads  int64          `json:"MaxThreads"`
}
var _ OperatorParallel = (*OpParallel)(nil) // Compile time assertion: type implements the interface

func NewOpParallel(operator OperatorUnary, maxThreads int64) *OpParallel {
	return &OpParallel{operator, maxThreads} 
}

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func (op *OpParallel) ApplyToFiles(sources []*OpLoadFile, logWriter io.Writer) (fOuts []*FITSImage, err error) {
	fOuts =make([]*FITSImage, len(sources))
	sem  :=make(chan bool, op.MaxThreads)
	for i, src := range(sources) {
		sem <- true 
		go func(i int, source *OpLoadFile) {
			defer func() { <-sem }()
			f, err:=source.Apply(logWriter)
			if err!=nil { 
				fmt.Fprintf(logWriter, err.Error())
				return
			}
			f, err=op.Operator.Apply(f, logWriter)
			if err!=nil { 
				fmt.Fprintf(logWriter, err.Error())
				return
			}
			fOuts[i]=f
		}(i, src)
	}
	for i:=0; i<cap(sem); i++ {  // wait for goroutines to finish
		sem <- true
	}
	return fOuts, nil  // FIXME: not collecting and aggregating errors from the workers!!
}


// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func (op *OpParallel) ApplyToFITS(sources []*FITSImage, logWriter io.Writer) (fOuts []*FITSImage, err error) {
	fOuts =make([]*FITSImage, len(sources))
	sem  :=make(chan bool, op.MaxThreads)
	for i, src := range(sources) {
		sem <- true 
		go func(i int, f *FITSImage) {
			defer func() { <-sem }()
			f, err=op.Operator.Apply(f, logWriter)
			if err!=nil { 
				fmt.Fprintf(logWriter, err.Error())
				return
			}
			fOuts[i]=f
		}(i, src)
	}
	for i:=0; i<cap(sem); i++ {  // wait for goroutines to finish
		sem <- true
	}
	return fOuts, nil  // FIXME: not collecting and aggregating errors from the workers!!
}


