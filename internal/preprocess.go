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
	"errors"
	"fmt"
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



// Load dark and flat in parallel if flagged
func (cf *CalibrationFrames) Load(set *CalibrationFrameSettings) error {
    sem    :=make(chan bool, 2) // limit parallelism to 2
    waiting:=0

    cf.Dark=nil
    if set.ActiveDark && set.Dark!="" { 
		waiting++ 
		go func() { 
			cf.Dark=LoadDark(set.Dark) 
			sem <- true
		}()
	}

	cf.Flat=nil
    if set.ActiveFlat && set.Flat!="" { 
		waiting++ 
    	go func() { 
    		cf.Flat=LoadFlat(set.Flat) 
			sem <- true
		}() 
	}

	for ; waiting>0; waiting-- {
		<- sem   // wait for goroutines to finish
	}

	if cf.Dark!=nil && cf.Flat!=nil && !EqualInt32Slice(cf.Dark.Naxisn, cf.Flat.Naxisn) {
		return errors.New(fmt.Sprintf("Error: dark dimensions %v differ from flat dimensions %v.", 
			                          cf.Dark.Naxisn, cf.Flat.Naxisn))
	}
	return nil
}



type PreProcessingJob struct {
	Items     []WorkItem
	Settings    PreProcessingSettings
}

type WorkItem struct {
	ID 		    int
	FileName    string
}

type PreProcessingSettings struct {
	Name        string                        `json:"name"`
	Calibration CalibrationFrameSettings      `json:"calibration"`
	BadPixel    BadPixelRemovalSettings       `json:"badPixel"`
	Debayer     DebayerSettings               `json:"debayer"`
	Binning     BinningSettings               `json:"binning"`
	BackExtract BackgroundExtractionSettings  `json:"backExtract"`
	StarDetect  StarDetectionSettings         `json:"starDetect"`
	Save        SavingSettings                `json:"save"`
}

type CalibrationFrameSettings struct {
	ActiveDark        bool        `json:"activeDark"`
	Dark              string      `json:"dark"`
	ActiveFlat        bool        `json:"activeFlat"`
	Flat              string      `json:"flat"`
}

type CalibrationFrames struct {
	Dark              *FITSImage
	Flat              *FITSImage
}

type BadPixelRemovalSettings struct {
	Active            bool        `json:"active"`
	SigmaLow          float32     `json:"sigmaLow"`
	SigmaHigh         float32     `json:"sigmaHigh"`
}

type DebayerSettings struct {
	Active            bool        `json:"active"`
	Debayer           string      `json:"debayer"`
	ColorFilterArray  string      `json:"colorFilterArray"`
}

type BinningSettings struct {
	Active            bool        `json:"active"`
	BinSize           int32       `json:"binSize"`
}

type BackgroundExtractionSettings struct {
	Active            bool            `json:"active"`
    GridSize     	  int32           `json:"gridSize"`
    Sigma 		      float32         `json:"sigma"`
    NumBlocksToClip   int32           `json:"numBlocksToClip"`
    Save              SavingSettings  `json:"save"`
} 

type StarDetectionSettings struct {
	Active            bool            `json:"active"`
    Radius            int32           `json:"radius"`
	Sigma             float32         `json:"sigma"`
    BadPixelSigma     float32         `json:"badPixelSigma"`
    InOutRatio        float32         `json:"inOutRatio"`
    Save              SavingSettings  `json:"save"`
}

