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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/debug"
	"strings"
	"time"
	nl "github.com/mlnoga/nightlight/internal"
	"github.com/mlnoga/nightlight/internal/rest"
	"github.com/pbnjay/memory"
	colorful "github.com/lucasb-eyer/go-colorful"
)

const version = "0.2.5"

type Job struct {
	Id       int
	FileName string
	Image    *nl.FITSImage 
	Err      error
}

var totalMiBs=memory.TotalMemory()/1024/1024

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

var out  = flag.String("out", "out.fits", "save output to `file`")
var jpg  = flag.String("jpg", "%auto",  "save 8bit preview of output as JPEG to `file`. `%auto` replaces suffix of output file with .jpg")
var log  = flag.String("log", "%auto",    "save log output to `file`. `%auto` replaces suffix of output file with .log")
var pre  = flag.String("pre",  "",  "save pre-processed frames with given filename pattern, e.g. `pre%04d.fits`")
var stars= flag.String("stars","","save star detections with given filename pattern, e.g. `stars%04d.fits`")
var back = flag.String("back","","save extracted background with given filename pattern, e.g. `back%04d.fits`")
var post = flag.String("post", "",  "save post-processed frames with given filename pattern, e.g. `post%04d.fits`")
var batch= flag.String("batch", "", "save stacked batches with given filename pattern, e.g. `batch%04d.fits`")

var dark = flag.String("dark", "", "apply dark frame from `file`")
var flat = flag.String("flat", "", "apply flat frame from `file`")

var debayer = flag.String("debayer", "", "debayer the given channel, one of R, G, B or blank for no op")
var cfa     = flag.String("cfa", "RGGB", "color filter array type for debayering, one of RGGB, GRBG, GBRG, BGGR")

var binning= flag.Int64("binning", 0, "apply NxN binning, 0 or 1=no binning")

var bpSigLow  = flag.Float64("bpSigLow", 3.0,"low sigma for bad pixel removal as multiple of standard deviations")
var bpSigHigh = flag.Float64("bpSigHigh",5.0,"high sigma for bad pixel removal as multiple of standard deviations")

var starSig   = flag.Float64("starSig", 15.0,"sigma for star detection as multiple of standard deviations")
var starBpSig = flag.Float64("starBpSig",-1.0,"sigma for star detection bad pixel removal as multiple of standard deviations, -1: auto")
var starInOut = flag.Float64("starInOut",1.4,"minimal ratio of brightness inside HFR to outside HFR for star detection")
var starRadius= flag.Int64("starRadius", 16.0, "radius for star detection in pixels")

var backGrid  = flag.Int64("backGrid", 0, "automated background extraction: grid size in pixels, 0=off")
var backSigma = flag.Float64("backSigma", 1.5 ,"automated background extraction: sigma for detecting foreground objects")
var backClip  = flag.Int64("backClip", 0, "automated background extraction: clip the k brightest grid cells and replace with local median")

var usmSigma  = flag.Float64("usmSigma", 1, "unsharp masking sigma, ~1/3 radius")
var usmGain   = flag.Float64("usmGain", 0, "unsharp masking gain, 0=no op")
var usmThresh = flag.Float64("usmThresh", 1, "unsharp masking threshold, in standard deviations above background")

var align     = flag.Int64("align",1,"1=align frames, 0=do not align")
var alignK    = flag.Int64("alignK",20,"use triangles fromed from K brightest stars for initial alignment")
var alignT    = flag.Float64("alignT",1.0,"skip frames if alignment to reference frame has residual greater than this")
var alignTo   = flag.String("alignTo", "", "use given `file` as alignment reference")

var lsEst     = flag.Int64("lsEst",3,"location and scale estimators 0=mean/stddev, 1=median/MAD, 2=IKSS, 3=iterative sigma-clipped sampled median and sampled Qn (standard), 4=histogram peak")
var normRange = flag.Int64("normRange",0,"normalize range: 1=normalize to [0,1], 0=do not normalize")
var normHist  = flag.Int64("normHist",4,"normalize histogram: 0=do not normalize, 1=location, 2=location and scale, 3=black point shift for RGB align, 4=auto")

