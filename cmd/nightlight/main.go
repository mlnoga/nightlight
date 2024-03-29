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
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/ops"
	"github.com/mlnoga/nightlight/internal/ops/hsl"
	"github.com/mlnoga/nightlight/internal/ops/post"
	"github.com/mlnoga/nightlight/internal/ops/pre"
	"github.com/mlnoga/nightlight/internal/ops/ref"
	"github.com/mlnoga/nightlight/internal/ops/rgb"
	"github.com/mlnoga/nightlight/internal/ops/stack"
	"github.com/mlnoga/nightlight/internal/ops/stretch"
	"github.com/mlnoga/nightlight/internal/rest"
	"github.com/mlnoga/nightlight/internal/stats"
	"github.com/pbnjay/memory"
)

const version = "0.2.5"

var totalMiBs = memory.TotalMemory() / 1024 / 1024

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

var port = flag.Int64("port", 8080, "port for serving HTTP API")
var chroot = flag.String("chroot", "", "directory to chroot and chdir to when serving HTTP. must be run as root")
var setuid = flag.Int64("setuid", -1, "user id number to setuid to when serving HTTP. must be run as root")
var job = flag.String("job", "", "JSON job specification to run")

var out = flag.String("out", "out.fits", "save output to `file`")
var jpg = flag.String("jpg", "%auto", "save 8bit preview of output as JPEG to `file`. `%auto` replaces suffix of output file with .jpg")
var jpgGamma = flag.Float64("jpgGamma", 1.0, "gamma correction for JPG output, 1.0=off")
var tiff = flag.String("tiff", "", "save 16bit preview of output as TIFF to `file`. `%auto` replaces suffix of output file with .tif")
var log = flag.String("log", "%auto", "save log output to `file`. `%auto` replaces suffix of output file with .log")
var pPre = flag.String("pre", "", "save pre-processed frames with given filename pattern, e.g. `pre%04d.fits`")
var stars = flag.String("stars", "", "save star detections with given filename pattern, e.g. `stars%04d.fits`")
var back = flag.String("back", "", "save extracted background with given filename pattern, e.g. `back%04d.fits`")
var pPost = flag.String("post", "", "save post-processed frames with given filename pattern, e.g. `post%04d.fits`")
var batch = flag.String("batch", "", "save stacked batches with given filename pattern, e.g. `batch%04d.fits`")

var dark = flag.String("dark", "", "apply dark frame from `file`")
var flat = flag.String("flat", "", "apply flat frame from `file`")

var debayer = flag.String("debayer", "", "debayer the given channel, one of R, G, B or blank for no op")
var cfa = flag.String("cfa", "RGGB", "color filter array type for debayering, one of RGGB, GRBG, GBRG, BGGR")

var debandH = flag.Float64("debandH", 0.0, "deband horizontally with given percentile [0..100], 0=off")
var debandV = flag.Float64("debandV", 0.0, "deband vertically with given percentile [0..100], 0=off")
var debandHWindow = flag.Int64("debandHWindow", 128, "deband horizontally with given window size [1..imageHeight], 0=off")
var debandVWindow = flag.Int64("debandVWindow", 128, "deband vertically with given window size [1..imageWidth], 0=off")
var debandHSigma = flag.Float64("debandHSigma", 3.0, "deband horizontally excluding values above location + sigma*scale, 0=off")
var debandVSigma = flag.Float64("debandVSigma", 3.0, "deband vertically excluding values above location + sigma*scale, 0=off")

var binning = flag.Int64("binning", 0, "apply NxN binning, 0 or 1=no binning")

var bpSigLow = flag.Float64("bpSigLow", 3.0, "low sigma for bad pixel removal as multiple of standard deviations")
var bpSigHigh = flag.Float64("bpSigHigh", 5.0, "high sigma for bad pixel removal as multiple of standard deviations")

