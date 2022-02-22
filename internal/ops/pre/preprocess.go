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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/star"
	"github.com/mlnoga/nightlight/internal/ops"
)


type OpPreProcess struct {
	Calibrate   *OpCalibrate    `json:"calibrate"` 
	BadPixel    *OpBadPixel     `json:"badPixel"`
	Debayer     *OpDebayer      `json:"debayer"`
	Bin         *OpBin          `json:"bin"`
	BackExtract *OpBackExtract  `json:"backExtract"`
	StarDetect  *OpStarDetect   `json:"starDetect"` 
	Save        *ops.OpSave         `json:"save"`
}
var _ ops.OperatorUnary = (*OpPreProcess)(nil) // Compile time assertion: type implements the interface

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpPreProcess(opCalibrate *OpCalibrate, opBadPixel *OpBadPixel, opDebayer *OpDebayer,
                     opBin *OpBin, opBackExtract *OpBackExtract, opStarDetect *OpStarDetect,
                     opSave *ops.OpSave) *OpPreProcess {
	return &OpPreProcess{
		Calibrate   : opCalibrate, 
		BadPixel    : opBadPixel,
		Debayer     : opDebayer,
		Bin         : opBin,
		BackExtract : opBackExtract,
		StarDetect  : opStarDetect,
		Save        : opSave,
	}	
}


// func NewOpPreProcess(dark, flat string, debayer, cfa string, binning int32, 
// 	bpSigLow, bpSigHigh, starSig, starBpSig, starInOut float32, starRadius int32, starsPattern string, 
// 	backGrid int32, backSigma float32, backClip int32, backPattern, preprocessedPattern string) *OpPreProcess {
// 	opDebayer:=NewOpDebayer(debayer, cfa)
// 	return &OpPreProcess{
// 		Calibrate   : NewOpCalibrate(dark, flat),
// 		BadPixel    : NewOpBadPixel(bpSigLow, bpSigHigh, opDebayer),
// 		Debayer     : opDebayer,
// 		Bin         : NewOpBin(binning),
// 		BackExtract : NewOpBackExtract(backGrid, backSigma, backClip, backPattern),
// 		StarDetect  : NewOpStarDetect(starRadius, starSig, starBpSig, starInOut, starsPattern),
// 		Save        : ops.NewOpSave(preprocessedPattern),
// 	}	
// }

func (op *OpPreProcess) Apply(f *fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) {
	if op.Calibrate  !=nil { if f, err=op.Calibrate  .Apply(f, logWriter); err!=nil { return nil, err} }
	if op.BadPixel   !=nil { if f, err=op.BadPixel   .Apply(f, logWriter); err!=nil { return nil, err} }
	if op.Debayer    !=nil { if f, err=op.Debayer    .Apply(f, logWriter); err!=nil { return nil, err} }
	if op.Bin        !=nil { if f, err=op.Bin        .Apply(f, logWriter); err!=nil { return nil, err} }
	if op.BackExtract!=nil { if f, err=op.BackExtract.Apply(f, logWriter); err!=nil { return nil, err} }
	if op.StarDetect !=nil { if f, err=op.StarDetect .Apply(f, logWriter); err!=nil { return nil, err} }
	if op.Save       !=nil { if f, err=op.Save       .Apply(f, logWriter); err!=nil { return nil, err} }
	return f, nil
}


type OpCalibrate struct {
	ActiveDark        bool        `json:"activeDark"`
	Dark              string      `json:"dark"`
	DarkFrame         *fits.Image  `json:"-"`
	ActiveFlat        bool        `json:"activeFlat"`
	Flat              string      `json:"flat"`
	FlatFrame         *fits.Image  `json:"-"`
	mutex             sync.Mutex  `json:"-"`
}
var _ ops.OperatorUnary = (*OpCalibrate)(nil) // Compile time assertion: type implements the interface

func NewOpCalibrate(dark, flat string) *OpCalibrate {
	return &OpCalibrate{
		ActiveDark : dark!="",
		Dark       : dark,
		ActiveFlat : flat!="",
		Flat       : flat,
	}
}