var stMode    = flag.Int64("stMode", 5, "stacking mode. 0=median, 1=mean, 2=sigma clip, 3=winsorized sigma clip, 4=linear fit, 5=auto")
var stClipPercLow = flag.Float64("stClipPercLow", 0.5,"set desired low clipping percentage for stacking, 0=ignore (overrides sigmas)")
var stClipPercHigh= flag.Float64("stClipPercHigh",0.5,"set desired high clipping percentage for stacking, 0=ignore (overrides sigmas)")
var stSigLow  = flag.Float64("stSigLow", -1,"low sigma for stacking as multiple of standard deviations, -1: use clipping percentage to find")
var stSigHigh = flag.Float64("stSigHigh",-1,"high sigma for stacking as multiple of standard deviations, -1: use clipping percentage to find")
var stWeight  = flag.Int64("stWeight", 0, "weights for stacking. 0=unweighted (default), 1=by exposure, 2=by inverse noise")
var stMemory  = flag.Int64("stMemory", int64((totalMiBs*7)/10), "total MiB of memory to use for stacking, default=0.7x physical memory")

var refSelMode= flag.Int64("refSelMode", 0, "reference frame selection mode, 0=best #stars/HFR (default), 1=median HFR (for master flats)")

var neutSigmaLow  = flag.Float64("neutSigmaLow", -1, "neutralize background color below this threshold, <0 = no op")
var neutSigmaHigh = flag.Float64("neutSigmaHigh", -1, "keep background color above this threshold, interpolate in between, <0 = no op")

var chromaGamma=flag.Float64("chromaGamma", 1.0, "scale LCH chroma curve by given gamma for luminances n sigma above background, 1.0=no op")
var chromaSigma=flag.Float64("chromaSigma", 1.0, "only scale and add to LCH chroma for luminances n sigma above background")

var chromaFrom= flag.Float64("chromaFrom", 295, "scale LCH chroma for hues in [from,to] by given factor, e.g. 295 to desaturate violet stars")
var chromaTo  = flag.Float64("chromaTo", 40, "scale LCH chroma for hues in [from,to] by given factor, e.g. 40 to desaturate violet stars")
var chromaBy  = flag.Float64("chromaBy", 1, "scale LCH chroma for hues in [from,to] by given factor, e.g. -1 to desaturate violet stars")

var rotFrom   = flag.Float64("rotFrom", 100, "rotate LCH color angles in [from,to] by given offset, e.g. 100 to aid Hubble palette for S2HaO3")
var rotTo     = flag.Float64("rotTo", 190, "rotate LCH color angles in [from,to] by given offset, e.g. 190 to aid Hubble palette for S2HaO3")
var rotBy     = flag.Float64("rotBy", 0, "rotate LCH color angles in [from,to] by given offset, e.g. -30 to aid Hubble palette for S2HaO3")
var rotSigma  = flag.Float64("rotSigma", 1, "rotate LCH color angles in [from, to] vor luminances >= location + scale*sigma")

var scnr      = flag.Float64("scnr",0,"apply SCNR in [0,1] to green channel, e.g. 0.5 for tricolor with S2HaO3 and 0.1 for bicolor HaO3O3")

var autoLoc   = flag.Float64("autoLoc", 10, "histogram peak location in %% to target with automatic curves adjustment, 0=don't")
var autoScale = flag.Float64("autoScale", 0.4, "histogram peak scale in %% to target with automatic curves adjustment, 0=don't")

var midtone   = flag.Float64("midtone", 0, "midtone value in multiples of standard deviation; 0=no op")
var midBlack  = flag.Float64("midBlack", 2, "midtone black in multiples of standard deviation below background location")

var gamma     = flag.Float64("gamma", 1, "apply output gamma, 1: keep linear light data")
var ppGamma   = flag.Float64("ppGamma", 1, "apply post-peak gamma, scales curve from location+scale...ppLimit, 1: keep linear light data")
var ppSigma   = flag.Float64("ppSigma", 1, "apply post-peak gamma this amount of scales from the peak (to avoid scaling background noise)")

var scaleBlack= flag.Float64("scaleBlack", 0, "move black point so histogram peak location is given value in %%, 0=don't")

var lights   =[]*nl.FITSImage{}