var starSig = flag.Float64("starSig", 15.0, "sigma for star detection as multiple of standard deviations")
var starBpSig = flag.Float64("starBpSig", -1.0, "sigma for star detection bad pixel removal as multiple of standard deviations, -1: auto")
var starInOut = flag.Float64("starInOut", 1.4, "minimal ratio of brightness inside HFR to outside HFR for star detection")
var starRadius = flag.Int64("starRadius", 16.0, "radius for star detection in pixels")

var backGrid = flag.Int64("backGrid", 0, "automated background extraction: grid size in pixels, 0=off")
var backHFRFactor = flag.Float64("backHFRFactor", 4.0, "automated background extraction: exclude stars with HFR multiplied by this factor")
var backSigma = flag.Float64("backSigma", 1.5, "automated background extraction: sigma for detecting foreground objects")
var backClip = flag.Int64("backClip", 0, "automated background extraction: clip the k brightest grid cells and replace with local median")

var minStars = flag.Int64("minStars", 0, "minimum number of stars for an image to be included in stacking, 0=don't filter")

var blurSigma = flag.Float64("blurSigma", 0, "gaussian blurring sigma, ~1/3 radius, 0=no op")

var usmSigma = flag.Float64("usmSigma", 1, "unsharp masking sigma, ~1/3 radius, 0=no op")
var usmGain = flag.Float64("usmGain", 0, "unsharp masking gain, 0=no op")
var usmThresh = flag.Float64("usmThresh", 1, "unsharp masking threshold, in standard deviations above background")

var alignK = flag.Int64("alignK", 20, "use triangles formed from K brightest stars for initial alignment")
var alignT = flag.Float64("alignT", 1.0, "skip frames if alignment to reference frame has residual greater than this")

var lsEst = flag.Int64("lsEst", 3, "location and scale estimators 0=mean/stddev, 1=median/MAD, 2=IKSS, 3=iterative sigma-clipped sampled median and sampled Qn (standard), 4=histogram peak")
var normRange = flag.Int64("normRange", 0, "normalize range: 1=normalize to [0,1], 0=do not normalize")
var normHist = flag.Int64("normHist", 4, "normalize histogram: 0=do not normalize, 1=location, 2=location and scale, 3=black point shift for RGB align, 4=auto")

var stMode = flag.Int64("stMode", 6, "stacking mode. 0=median, 1=mean, 2=sigma clip, 3=winsorized sigma clip, 4=MAD sigma clip, 5=linear fit, 6=auto")
var stSigLow = flag.Float64("stSigLow", -1, "low sigma for stacking as multiple of standard deviations, -1: use clipping percentage to find")
var stSigHigh = flag.Float64("stSigHigh", -1, "high sigma for stacking as multiple of standard deviations, -1: use clipping percentage to find")
var stWeight = flag.Int64("stWeight", 0, "weights for stacking. 0=unweighted (default), 1=by exposure, 2=by inverse noise")
var stMemory = flag.Int64("stMemory", int64((totalMiBs*7)/10), "total MiB of memory to use for stacking, default=0.7x physical memory")

var histoRef = flag.String("histoRef", "%starsHFR", "histogram reference, %starsHFR= best #stars/HFR (default), %location=median location, any int=image ID, filename=image filename")
var alignRef = flag.String("alignRef", "%starsHFR", "alignment reference, %starsHFR= best #stars/HFR (default), %location=median location, any int=image ID, filename=image filename")

var neutSigmaLow = flag.Float64("neutSigmaLow", -1, "neutralize background color below this threshold, <0 = no op")
var neutSigmaHigh = flag.Float64("neutSigmaHigh", -1, "keep background color above this threshold, interpolate in between, <0 = no op")