type SavingSettings struct {
	Active            bool            `json:"active"`
	FilePattern       string          `json:"filePattern"`
}

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func PreProcessLightSettings(dark, flat string, debayer, cfa string, binning, normRange int32, 
	bpSigLow, bpSigHigh, starSig, starBpSig, starInOut float32, starRadius int32, starsPattern string, 
	backGrid int32, backSigma float32, backClip int32, backPattern, preprocessedPattern string) *PreProcessingSettings {
	return &PreProcessingSettings{
		Name: "command line parameters",
			Calibration : CalibrationFrameSettings{
			ActiveDark : dark!="",
			Dark       : dark,
			ActiveFlat : flat!="",
			Flat       : flat,
		},
		BadPixel    : BadPixelRemovalSettings{
			Active    : bpSigLow!=0 && bpSigHigh!=0,
			SigmaLow  : bpSigLow, 
			SigmaHigh : bpSigHigh,
		},
		Debayer     : DebayerSettings{
			Active           : debayer!="",
			Debayer          : debayer,
			ColorFilterArray : cfa,
		},
		Binning     : BinningSettings{
			Active  : binning>1,
			BinSize : binning,
		},
		BackExtract : BackgroundExtractionSettings{
			Active          : backGrid>0,
		    GridSize     	: backGrid,
		    Sigma 		    : backSigma,
		    NumBlocksToClip : backClip,
		    Save            : SavingSettings {
				Active      : backPattern!="",
				FilePattern : backPattern,
			},
		},
		StarDetect  : StarDetectionSettings{
			Active          : true,
		    Radius          : starRadius,
			Sigma           : starSig,
		    BadPixelSigma   : starBpSig,
		    InOutRatio      : starInOut,
		    Save            : SavingSettings {
				Active      : starsPattern!="",
				FilePattern : starsPattern,
			},
		},
		Save        : SavingSettings{
			Active      : preprocessedPattern!="",
			FilePattern : preprocessedPattern,
		},		
	}	
}

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func PreProcessLights(ids []int, fileNames []string, set *PreProcessingSettings, calibF *CalibrationFrames, imageLevelParallelism int32) (lights []*FITSImage) {
	lights =make([]*FITSImage, len(fileNames))
	sem   :=make(chan bool, imageLevelParallelism)
	for i, fileName := range(fileNames) {
		id:=ids[i]
		sem <- true 
		go func(i int, id int, fileName string) {
			defer func() { <-sem }()
			lightP, logMsg, err:=PreProcessLight(id, fileName, set, calibF)
			LogPrintf("%s", logMsg)
			if err!=nil {
				LogPrintf("%d: Error: %s\n", id, err.Error())
			} else {
				lights[i]=lightP
			}
		}(i, id, fileName)
	}
	for i:=0; i<cap(sem); i++ {  // wait for goroutines to finish
		sem <- true
	}
	return lights	
}


// Preprocess a single light frame with given settings.
// Pre-processing includes loading, basic statistics, dark subtraction, flat division, 
// bad pixel removal, star detection and HFR calculation.
func PreProcessLight(id int, fileName string, set *PreProcessingSettings, calibF *CalibrationFrames) (f *FITSImage, logMsgs string, err error) {
	theF:=NewFITSImage()
	theF.ID=id
	f=&theF

	logMsg:=fmt.Sprintf("%d: Loading light frame from %s\n", f.ID, fileName)
	err=f.ReadFile(fileName)
	logMsgs+=logMsg;
	if err!=nil { return nil, "", err }

	logMsg, err=ApplyCalibrationFrames(f, &(set.Calibration), calibF);
	logMsgs+=logMsg;
	if err!=nil { return nil, logMsgs, err }

	logMsg, err=ApplyBadPixelRemoval(f, &(set.BadPixel), &(set.Debayer));
	logMsgs+=logMsg;
	if err!=nil { return nil, logMsgs, err }

	logMsg, err=ApplyDebayering(f, &(set.Debayer))
	logMsgs+=logMsg;
	if err!=nil { return nil, logMsgs, err }

	f, logMsg, err=ApplyBinning(f, &(set.Binning))
	logMsgs+=logMsg;
	if err!=nil { return nil, logMsgs, err }

	logMsg, err=ApplyBackgroundExtraction(f, &(set.BackExtract))
	logMsgs+=logMsg;
	if err!=nil { return nil, logMsgs, err }

	logMsg, err=ApplyStarDetection(f, &(set.StarDetect))
	logMsgs+=logMsg;
	if err!=nil { return nil, logMsgs, err }

	return f, logMsgs, nil
}