func main() {
	logWriter:=os.Stdout
	debug.SetGCPercent(10)
	start:=time.Now()
	flag.Usage=func(){
 	    fmt.Fprintf(logWriter, `Nightlight Copyright (c) 2020 Markus L. Noga
This program comes with ABSOLUTELY NO WARRANTY.
This is free software, and you are welcome to redistribute it under certain conditions.
Refer to https://www.gnu.org/licenses/gpl-3.0.en.html for details.

Usage: %s [-flag value] (stats|stack|rgb|argb|lrgb|legal) (img0.fits ... imgn.fits)

Commands:
  stats   Show input image statistics
  stack   Stack input images
  stretch Stretch single image
  rgb     Combine color channels. Inputs are treated as r, g and b channel in that order
  lrgb    Combine color channels and combine with luminance. Inputs are treated as l, r, g and b channels
  legal   Show license and attribution information
  version Show version information

Flags:
`, os.Args[0])
	    flag.PrintDefaults()
	}
	flag.Parse()

	// Initialize logging to file in addition to stdout, if selected
	if *log=="%auto" {
		if *out!="" {
			*log=strings.TrimSuffix(*out, filepath.Ext(*out))+".log"			
		} else {
			*log=""
		}
	}
	if *log!="" { 
		err:=nl.LogAlsoToFile(*log)
		if err!=nil { nl.LogFatalf("Unable to open logfile '%s'\n", *log) }
	}

	// Also auto-select JPEG output target
	if *jpg=="%auto" {
		if *out!="" {
			*jpg=strings.TrimSuffix(*out, filepath.Ext(*out))+".jpg"			
		} else {
			*jpg=""
		}
	}

	// Enable CPU profiling if flagged
    if *cpuprofile != "" {
        f, err := os.Create(*cpuprofile)
        if err != nil {
            nl.LogFatal("Could not create CPU profile: ", err)
        }
        defer f.Close()
        if err := pprof.StartCPUProfile(f); err != nil {
            nl.LogFatal("Could not start CPU profile: ", err)
        }
        defer pprof.StopCPUProfile()
    }

    args:=flag.Args()
    if len(args)<1 {
    	flag.Usage()
    	return
    }
    if args[0]=="stats" || args[0]=="stack" || args[0]=="stretch" || args[0]=="rgb" || args[0]=="lrgb" {
	    fmt.Fprintf(logWriter, "Using location and scale estimator %d\n", *lsEst)
		nl.LSEstimator=nl.LSEstimatorMode(*lsEst)
	}

	// set defaults per command
    switch args[0] {
    case "serve":
    case "stats":
		if *normHist==nl.HNMAuto { *normHist=nl.HNMNone }
		if *starBpSig<0 { *starBpSig=0 } // default to no noise elimination
    case "stack":
		if *normHist==nl.HNMAuto { *normHist=nl.HNMLocScale }
		if *starBpSig<0 { *starBpSig=5 } // default to noise elimination when working with individual subexposures
    case "stretch":
    case "rgb":
    case "lrgb":
    case "legal":
    case "version":
    case "help", "?":
    default:
    }

	// glob filename arguments into OpLoadFiles operators
	var err error
	opLoadFiles, err:=nl.NewOpLoadFiles(args, logWriter)
	if err!=nil {
		fmt.Fprintf(logWriter, "Error globbing filenames: %s\n", err.Error())
		os.Exit(-1)
	}

	// parse flags into OperatorUnary objects
	opPreProc:=nl.NewOpPreProcess(*dark, *flat, *debayer, *cfa, int32(*binning), float32(*bpSigLow), float32(*bpSigHigh), 
		float32(*starSig), float32(*starBpSig), float32(*starInOut), int32(*starRadius), *stars, 
		int32(*backGrid), float32(*backSigma), int32(*backClip), *back, *pre,
	)
	if err:=opPreProc.Init(); err!=nil {
		fmt.Fprintf(logWriter, "Error initializing preprocessing: %s\n", err.Error())
		os.Exit(-1)
	}

	opPostProc:=nl.NewOpPostProcess(nl.HistoNormMode(*normHist), int32(*align), int32(*alignK), float32(*alignT), 
									nl.OOBModeNaN, nl.RefSelMode(*refSelMode), *post)
	if err:=opPostProc.Init(); err!=nil {
		fmt.Fprintf(logWriter, "Error initializing postprocessing: %s\n", err.Error())
		os.Exit(-1)
	}

	// run actions
    switch args[0] {
    case "serve":
    	rest.Serve();

    case "stats":
		opParallel:=nl.NewOpParallel(opPreProc, int64(runtime.NumCPU()))
		_, err=opParallel.ApplyToFiles(opLoadFiles, logWriter)

	case "stack":
		opStack:=nl.NewOpStack(nl.StackMode(*stMode), nl.StackWeighting(*stWeight), float32(*stSigLow), float32(*stSigHigh))
    	opSingleBatch:=nl.NewOpSingleBatch(opPreProc, opPostProc, opStack, opPreProc.StarDetect, *batch)
    	opMultiBatch :=nl.NewOpMultiBatch(opSingleBatch, *stMemory, nl.NewOpSave(*out))
		if err:=opMultiBatch.Init(); err!=nil {
			fmt.Fprintf(logWriter, "Error initializing stacking: %s\n", err.Error())
			os.Exit(-1)
		}
    	_, err=opMultiBatch.Apply(opLoadFiles, logWriter)

    case "stretch":
    	opStretch:=nl.NewOpStretch(
			nl.NewOpNormalizeRange  (true),
			nl.NewOpStretchIterative(float32(*autoLoc / 100), float32(*autoScale / 100)),
			nl.NewOpMidtones        (float32(*midtone), float32(*midBlack)),
			nl.NewOpGamma           (float32(*gamma)),
			nl.NewOpPPGamma         (float32(*ppGamma), float32(*ppSigma)),
			nl.NewOpScaleBlack      (float32(*scaleBlack / 100)),
			nl.NewOpStarDetect      (int32(*starRadius), float32(*starSig), float32(*starBpSig), float32(*starInOut), *stars),
			nl.NewOpAlign           (int32(*align), int32(*alignK), float32(*alignT), nl.OOBModeOwnLocation, nl.RefSelMode(*refSelMode)),
			nl.NewOpUnsharpMask     (float32(*usmSigma), float32(*usmGain), float32(*usmThresh)),
			nl.NewOpSave            (*out),
			nl.NewOpSave            (*jpg),
    	)

    	var m []byte
		m,err=json.MarshalIndent(opStretch,"", "  ")
		if err!=nil { break }
		fmt.Fprintf(logWriter, "\nStretching %d frames with these settings:\n%s\n", len(opLoadFiles), string(m))

    	opParallel:=nl.NewOpParallel(opStretch, int64(runtime.GOMAXPROCS(0)))
    	_, err=opParallel.ApplyToFiles(opLoadFiles, logWriter)  // FIXME: materializes all files in memory

    case "rgb":
    	cmdRGB(args[1:], logWriter)

    case "lrgb":
    	cmdLRGB(args[1:], logWriter)

    case "legal":
    	cmdLegal()

    case "version":
    	fmt.Fprintf(logWriter, "Version %s\n", version)

    case "help", "?":
    	flag.Usage()

    default:
    	fmt.Fprintf(logWriter, "Unknown command '%s'\n\n", args[0])
    	flag.Usage()
    	return 
    }

	now:=time.Now()
	elapsed:=now.Sub(start)
	fmt.Fprintf(logWriter, "\nDone after %v\n", elapsed)

	// Store memory profile if flagged
    if *memprofile != "" {
        f, err := os.Create(*memprofile)
        if err != nil {
            nl.LogFatal("Could not create memory profile: ", err)
        }
        defer f.Close()
        runtime.GC() // get up-to-date statistics
        if err := pprof.Lookup("allocs").WriteTo(f,0); err != nil {
            nl.LogFatal("Could not write allocation profile: ", err)
        }
    }

    if err!=nil {
		fmt.Fprintf(logWriter, "Error: %s\n", err.Error())
		os.Exit(-1)
	}
    nl.LogSync()
}