var balBlock = flag.Int64("balBlock", 16, "balance darkest NxN block to black")
var balBorder = flag.Float64("balBorder", 0.1, "balance darkest NxN block to black, excluding this fraction of screen size as a border")
var balSkipBright = flag.Float64("balSkipBright", 0, "balance star colors skipping brightest fraction of stars")
var balSkipDim = flag.Float64("balSkipDim", 0.5, "balance star colors skipping dimmest fraction of stars")
var balShR = flag.Float64("balShR", 1, "balance colors by tinting shadows with this red color, range 0.0-1.0")
var balShG = flag.Float64("balShG", 1, "balance colors by tinting shadows with this green color, range 0.0-1.0")
var balShB = flag.Float64("balShB", 1, "balance colors by tinting shadows with this blue color, range 0.0-1.0")
var balHiR = flag.Float64("balHiR", 1, "balance colors by tinting highlights with this red color, range 0.0-1.0")
var balHiG = flag.Float64("balHiG", 1, "balance colors by tinting highlights with this green color, range 0.0-1.0")
var balHiB = flag.Float64("balHiB", 1, "balance colors by tinting highlights with this blue color, range 0.0-1.0")

var chromaGamma = flag.Float64("chromaGamma", 1.0, "scale LCH chroma curve by given gamma for luminances n sigma above background, 1.0=no op")
var chromaSigma = flag.Float64("chromaSigma", 1.0, "only scale and add to LCH chroma for luminances n sigma above background")

var chromaFrom = flag.Float64("chromaFrom", 295, "scale LCH chroma for hues in [from,to] by given factor, e.g. 295 to desaturate violet stars")
var chromaTo = flag.Float64("chromaTo", 40, "scale LCH chroma for hues in [from,to] by given factor, e.g. 40 to desaturate violet stars")
var chromaBy = flag.Float64("chromaBy", 1, "scale LCH chroma for hues in [from,to] by given factor, e.g. -1 to desaturate violet stars")

var rotFrom = flag.Float64("rotFrom", 100, "rotate LCH color angles in [from,to] by given offset, e.g. 100 to aid Hubble palette for S2HaO3")
var rotTo = flag.Float64("rotTo", 190, "rotate LCH color angles in [from,to] by given offset, e.g. 190 to aid Hubble palette for S2HaO3")
var rotBy = flag.Float64("rotBy", 0, "rotate LCH color angles in [from,to] by given offset, e.g. -30 to aid Hubble palette for S2HaO3")
var rotSigma = flag.Float64("rotSigma", 1, "rotate LCH color angles in [from, to] vor luminances >= location + scale*sigma")

var scnr = flag.Float64("scnr", 0, "apply SCNR in [0,1] to green channel, e.g. 0.5 for tricolor with S2HaO3 and 0.1 for bicolor HaO3O3")

var autoLoc = flag.Float64("autoLoc", 10, "histogram peak location in %% to target with automatic curves adjustment, 0=don't")
var autoScale = flag.Float64("autoScale", 0.4, "histogram peak scale in %% to target with automatic curves adjustment, 0=don't")

var midtone = flag.Float64("midtone", 0, "midtone value in multiples of standard deviation; 0=no op")
var midBlack = flag.Float64("midBlack", 2, "midtone black in multiples of standard deviation below background location")

var gamma = flag.Float64("gamma", 1, "apply output gamma, 1: keep linear light data")
var ppGamma = flag.Float64("ppGamma", 1, "apply post-peak gamma, scales curve from location+scale...ppLimit, 1: keep linear light data")
var ppSigma = flag.Float64("ppSigma", 1, "apply post-peak gamma this amount of scales from the peak (to avoid scaling background noise)")

var preScale = flag.Float64("preScale", 1, "scale pixels by this factor")
var preOffset = flag.Float64("preOffset", 0, "offset pixels with this factor")

var lumScale = flag.Float64("lumScale", 1, "scale luminance by this factor")
var lumOffset = flag.Float64("lumOffset", 0, "offset luminance with this factor")

var scaleBlack = flag.Float64("scaleBlack", 0, "move black point so histogram peak location is given value in %%, 0=don't")

var exportStats = flag.String("exportStats", "%auto", "export statistics to file, %auto replaces output file extension with .stats, default=%auto")