// Load dark and flat in parallel if flagged
func (op *OpCalibrate) init(logWriter io.Writer) error {
	op.mutex.Lock()
	defer op.mutex.Unlock()
  if !( (op.ActiveDark && op.Dark!="" && op.DarkFrame==nil) ||
          (op.ActiveFlat && op.Flat!="" && op.FlatFrame==nil)    ) {
        	return nil  
	}

  sem    :=make(chan error, 2) // limit parallelism to 2
  waiting:=0

  op.DarkFrame=nil
  if op.ActiveDark && op.Dark!="" { 
		waiting++ 
		go func() { 
			var err error
			op.DarkFrame, err=ops.LoadAndCalcStats(op.Dark, -1, "dark", logWriter)
			sem <- err
		}()
	}

	op.FlatFrame=nil
  if op.ActiveFlat && op.Flat!="" { 
		waiting++ 
  	go func() { 
			var err error
  		op.FlatFrame, err=ops.LoadAndCalcStats(op.Flat, -2, "flat", logWriter) 
		  sem <- err
	  }() 
  }

  var err error
	for ; waiting>0; waiting-- {
		threadErr := <- sem   // wait for goroutines to finish
		if threadErr!=nil {
			if err==nil {
				err=threadErr
			} else {
				 err=errors.New("Multiple errors: " + err.Error() + " and " + threadErr.Error())
			}
		}
	}
	if err!=nil {
		return err
	}

	if op.DarkFrame!=nil && op.FlatFrame!=nil && !fits.EqualInt32Slice(op.DarkFrame.Naxisn, op.FlatFrame.Naxisn) {
		return errors.New(fmt.Sprintf("Error: dark dimensions %v differ from flat dimensions %v.", 
			                          op.DarkFrame.Naxisn, op.FlatFrame.Naxisn))
	}
	return nil
}


// Apply calibration frames if active and available. Must have been loaded
func (op *OpCalibrate) Apply(f *fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) {
	if err=op.init(logWriter); err!=nil { return nil, err }

	if op.ActiveDark && op.DarkFrame!=nil && op.DarkFrame.Pixels>0 {
		if !fits.EqualInt32Slice(f.Naxisn, op.DarkFrame.Naxisn) {
			return nil, errors.New(fmt.Sprintf("%d: Light dimensions %v differ from dark dimensions %v",
			                      f.ID, f.Naxisn, op.DarkFrame.Naxisn))
		}
		Subtract(f.Data, f.Data, op.DarkFrame.Data)
		f.Stats.Clear()
	}

	if op.ActiveFlat && op.FlatFrame!=nil && op.FlatFrame.Pixels>0 {
		if !fits.EqualInt32Slice(f.Naxisn, op.FlatFrame.Naxisn) {
			return nil, errors.New(fmt.Sprintf("%d: Light dimensions %v differ from flat dimensions %v",
			                      f.ID, f.Naxisn, op.FlatFrame.Naxisn))
		}
		Divide(f.Data, f.Data, op.FlatFrame.Data, op.FlatFrame.Stats.Max())
		f.Stats.Clear()
	}
	return f, nil
}