// Perform RGB combination command
func cmdRGB(args []string, logWriter io.Writer) {
	// Set default parameters for this command
	if *normHist==nl.HNMAuto { *normHist=nl.HNMNone }
	if *starBpSig<0 { *starBpSig=0 }  // inputs are typically stacked and have undergone noise removal

	// Glob file name wildcards
	opLoadFiles, err:=nl.NewOpLoadFiles(args, logWriter)
	if err!=nil {
		fmt.Fprintf(logWriter, "error globbing wildcards: %s\n", err.Error())
		return // FIXME
	}
	if len(opLoadFiles)!=3 {
		fmt.Fprintf(logWriter, "Need exactly three input files to perform a RGB combination")
		return // FIXME
	}

	// Read files and detect stars
	imageLevelParallelism:=int64(runtime.GOMAXPROCS(0))
	if imageLevelParallelism>3 { imageLevelParallelism=3 }
	fmt.Fprintf(logWriter, "\nReading color channels and detecting stars:\n")

	// Parse command line parameters into settings object
	opPreProc:=nl.NewOpPreProcess("", "", *debayer, *cfa, 1, 0, 0, 
		float32(*starSig), float32(*starBpSig), float32(*starInOut), int32(*starRadius), *stars, 
		int32(*backGrid), float32(*backSigma), int32(*backClip), *back, *pre,
	)
	if err:=opPreProc.Init(); err!=nil {
		panic(err)
	}

	opParallelPre:=nl.NewOpParallel(opPreProc, imageLevelParallelism)
	lights, err=opParallelPre.ApplyToFiles(opLoadFiles, logWriter)
	if err!=nil { panic(err.Error()) }

	// Pick reference frame
	var refFrame *nl.FITSImage
	var refFrameScore float32

	//if (*align)!=0 || (*normHist)!=0 {
		refFrame, refFrameScore=nl.SelectReferenceFrame(lights, nl.RefSelMode(*refSelMode))
		if refFrame==nil { panic("Reference channel for alignment not found.") }
		fmt.Fprintf(logWriter, "Using channel %d with score %.4g as reference for alignment and normalization.\n\n", refFrame.ID, refFrameScore)
	//}

/*
	// Post-process all channels (align, normalize)
	var oobMode nl.OutOfBoundsMode=nl.OOBModeOwnLocation
	fmt.Fprintf(logWriter, "Postprocessing %d channels with align=%d alignK=%d alignT=%.3f normHist=%d oobMode=%d usmSigma=%g usmGain=%g usmThresh=%g:\n", 
				 len(lights), *align, *alignK, *alignT, *normHist, oobMode, float32(*usmSigma), float32(*usmGain), float32(*usmThresh))
	numErrors:=nl.PostProcessLights(refFrame, refFrame, lights, int32(*align), int32(*alignK), float32(*alignT), nl.HistoNormMode(*normHist), oobMode, 
									float32(*usmSigma), float32(*usmGain), float32(*usmThresh), *post, imageLevelParallelism)
    if numErrors>0 { nl.LogFatal("Need aligned RGB frames to proceed") }
*/

	// Combine RGB channels
	fmt.Fprintf(logWriter, "\nCombining color channels...\n")
	rgb:=nl.CombineRGB(lights, refFrame)

	postProcessAndSaveRGBComposite(&rgb, nil, logWriter)
	rgb.Data=nil
}