func main() {
	var logWriter io.Writer = os.Stdout
	debug.SetGCPercent(10)
	start := time.Now()
	flag.Usage = func() {
		fmt.Fprintf(logWriter, `Nightlight Copyright (c) 2020-23 Markus L. Noga
This program comes with ABSOLUTELY NO WARRANTY.
This is free software, and you are welcome to redistribute it under certain conditions.
Refer to https://www.gnu.org/licenses/gpl-3.0.en.html for details.

Usage: %s [-flag value] (stats|stack|rgb|argb|lrgb|legal) (img0.fits ... imgn.fits)

Commands:
  stats   Show input image statistics
  stack   Stack input images
  stretch Stretch single image
  rgb     Combine color channels. Inputs are treated as r, g, b and optional l channel in that order
  run     Run a JSON job from the file specified by -job 
  legal   Show license and attribution information
  version Show version information

Flags:
`, os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	// Initialize logging to file in addition to stdout, if selected
	autoFill(log, *out, ".log")
	if *log != "" {
		logFile, err := os.Create(*log)
		if err != nil {
			panic(fmt.Sprintf("Unable to open log file %s\n", *log))
		}
		logWriter = io.MultiWriter(logWriter, logFile)
	}

	// auto-fill filenames for secondary targets
	autoFill(jpg, *out, ".jpg")
	autoFill(tiff, *out, ".tif")
	autoFill(exportStats, *out, ".html")

	// Enable CPU profiling if flagged
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintf(logWriter, "Could not create CPU profile: %s\n", err)
			os.Exit(-1)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(logWriter, "Could not start CPU profile: %s\n", err)
			os.Exit(-1)
		}
		defer pprof.StopCPUProfile()
	}

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		return
	}
	if args[0] == "stats" || args[0] == "stack" || args[0] == "stretch" || args[0] == "rgb" || args[0] == "lrgb" {
		fmt.Fprintf(logWriter, "Using location and scale estimator %d\n", *lsEst)
		stats.LSEstimator = stats.LSEstimatorMode(*lsEst)
	}

	// set defaults per command
	switch args[0] {
	case "serve":
	case "stats":
		*bpSigLow = 0
		*bpSigHigh = 0
		if *normHist == post.HNMAuto {
			*normHist = post.HNMNone
		}
		if *starBpSig < 0 {
			*starBpSig = 0
		} // default to no noise elimination
	case "stack":
		if *normHist == post.HNMAuto {
			*normHist = post.HNMLocScale
		}
		if *starBpSig < 0 {
			*starBpSig = 5
		} // default to noise elimination when working with individual subexposures
	case "stretch":
	case "rgb":
		if *normHist == post.HNMAuto {
			*normHist = post.HNMNone
		}
		if *starBpSig < 0 {
			*starBpSig = 0
		} // inputs are typically stacked and have undergone noise removal
	case "lrgb":
		if *normHist == post.HNMAuto {
			*normHist = post.HNMNone
		}
		if *starBpSig < 0 {
			*starBpSig = 0
		} // inputs are typically stacked and have undergone noise removal
	case "legal":
	case "version":
	case "help", "?":
	default:
	}

	// create context
	c := ops.NewContext(logWriter, int(*stMemory), stats.LSEstimatorMode(*lsEst))

	// glob filename arguments into an opLoadMany operator
	var err error
	opLoadMany := ops.NewOpLoadMany(args)

	// parse preprocessing flags into preprocessing sequence operator
	opDebayer := pre.NewOpDebayer(*debayer, *cfa)
	opStarDetect := pre.NewOpStarDetect(int32(*starRadius), float32(*starSig), float32(*starBpSig), float32(*starInOut), *stars)
	opPreProc := ops.NewOpSequence(
		pre.NewOpCalibrate(*dark, *flat),
		pre.NewOpBadPixel(float32(*bpSigLow), float32(*bpSigHigh), opDebayer),
		opDebayer,
		pre.NewOpDebandHoriz(float32(*debandH), int32(*debandHWindow), float32(*debandHSigma)),
		pre.NewOpDebandVert(float32(*debandV), int32(*debandVWindow), float32(*debandVSigma)),
		pre.NewOpScaleOffset(float32(*preScale), float32(*preOffset)),
		pre.NewOpBin(int32(*binning)),
		opStarDetect,
		pre.NewOpBackExtract(int32(*backGrid), float32(*backHFRFactor), float32(*backSigma), int32(*backClip), *back),
		ref.NewOpExportStats(*exportStats),
		ops.NewOpSave(*pPre, ops.EMMinMax, 1),
	)

	// run actions
	switch args[0] {
	case "serve":
		rest.MakeSandbox(*chroot, int(*setuid))
		rest.Serve(int(*port), int(*stMemory))

	case "stats":
		opSeq := ops.NewOpSequence(opLoadMany, opPreProc)
		err = runOp(opSeq, c)

	case "stack":
		opSeq := ops.NewOpSequence(
			opLoadMany,
			stack.NewOpStackBatches(
				ops.NewOpSequence(
					opPreProc,
					ref.NewOpSelectReference(ref.SRHisto, *histoRef, opStarDetect),
					ref.NewOpSelectReference(ref.SRAlign, *alignRef, opStarDetect),
					ref.NewOpFilter(int(*minStars)),
					post.NewOpMatchHistogram(post.HistoNormMode(*normHist)),
					post.NewOpAlign(int32(*alignK), float32(*alignT), post.OOBModeNaN),
					ops.NewOpSave(*pPost, ops.EMMinMax, 1),
					stack.NewOpStack(
						stack.StackMode(*stMode),
						stack.StackWeighting(*stWeight),
						float32(*stSigLow),
						float32(*stSigHigh),
					),
					opStarDetect,
					ops.NewOpSave(*batch, ops.EMMinMax, 1),
				),
			),
			opStarDetect,
			ops.NewOpSave(*out, ops.EMMinMax, 1),
			ops.NewOpSave(*tiff, ops.EM0_65535, 1),
			ops.NewOpSave(*jpg, ops.EM0_65535, float32(*jpgGamma)),
		)
		err = runOp(opSeq, c)

	case "stretch":
		opSeq := ops.NewOpSequence(
			opLoadMany,
			stretch.NewOpNormalizeRange(),
			stretch.NewOpStretchIterative(float32(*autoLoc/100), float32(*autoScale/100)),
			stretch.NewOpMidtones(float32(*midtone), float32(*midBlack)),
			stretch.NewOpGamma(float32(*gamma)),
			stretch.NewOpGammaPP(float32(*ppGamma), float32(*ppSigma)),
			stretch.NewOpScaleBlack(float32(*scaleBlack/100)),
			opStarDetect,
			ref.NewOpSelectReference(ref.SRAlign, *alignRef, opStarDetect),
			post.NewOpAlign(int32(*alignK), float32(*alignT), post.OOBModeOwnLocation),
			stretch.NewOpGaussianBlur(float32(*blurSigma)),
			stretch.NewOpUnsharpMask(float32(*usmSigma), float32(*usmGain), float32(*usmThresh)),
			ops.NewOpSave(*out, ops.EMMinMax, 1),
			ops.NewOpSave(*tiff, ops.EM0_1, 1),
			ops.NewOpSave(*jpg, ops.EM0_1, float32(*jpgGamma)),
		)
		err = runOp(opSeq, c)

	case "rgb":
		opSeq := ops.NewOpSequence(
			opLoadMany,
			opStarDetect,
			ref.NewOpSelectReference(ref.SRAlign, "%rgb", opStarDetect),

			rgb.NewOpRGBCombine(),
			rgb.NewOpRGBBalance(int32(*balBlock), float32(*balBorder), float32(*balSkipBright), float32(*balSkipDim),
				fits.RGB{R: float32(*balShR), G: float32(*balShG), B: float32(*balShB)},
				fits.RGB{R: float32(*balHiR), G: float32(*balHiG), B: float32(*balHiB)}),

			rgb.NewOpRGBToHSLuv(),
			hsl.NewOpHSLApplyLum(),

			hsl.NewOpHSLUnsharpMask(float32(*usmSigma), float32(*usmGain), float32(*usmThresh)),

			hsl.NewOpHSLNeutralizeBackground(float32(*neutSigmaLow), float32(*neutSigmaHigh)),
			hsl.NewOpHSLSaturationGamma(float32(*chromaGamma), float32(*chromaSigma)),
			hsl.NewOpHSLSelectiveSaturation(float32(*chromaFrom), float32(*chromaTo), float32(*chromaBy)),
			hsl.NewOpHSLRotateHue(float32(*rotFrom), float32(*rotTo), float32(*rotBy), float32(*rotSigma)),
			hsl.NewOpHSLSCNR(float32(*scnr)),

			hsl.NewOpHSLStretchIterative(float32(*autoLoc/100), float32(*autoScale/100)),
			hsl.NewOpHSLMidtones(float32(*midtone), float32(*midBlack)),
			hsl.NewOpHSLGamma(float32(*gamma)),
			hsl.NewOpHSLGammaPP(float32(*ppGamma), float32(*ppSigma)),
			hsl.NewOpHSLScaleOffsetChannel(2, float32(*lumScale), float32(*lumOffset)),
			hsl.NewOpHSLScaleBlack(float32(*scaleBlack/100)),

			rgb.NewOpHSLuvToRGB(),
			ops.NewOpSave(*out, ops.EMMinMax, 1),
			ops.NewOpSave(*tiff, ops.EM0_1, 1),
			ops.NewOpSave(*jpg, ops.EM0_1, float32(*jpgGamma)),
		)
		err = runOp(opSeq, c)

	case "run":
		var content []byte
		content, err = os.ReadFile(*job) // don't use := here, or the last error in this block is discarded
		if err != nil {
			panic(fmt.Sprintf("Error opening %s: %s\n", *job, err.Error()))
		}
		var opSeq ops.OpSequence
		err = json.Unmarshal(content, &opSeq)
		if err != nil {
			panic(fmt.Sprintf("Error unmarshaling JSON: %s\n", err.Error()))
		}
		err = runOp(&opSeq, c)

	case "legal":
		fmt.Fprint(logWriter, legal)

	case "version":
		fmt.Fprintf(logWriter, "Version %s\n", version)

	case "help", "?":
		flag.Usage()

	default:
		fmt.Fprintf(logWriter, "Unknown command '%s'\n\n", args[0])
		flag.Usage()
		return
	}

	if err != nil {
		fmt.Fprintf(logWriter, "Error: %s\n", err.Error())
		os.Exit(-1)
	}

	now := time.Now()
	elapsed := now.Sub(start).Round(time.Millisecond * 10)
	fmt.Fprintf(logWriter, "\nDone after %s\n", elapsed)

	// Store memory profile if flagged
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			fmt.Fprintf(logWriter, "Could not create memory profile: %s\n", err)
			os.Exit(-1)
		}
		defer f.Close()
		runtime.GC() // get up-to-date statistics
		if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
			fmt.Fprintf(logWriter, "Could not write allocation profile: %s\n", err)
			os.Exit(-1)
		}
	}
}

// if the value is equal to %auto, replace it with the base filename modified with the given extension
func autoFill(val *string, base, extension string) {
	if *val == "%auto" {
		if base != "" {
			*val = strings.TrimSuffix(base, filepath.Ext(base)) + extension
		} else {
			*val = ""
		}
	}
}

func runOp(op ops.Operator, c *ops.Context) (err error) {
	var m []byte
	m, err = json.MarshalIndent(op, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintf(c.Log, "\nRunning JSON job:\n%s\n", string(m))

	promises, err := op.MakePromises(nil, c)
	if err != nil {
		return err
	}

	_, err = ops.MaterializeAll(promises, c.MaxThreads, true)
	return err
}