// Apply calibration frames if active and available
func ApplyCalibrationFrames(f *FITSImage, set *CalibrationFrameSettings, calibF *CalibrationFrames) (logMsg string, err error) {
	if set.ActiveDark && calibF.Dark!=nil && calibF.Dark.Pixels>0 {
		if !EqualInt32Slice(f.Naxisn, calibF.Dark.Naxisn) {
			return "", errors.New(fmt.Sprintf("%d: Light dimensions %v differ from dark dimensions %v",
			                      f.ID, f.Naxisn, calibF.Dark.Naxisn))
		}
		Subtract(f.Data, f.Data, calibF.Dark.Data)
		f.Stats=nil // invalidate stats
	}

	if set.ActiveFlat && calibF.Flat!=nil && calibF.Flat.Pixels>0 {
		if !EqualInt32Slice(f.Naxisn, calibF.Flat.Naxisn) {
			return "", errors.New(fmt.Sprintf("%d: Light dimensions %v differ from flat dimensions %v",
			                      f.ID, f.Naxisn, calibF.Dark.Naxisn))
		}
		Divide(f.Data, f.Data, calibF.Flat.Data, calibF.Flat.Stats.Max)
		f.Stats=nil // invalidate stats
	}
	return "", nil
}

// Apply bad pixel removal if active
func ApplyBadPixelRemoval(f *FITSImage, set *BadPixelRemovalSettings, bay *DebayerSettings) (logMsg string, err error) {
	if !set.Active ||  set.SigmaLow==0 || set.SigmaHigh==0 {
		return "", nil
	}
	if !bay.Active {
		var bpm []int32
		bpm, f.MedianDiffStats=BadPixelMap(f.Data, f.Naxisn[0], set.SigmaLow, set.SigmaHigh)
		mask:=CreateMask(f.Naxisn[0], 1.5)
		MedianFilterSparse(f.Data, bpm, mask)
		logMsg=fmt.Sprintf("%d: Removed %d bad pixels (%.2f%%) with sigma low=%.2f high=%.2f\n", 
			f.ID, len(bpm), 100.0*float32(len(bpm))/float32(f.Pixels), set.SigmaLow, set.SigmaHigh)
	} else {
		numRemoved,err:=CosmeticCorrectionBayer(f.Data, f.Naxisn[0], bay.Debayer, bay.ColorFilterArray, set.SigmaLow, set.SigmaHigh)
		if err!=nil { return "", err }
		logMsg=fmt.Sprintf("%d: Removed %d bad bayer pixels (%.2f%%) with sigma low=%.2f high=%.2f\n", 
			f.ID, numRemoved, 100.0*float32(numRemoved)/float32(f.Pixels), set.SigmaLow, set.SigmaHigh)
	}
	return logMsg, nil
}

// Apply debayering if active 
func ApplyDebayering(f *FITSImage, set *DebayerSettings) (logMsg string, err error) {
	if !set.Active { return "", nil }

	f.Data, f.Naxisn[0], err=DebayerBilinear(f.Data, f.Naxisn[0], set.Debayer, set.ColorFilterArray)
	if err!=nil { return "", err }
	f.Pixels=int32(len(f.Data))
	f.Naxisn[1]=f.Pixels/f.Naxisn[0]
	logMsg=fmt.Sprintf("%d: Debayered channel %s from cfa %s, new size %dx%d\n", f.ID, set.Debayer, set.ColorFilterArray, f.Naxisn[0], f.Naxisn[1])

	return logMsg, nil
}

// Apply binning if active
func ApplyBinning(f *FITSImage, set *BinningSettings) (fPrime *FITSImage, logMsg string, err error) {
	if !set.Active || set.BinSize<1 { return f, "", nil }

	newF:=BinNxN(f, set.BinSize)
	f=&newF
	logMsg=fmt.Sprintf("%d: Applying %xx%d binning, new image size %dx%d\n", f.ID, set.BinSize,f.Naxisn[0], f.Naxisn[1])

	return f, logMsg, nil
}