// Perform LRGB combination command
func cmdLRGB(args []string, logWriter io.Writer) {
	// Set default parameters for this command
	if *normHist==nl.HNMAuto { *normHist=nl.HNMNone }
	if *starBpSig<0 { *starBpSig=0 }    // inputs are typically stacked and have undergone noise removal

	// Glob file name wildcards
	opLoadFiles,err:=nl.NewOpLoadFiles(args, logWriter)
	if len(opLoadFiles)!=4 {
		nl.LogFatal("Need exactly four input files to perform a LRGB combination")
	}

	// Read files and detect stars
	imageLevelParallelism:=int64(runtime.GOMAXPROCS(0))
	if imageLevelParallelism>3 { imageLevelParallelism=3 }
	fmt.Fprintf(logWriter, "\nReading color channels and detecting stars:\n")

	// Parse command line parameters into settings object
	opPreProc:=nl.NewOpPreProcess("", "", *debayer, *cfa, 1, 0, 0, 
		float32(*starSig), float32(*starBpSig), float32(*starInOut), int32(*starRadius), *stars, 
		int32(*backGrid), float32(*backSigma), int32(*backClip), *back, *pre,
	)
	if err:=opPreProc.Init(); err!=nil {
		panic(err)
	}

	opParallelPre:=nl.NewOpParallel(opPreProc, imageLevelParallelism)
	lights, err=opParallelPre.ApplyToFiles(opLoadFiles, logWriter)
	if err!=nil { panic(err.Error()) }

	var refFrame, histoRef *nl.FITSImage
	if (*align)!=0 {
		// Always use luminance as reference frame
		refFrame=lights[0]
		fmt.Fprintf(logWriter, "Using luminance channel %d as reference for alignment.\n", refFrame.ID)

		// Recalculate star detections for RGB frames with corrected radius if the images were previously binned
		for _, light:=range(lights[1:]) {
			correction:=refFrame.Naxisn[0] / light.Naxisn[0]
			if correction!=1 {
				light.Stars, _, light.HFR=nl.FindStars(light.Data, light.Naxisn[0], light.Stats.Location, light.Stats.Scale, float32(*starSig), float32(*starBpSig), float32(*starInOut), int32(*starRadius)/correction, nil)
				fmt.Fprintf(logWriter, "%d: Stars %d HFR %.3g %v\n", light.ID, len(light.Stars), light.HFR, light.Stats)
			}
		}
	}

	if (*normHist)!=0 {
		// Normalize to [0,1]
		histoRef=lights[1]
		minLoc:=float32(histoRef.Stats.Location)
	    for id, light:=range(lights) {
	    	if id>0 && light.Stats.Location<minLoc { 
	    		minLoc=light.Stats.Location 
	    		histoRef=light
	    	}
	    }
		fmt.Fprintf(logWriter, "Using color channel %d as reference for RGB peak normalization to %.4g...\n\n", histoRef.ID, histoRef.Stats.Location)
	}

	/*
	// Align images if selected
	var oobMode nl.OutOfBoundsMode=nl.OOBModeOwnLocation
	fmt.Fprintf(logWriter, "Postprocessing %d channels with align=%d alignK=%d alignT=%.3f normHist=%d oobMode=%d usmSigma=%g usmGain=%g usmThresh=%g:\n", 
		         len(lights), *align, *alignK, *alignT, *normHist, oobMode, *usmSigma, *usmGain, *usmThresh)
	numErrors:=nl.PostProcessLights(refFrame, histoRef, lights, int32(*align), int32(*alignK), float32(*alignT), nl.HistoNormMode(*normHist), oobMode, 
									float32(*usmSigma), float32(*usmGain), float32(*usmThresh), "", imageLevelParallelism)
    if numErrors>0 { nl.LogFatal("Need aligned RGB frames to proceed") }
    */

	// Combine RGB channels
	fmt.Fprintf(logWriter, "\nCombining color channels...\n")
	rgb:=nl.CombineRGB(lights[1:], lights[0])

	postProcessAndSaveRGBComposite(&rgb, lights[0], logWriter)
	rgb.Data=nil
}