type OpBadPixel struct {
	Active            bool        `json:"active"`
	SigmaLow          float32     `json:"sigmaLow"`
	SigmaHigh         float32     `json:"sigmaHigh"`
	Debayer           *OpDebayer  `json:"-"`
}
var _ ops.OperatorUnary = (*OpBadPixel)(nil) // Compile time assertion: type implements the interface


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
func (op *OpBadPixel) Apply(f *fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) {
	if !op.Active ||  op.SigmaLow==0 || op.SigmaHigh==0 {
		return f, nil
	}
	if op.Debayer==nil || !op.Debayer.Active {
		var bpm []int32
		bpm, f.MedianDiffStats=BadPixelMap(f.Data, f.Naxisn[0], op.SigmaLow, op.SigmaHigh)
		mask:=star.CreateMask(f.Naxisn[0], 1.5)
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


type OpDebayer struct {
	Active            bool        `json:"active"`
	Debayer           string      `json:"debayer"`
	ColorFilterArray  string      `json:"colorFilterArray"`
}
var _ ops.OperatorUnary = (*OpDebayer)(nil) // Compile time assertion: type implements the interface


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
func (op *OpDebayer) Apply(f *fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) {
	if !op.Active { return f, nil }

	f.Data, f.Naxisn[0], err=DebayerBilinear(f.Data, f.Naxisn[0], op.Debayer, op.ColorFilterArray)
	if err!=nil { return nil, err }
	f.Pixels=int32(len(f.Data))
	f.Naxisn[1]=f.Pixels/f.Naxisn[0]
	fmt.Fprintf(logWriter, "%d: Debayered channel %s from cfa %s, new size %dx%d\n", f.ID, op.Debayer, op.ColorFilterArray, f.Naxisn[0], f.Naxisn[1])

	return f, nil
}


type OpBin struct {
	Active            bool        `json:"active"`
	BinSize           int32       `json:"binSize"`
}
var _ ops.OperatorUnary = (*OpBin)(nil) // Compile time assertion: type implements the interface

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
func (op *OpBin) Apply(f *fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) {
	if !op.Active || op.BinSize<1 { return f, nil }

	newF:=fits.NewImageBinNxN(f, op.BinSize)
	fmt.Fprintf(logWriter, "%d: Applying %xx%d binning, new image size %dx%d\n", newF.ID, op.BinSize, op.BinSize, newF.Naxisn[0], newF.Naxisn[1])

	return f, nil
}


type OpBackExtract struct {
	Active            bool            `json:"active"`
    GridSize     	  int32           `json:"gridSize"`
    Sigma 		      float32         `json:"sigma"`
    NumBlocksToClip   int32           `json:"numBlocksToClip"`
    Save             *ops.OpSave          `json:"save"`
} 
var _ ops.OperatorUnary = (*OpBackExtract)(nil) // Compile time assertion: type implements the interface

func NewOpBackExtract(backGrid int32, backSigma float32, backClip int32, savePattern string) *OpBackExtract {
	return &OpBackExtract{
		Active          : backGrid>0,
	    GridSize     	: backGrid,
	    Sigma 		    : backSigma,
	    NumBlocksToClip : backClip,
	    Save            : ops.NewOpSave(savePattern),
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
	    Save            : ops.NewOpSave(""),
	}
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpBackExtract(def)
	return nil
}

// Apply background extraction if active
func (op *OpBackExtract) Apply(f *fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) {
	if !op.Active || op.GridSize<=0 { return f, nil }

	bg:=NewBackground(f.Data, f.Naxisn[0], op.GridSize, op.Sigma, op.NumBlocksToClip, logWriter)
	fmt.Fprintf(logWriter, "%d: %s\n", f.ID, bg)

	if op.Save==nil || !op.Save.Active || op.Save.FilePattern=="" {
		// faster, does not materialize background image explicitly
		err=bg.Subtract(f.Data)
		if err!=nil { return nil, err }
	} else { 
		bgData:=bg.Render()
		bgFits:=fits.NewImageFromNaxisn(f.Naxisn, bgData)
		_,err:=op.Save.Apply(bgFits, logWriter)
		if err!=nil { return nil, err }
		Subtract(f.Data, f.Data, bgData)
		bgFits.Data, bgData=nil, nil
	}
	f.Stats.Clear()
	return f, nil
}


type OpStarDetect struct {
	Active            bool            `json:"active"`
    Radius            int32           `json:"radius"`
	Sigma             float32         `json:"sigma"`
    BadPixelSigma     float32         `json:"badPixelSigma"`
    InOutRatio        float32         `json:"inOutRatio"`
    Save             *ops.OpSave          `json:"save"`
}
var _ ops.OperatorUnary = (*OpStarDetect)(nil) // Compile time assertion: type implements the interface

func NewOpStarDetect(starRadius int32, starSig, starBpSig, starInOut float32, savePattern string) *OpStarDetect {  
	return &OpStarDetect{
		Active          : true,
	    Radius          : starRadius,
		Sigma           : starSig,
	    BadPixelSigma   : starBpSig,
	    InOutRatio      : starInOut,
	    Save            : ops.NewOpSave(savePattern),
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
	    Save            : ops.NewOpSave(""),
	}
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpStarDetect(def)
	return nil
}

// Apply star detection if active. Calculates needed stats on demand if not current
func (op *OpStarDetect) Apply(f *fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) {
	if !op.Active { return f, nil }

	if f.Stats==nil {
		panic("nil stats")
	}

	f.Stars, _, f.HFR=star.FindStars(f.Data, f.Naxisn[0], f.Stats.Location(), f.Stats.Scale(), op.Sigma, op.BadPixelSigma, op.InOutRatio, op.Radius, f.MedianDiffStats)
	fmt.Fprintf(logWriter, "%d: Stars %d HFR %.3g %v\n", f.ID, len(f.Stars), f.HFR, f.Stats)

	if op.Save!=nil && op.Save.Active {
		stars:=fits.NewImageFromStars(f, 2.0)
		_, err=op.Save.Apply(stars, logWriter)
		if err!=nil { return nil, err }
	}	

	return f, nil
}