// Apply background extraction if active
func ApplyBackgroundExtraction(f *FITSImage, set *BackgroundExtractionSettings) (logMsg string, err error) {
	if !set.Active || set.GridSize<=0 { return "", nil }

	bg:=NewBackground(f.Data, f.Naxisn[0], set.GridSize, set.Sigma, set.NumBlocksToClip)
	logMsg=fmt.Sprintf("%d: %s\n", f.ID, bg)

	if !set.Save.Active || set.Save.FilePattern=="" {
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
		fileName:=fmt.Sprintf(set.Save.FilePattern, f.ID)
		err=bgFits.WriteFile(fileName)
		if err!=nil { return logMsg, errors.New(fmt.Sprintf("%d: Error writing background file %s: %s\n", f.ID, fileName, err)) }
		Subtract(f.Data, f.Data, bgData)
		bgFits.Data, bgData=nil, nil
	}
	f.Stats=nil // invalidate stats
	return logMsg, nil
}

// Apply star detection if active. Calculates needed stats on demand if not current
func ApplyStarDetection(f *FITSImage, set *StarDetectionSettings) (logMsg string, err error) {
	if !set.Active { return "", nil }

	if f.Stats==nil {
		f.Stats, err=CalcExtendedStats(f.Data, f.Naxisn[0])
		if err!=nil { return logMsg, err }
	}

	f.Stars, _, f.HFR=FindStars(f.Data, f.Naxisn[0], f.Stats.Location, f.Stats.Scale, set.Sigma, set.BadPixelSigma, set.InOutRatio, set.Radius, f.MedianDiffStats)
	logMsg=fmt.Sprintf("%d: Stars %d HFR %.3g %v\n", f.ID, len(f.Stars), f.HFR, f.Stats)

	if set.Save.Active && set.Save.FilePattern!="" {
		stars:=ShowStars(f, 2.0)
		fileName:=fmt.Sprintf(set.Save.FilePattern, f.ID)
		err=stars.WriteFile(fileName)
		if err!=nil { return logMsg, errors.New(fmt.Sprintf("%d: Error writing star detections to file %s: %s\n", f.ID, fileName, err)) }
	}	

	return logMsg, nil
}

// Apply saving if active
func ApplySave(f *FITSImage, set *SavingSettings) (logMsg string, err error) {
	if !set.Active || set.FilePattern=="" { return "", nil }

	fileName:=fmt.Sprintf(set.FilePattern, f.ID)
	err=f.WriteFile(fileName)
	if err!=nil { return logMsg, errors.New(fmt.Sprintf("%d: Error writing preprocessed file %s: %s\n", f.ID, fileName, err)) }

	return logMsg,nil;
}



// Reference frame selection mode
type RefSelMode int
const (
	RFMStarsOverHFR = iota   // Pick frame with highest ratio of stars over HFR (for lights)
	RFMMedianLoc             // Pick frame with median location (for multiplicative correction when integrating master flats)
)


// Select reference frame by maximizing the number of stars divided by HFR
func SelectReferenceFrame(lights []*FITSImage, mode RefSelMode) (refFrame *FITSImage, refScore float32) {
	refFrame, refScore=(*FITSImage)(nil), -1

	if mode==RFMStarsOverHFR {
		for _, lightP:=range lights {
			if lightP==nil { continue }
			score:=float32(len(lightP.Stars))/lightP.HFR
			if len(lightP.Stars)==0 || lightP.HFR==0 { score=0 }
			if score>refScore {
				refFrame, refScore = lightP, score
			}
		}	
	} else if mode==RFMMedianLoc {
		locs:=make([]float32, len(lights))
		num:=0
		for _, lightP:=range lights {
			if lightP==nil { continue }
			locs[num]=lightP.Stats.Location
			num++
		}	
		medianLoc:=QSelectMedianFloat32(locs[:num])
		for _, lightP:=range lights {
			if lightP==nil { continue }
			if lightP.Stats.Location==medianLoc {
				return lightP, medianLoc
			}
		}	
	}
	return refFrame, refScore
}