func postProcessAndSaveRGBComposite(rgb *nl.FITSImage, lum *nl.FITSImage, logWriter io.Writer) {

	// Auto-balance colors in linear RGB color space
	autoBalanceColors(rgb)

	nl.LogPrintln("Converting color image to HSLuv color space")
	rgb.RGBToHSLuv()
	//nl.LogPrintln("Converting color image to modified CIE HCL space (i.e. HSL)")
	//rgb.RGBToCIEHSL()

	// Apply LRGB combination in linear CIE xyY color space
	if lum!=nil {
	    nl.LogPrintln("Converting luminance image to HSLuv as well...")
	    lum.MonoToHSLuvLum()
	    //nl.LogPrintln("Converting luminance image to HSL as well...")
	    //lum.MonoToHSLLum()

		nl.LogPrintln("Applying luminance image to luminance channel...")
		rgb.ApplyLuminanceToCIExyY(lum)
	}

    if (*neutSigmaLow>=0) && (*neutSigmaHigh>=0) {
		fmt.Fprintf(logWriter, "Neutralizing background values below %.4g sigma, keeping color above %.4g sigma\n", *neutSigmaLow, *neutSigmaHigh)    	

		_, _, loc, scale, err:=nl.HCLLumMinMaxLocScale(rgb.Data, rgb.Naxisn[0])
		if err!=nil { nl.LogFatal(err) }
		low :=loc + scale*float32(*neutSigmaLow)
		high:=loc + scale*float32(*neutSigmaHigh)
		fmt.Fprintf(logWriter, "Location %.2f%%, scale %.2f%%, low %.2f%% high %.2f%%\n", loc*100, scale*100, low*100, high*100)

		rgb.NeutralizeBackground(low, high)		
    }

    if (*chromaGamma)!=1 {
    	fmt.Fprintf(logWriter, "Applying gamma %.2f to saturation for values %.4g sigma above background...\n", *chromaGamma, *chromaSigma)

		// calculate basic image stats as a fast location and scale estimate
		_, _, loc, scale, err:=nl.HCLLumMinMaxLocScale(rgb.Data, rgb.Naxisn[0])
		if err!=nil { nl.LogFatal(err) }
		threshold :=loc + scale*float32(*chromaSigma)
		fmt.Fprintf(logWriter, "Location %.2f%%, scale %.2f%%, threshold %.2f%%\n", loc*100, scale*100, threshold*100)

		rgb.AdjustChroma(float32(*chromaGamma), threshold)
    }

    if (*chromaBy)!=1 {
    	fmt.Fprintf(logWriter, "Multiplying LCH chroma (saturation) by %.4g for hues in [%g,%g]...\n", *chromaBy, *chromaFrom, *chromaTo)
		rgb.AdjustChromaForHues(float32(*chromaFrom), float32(*chromaTo), float32(*chromaBy))
    }

    if (*rotBy)!=0 {
    	fmt.Fprintf(logWriter, "Rotating LCH hue angles in [%g,%g] by %.4g for lum>=loc+%g*scale...\n", *rotFrom, *rotTo, *rotBy, *rotSigma)
		_, _, loc, scale, err:=nl.HCLLumMinMaxLocScale(rgb.Data, rgb.Naxisn[0])
		if err!=nil { nl.LogFatal(err) }
		rgb.RotateColors(float32(*rotFrom), float32(*rotTo), float32(*rotBy), loc + float32(*rotSigma) * scale)
    }

    if (*scnr)!=0 {
    	fmt.Fprintf(logWriter, "Applying SCNR of %.4g ...\n", *scnr)
		rgb.SCNR(float32(*scnr))
    }

	// apply unsharp masking, if requested
	if *usmGain>0 {
		min, max, loc, scale, err:=nl.HCLLumMinMaxLocScale(rgb.Data, rgb.Naxisn[0])
		if err!=nil { nl.LogFatal(err) }
		absThresh:=loc + scale*float32(*usmThresh)
		fmt.Fprintf(logWriter, "%d: Unsharp masking with sigma %.3g gain %.3g thresh %.3g absThresh %.3g\n", rgb.ID, float32(*usmSigma), float32(*usmGain), float32(*usmThresh), absThresh)
		kernel:=nl.GaussianKernel1D(float32(*usmSigma))
		fmt.Fprintf(logWriter, "Unsharp masking kernel sigma %.2f size %d: %v\n", *usmSigma, len(kernel), kernel)
		newLum:=nl.UnsharpMask(rgb.Data[2*len(rgb.Data)/3:], int(rgb.Naxisn[0]), float32(*usmSigma), float32(*usmGain), min, max, absThresh)
		copy(rgb.Data[2*len(rgb.Data)/3:], newLum)
	}

	// Optionally adjust midtones
	if (*midtone)!=0 {
		fmt.Fprintf(logWriter, "Applying midtone correction with midtone=%.2f%% x scale and black=location - %.2f%% x scale\n", *midtone, *midBlack)
		// calculate basic image stats as a fast location and scale estimate
		_, _, loc, scale, err:=nl.HCLLumMinMaxLocScale(rgb.Data, rgb.Naxisn[0])
		if err!=nil { nl.LogFatal(err) }
		absMid:=float32(*midtone)*scale
		absBlack:=loc - float32(*midBlack)*scale
		fmt.Fprintf(logWriter, "loc %.2f%% scale %.2f%% absMid %.2f%% absBlack %.2f%%\n", 100*loc, 100*scale, 100*absMid, 100*absBlack)
		rgb.ApplyMidtonesToChannel(2, absMid, absBlack)
	}

	// Optionally adjust gamma
	if (*gamma)!=1 {
		fmt.Fprintf(logWriter, "Applying gamma %.3g\n", *gamma)
		rgb.ApplyGammaToChannel(2, float32(*gamma))
	}

	// Optionally adjust gamma post peak
	if (*ppGamma)!=1 {
	         _, _, loc, scale, err:=nl.HCLLumMinMaxLocScale(rgb.Data, rgb.Naxisn[0])
	         if err!=nil { nl.LogFatal(err) }
	 from:=loc+float32(*ppSigma)*scale
	 to  :=float32(1.0)
	 fmt.Fprintf(logWriter, "Based on sigma=%.4g, boosting values in [%.2f%%, %.2f%%] with gamma %.4g...\n", *ppSigma, from*100, to*100, *ppGamma)
	         rgb.ApplyPartialGammaToChannel(2, from, to, float32(*ppGamma))
	}

	// Optionally scale histogram peak
	if (*scaleBlack)!=0 {
	 xyyTargetBlack:=float32((*scaleBlack)/100.0)
		_,_,hclTargetBlack:=colorful.Xyy(0,0,float64(xyyTargetBlack)).Hcl()
		targetBlack:=float32(hclTargetBlack)
		_, _, loc, scale, err:=nl.HCLLumMinMaxLocScale(rgb.Data, rgb.Naxisn[0])
		if err!=nil { nl.LogFatal(err) }
		fmt.Fprintf(logWriter, "Location %.2f%% and scale %.2f%%: ", loc*100, scale*100)
		if loc>targetBlack {
			fmt.Fprintf(logWriter, "scaling black to move location to HCL %.2f%% for linear %.2f%%...\n", targetBlack*100.0, xyyTargetBlack*100.0)
			rgb.ShiftBlackToMoveChannel(2,loc, targetBlack)
		} else {
			fmt.Fprintf(logWriter, "cannot move to location %.2f%% by scaling black\n", targetBlack*100.0)
		}
	}

	nl.LogPrintln("Converting nonlinear HSLuv to linear RGB")
    rgb.HSLuvToRGB()
	//nl.LogPrintln("Converting modified CIE HCL (i.e. HSL) to linear RGB")
 	//rgb.CIEHSLToRGB()

	// Write outputs
	fmt.Fprintf(logWriter, "Writing FITS to %s ...\n", *out)
	err:=rgb.WriteFile(*out)
	if err!=nil { nl.LogFatalf("Error writing file: %s\n", err) }
	if (*jpg)!="" {
		fmt.Fprintf(logWriter, "Writing JPG to %s ...\n", *jpg)
		rgb.WriteJPGToFile(*jpg, 95)
		if err!=nil { nl.LogFatalf("Error writing file: %s\n", err) }
	}
}


// Automatically balance colors with multiple iterations of SetBlackWhitePoints, producing log output
func autoBalanceColors(rgb *nl.FITSImage) {
	if len(rgb.Stars)==0 {
		nl.LogPrintln("Skipping black and white point adjustment as zero stars have been detected")
	} else {
		nl.LogPrintln("Setting black point so histogram peaks align and white point so median star color becomes neutral...")
		err:=rgb.SetBlackWhitePoints()
		if err!=nil { nl.LogFatal(err) }
	}
}


// Helper: convert bool to int
func btoi(b bool) int {
	if b { return 1 }
	return 0
}

// Show licensing information
func cmdLegal() {
	nl.LogPrint(`Nightlight is Copyright (c) 2020 Markus L. Noga
This program comes with ABSOLUTELY NO WARRANTY.
This is free software, and you are welcome to redistribute it under certain conditions.
Refer to https://www.gnu.org/licenses/gpl-3.0.en.html for details.
The binary version of this program uses several open source libraries and components, which come with their own licensing terms. See below for a list of attributions.

ATTRIBUTIONS

A1. https://github.com/gonum/gonum is Copyright (c) 2013 The Gonum Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

* Neither the name of the copyright holder nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.


A2. https://github.com/pbnjay/memory is Copyright (c) 2017, Jeremy Jay. All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

* Neither the name of the copyright holder nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.


A3. https://github.com/valyala/fastrand is Copyright (c) 2017 Aliaksandr Valialkin

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.


A4. https://github.com/lucasb-eyer/go-colorful is Copyright (c) 2013 Lucas Beyer

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.


A5. https://github.com/klauspost/cpuid is Copyright (c) 2015 Klaus Post

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
`)
}