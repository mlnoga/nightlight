![Build](https://github.com/mlnoga/nightlight/workflows/Build/badge.svg)

# Nightlight: Astronomic Image Processing

Nightlight is a fast, high-quality and repeatable pipeline for astronomic image processing. Starting with faint monochrome FITS subexposures from your camera, Nightlight creates beautiful and striking stacked [images](https://photo.noga.de/#collection/4a66a793-07cd-4a69-bdc5-8dec2e82c4a2): LRGB composites in natural colors, narrowband images in the [Hubble palette](http://bf-astro.com/hubblep.htm), or in many other color schemes thanks to 32 bit floating point image processing. 

Nighlight automatically normalizes, aligns, stacks, composites and tunes your images. The in-memory architecture with randomized batching touches each file exactly once and requires no temporary files. Written in pure GoLang with selected AVX2 optimizations, Nightlight is fast and scales to use all available CPU cores efficiently. It currently supports Linux, Mac and Windows on reasonably modern AMD and Intel processors, and Linux on ARM7 like the Raspberry Pi 4. 

As a command line tool, Nightlight is ideal for creating an automated build pipeline for your images with tools like GNU [make](https://www.gnu.org/software/make/). Then apply your finishing touches by fine tuning curves in a tool like [GIMP](https://www.gimp.org/).

Discussion thread in German at [astronomie.de](https://forum.astronomie.de/threads/neuer-stacker-kombinierer.290375/).

## Releases

Download latest [binary releases](https://github.com/mlnoga/nightlight/releases) for Linux, Mac/Darwin and Windows on x86_64 bit processors and Raspberry Pi 4.

Here are some sample datasets to play with: 

* [Orion Nebula M42](https://github.com/mlnoga/dataset-M42-LRGB) in LRGB from a mono camera
* [Bubble Nebula NGC7635](https://github.com/mlnoga/dataset-NGC7635-nb) in narrowband from a mono camera
* [Arp 316 galaxy cluster](https://github.com/mlnoga/dataset-arp316-RGB-bayer) in RGB from a one-shot color DSLR

## Capabilities

* Read FITS files and normalize them to 32-bit floating point
* Estimate image location (histogram peak) and scale (peak width) via robust statistics
* Subtract dark frame and divide by flat frame
* Debayer one-shot color images
* Cosmetic correction of hot/cold pixels
* NxN Binning
* Auto-detect stars and measure half-flux radius (HFR)
* Automatic background extraction, masking out stars
* Calculate coarse alignment between images with full 2D transformations, using triangles
* Calculate fine alignment between images using optimizer on all detected stars
* Compute aligned images with bilinear interpolation
* Normalize light frame histogram to reference frame
* Stack light frames with median, mean, sigma clipping, winsorized sigma clipping, linear regression fit
* All mean-based stacking modes support noise weighting
* Goal seek sigma bounds for desired percentage outlier rejection rate
* Stack more files than fit in memory using randomized batching
* RGB and LRGB combination
* Auto-set color balance based on histogram peak and average color of detected stars
* Color composite operators: gamma, black/white point, saturation, selective saturation adjustment by hue, selective hue rotation, SCNR, background neutralization
* Unsharp masking
* Store FITS files, export to JPG

## Limitations

* Does not support RAW input from regular digital cameras, only FITS
* Does not support mosaicing or auto-cropping, output is currently identical to the extent of the reference frame
* Does not support full plate solving
* Does not support planetary disc alignment without stars in the picture, for planetary imaging

## Usage via Makefile

Usage of Nightlight in an automated build pipeline via GNU [make](https://www.gnu.org/software/make/) is recommended. A sample Makefile is provided in the [examples/](./examples/) directory. Copy that Makefile into a folder containing your light frames and edit as necessary. To get started, please enter the object name, select the desired color combination, and provide paths to your master dark and master flat files. Run `make folders` to sort your light subexposures into folders per color channel, unless your capture software already does this. Then call `make` to stack and combine channels. 

Key Makefile variables include:

| Variabe | Description |
|---------|-------------|
|OBJ      | Name of the main object in the image. Used for naming output files |
|CHAN     | Color combination to apply. Valid settings are RGB, aRGB, LRGB, HaS2O3, S2HaO3 and HaO3O3. aRGB produces a luminance file and an aligned RGB file, without performing actual LRGB combination, if you prefer to use nightlight for stacking only. |
|COMBINE  | Put arguments for special color procesing here, e.g. for Hubble color palette shifts (see direct usage below) |
|DARKnn   | Master dark frame for nn=L,R,G,B,Ha,O3,S2|
|FLATnn   | Master flat frame for nn=L,R,G,B,Ha,O3,S2|
|PARAMnn  | Any additional setttings for the given color channel (see direct usage below) |
|NL       | Path to the nightlight executable |

Key Makefile targets include:

| Target   | Description |
|----------|-------------|
| all      | Stack each color channel and perform color combination |
| clean    | Remove final output |
| realclean| Remove final and intermediate outputs| 
| folders  | Sort color channel files into folders based on filename patterns. This boldly assumes your filenames contain e.g. `_Ha_` in the name if they contain Ha data |
| count    | Count number of frames by color channel |
| backup   | Make a crude backup of your current processing results |


## Direct usage

The syntax for calling nightlight directly is: 

```
nightlight [-flag value] (stats|stack|rgb|argb|lrgb|legal|version) (light1.fit ... lightn.fit)
```

The available commands are:

| Command | Description |
|---------|-------------|
|stats    |Show input image statistics |
|stack    |Stack input images |
|rgb      |Combine color channels. Inputs are treated as r, g and b channel in that order |
|argb     |Combine color channels and align with luminance. Inputs are treated as l, r, g and b channels |
|lrgb     |Combine color channels and combine with luminance. Inputs are treated as l, r, g and b channels |
|legal    |Show license and attribution information |
|version  |Show version information |

Input and output files are automatically gunzipped and gzipped if .gz or .gzip suffixes are present in the filename. 

Available flags are:

| Flag          | Default    | Description |
|---------------|------------|-------------|
|out            |out.fits    | save output to `file` |
|jpg            |%auto       | save 8bit preview of output as JPEG to `file`. `%auto` replaces suffix of output file with .jpg |
|log            |%auto       | save log output to `file`. `%auto` replaces suffix of output file with .log |
|pre            |            | save pre-processed frames with given filename pattern, e.g. `pre%04d.fits` |
|star           |            | save star detections with given pattern, e.g. `stars%04d.fits` |
|back           |            | save extracted background with given filename pattern, e.g. `back%04d.fits` |
|post           |            | save post-processed frames with given filename pattern, e.g. `post%04d.fits` |
|batch          |            | save stacked batches with given filename pattern, e.g. `batch%04d.fits` |
|dark           |            | apply dark frame from `file` |
|flat           |            | apply flat frame from `file` |
|debayer        |            | debayer the given channel, one of R, G, B or blank for no op |
|cfa            |RGGB        | color filter array type for debayering, one of RGGB, GRBG, GBRG, BGGR|
|binning        |0           | apply NxN binning, 0 or 1=no binning |
|bpSigLow       |3.0         | low sigma for bad pixel removal as multiple of standard deviations |
|bpSigHigh      |5.0         | high sigma for bad pixel removal as multiple of standard deviations |
|starSig        |10.0        | sigma for star detection as multiple of standard deviations |
|starBpSig      |5.0         | sigma for star detection bad pixel removal as multiple of standard deviations, -1: auto |
|starRadius     |16.0        | radius for star detection in pixels |
|backGrid       |0           | automated background extraction: grid size in pixels, 0=off |
|backSigma      |1.5         | automated background extraction: sigma for detecting foreground objects |
|backClip       |0           | automated background extraction: clip the k brightest grid cells and replace with local median |
|align          |1           | 1=align frames, 0=do not align |
|alignK         |20          | use triangles fromed from K brightest stars for initial alignment |
|alignT         |1.0         | skip frames if alignment to reference frame has residual greater than this |
|lsEst          |3           | location and scale estimators 0=mean/stddev, 1=median/MAD, 2=IKSS, 3=iterative sigma-clipped sampled median and sampled Qn (standard) |
|normRange      |0           | normalize range: 1=normalize to [0,1], 0=do not normalize |
|normHist       |3           | normalize histogram: 0=do not normalize, 1=location and scale, 2=black point shift for RGB align, 3=auto |
|usmSigma       |1           | unsharp masking sigma, ~1/3 radius|
|usmGain        |0           | unsharp masking gain, 0=no op|
|usmThresh      |1           | unsharp masking threshold, in standard deviations above background|
|stMode         |5           | stacking mode. 0=median, 1=mean, 2=sigma clip, 3=winsorized sigma clip, 4=linear fit, 5=auto |
|stClipPercLow  |0.5         | set desired low clipping percentage for stacking, 0=ignore (overrides sigmas) |
|stClipPercHigh |0.5         | set desired high clipping percentage for stacking, 0=ignore (overrides sigmas) |
|stSigLow       |-1          | low sigma for stacking as multiple of standard deviations, -1: use clipping percentage to find |
|stSigHigh      |-1          | high sigma for stacking as multiple of standard deviations, -1: use clipping percentage to find |
|stWeight       |0           | weights for stacking. 0=unweighted (default), 1=by exposure, 2=by inverse noise |
|stMemory       |            | total MB of memory to use for stacking, default=80% of physical memory |
|neutSigmaLow   |-1          | neutralize background color below this threshold, <0 = no op|
|neutSigmaHigh  |-1          | keep background color above this threshold, interpolate in between, <0 = no op|
|chromaGamma    |1.0         | scale LCH chroma curve by given gamma for luminances n sigma above background, 1.0=no op |
|chromaSigma    |1.0         | only scale and add to LCH chroma for luminances n sigma above background |
|chromaFrom     |295         | scale LCH chroma for hues in [from,to] by given factor, e.g. 295 to desaturate violet stars |
|chromaTo       |40          | scale LCH chroma for hues in [from,to] by given factor, e.g. 40 to desaturate violet stars |
|chromaBy       |1           | scale LCH chroma for hues in [from,to] by given factor, e.g. -1 to desaturate violet stars |
|rotFrom        |100         | rotate LCH color angles in [from,to] by given offset, e.g. 100 to aid Hubble palette for S2HaO3 |
|rotTo          |190         | rotate LCH color angles in [from,to] by given offset, e.g. 190 to aid Hubble palette for S2HaO3 |
|rotBy          |0           | rotate LCH color angles in [from,to] by given offset, e.g. -30 to aid Hubble palette for S2HaO3 |
|scnr           |0           | apply SCNR in [0,1] to green channel, e.g. 0.5 for tricolor with S2HaO3 and 0.1 for bicolor HaO3O3 |
|autoLoc        |10          | histogram peak location in % to target with automatic curves adjustment, 0=don't|
|autoScale      |0.4         | histogram peak scale in % to target with automatic curves adjustment, 0=don't|
|gamma          |1           | apply output gamma, 1: keep linear light data |
|ppGamma        |1           | apply post-peak gamma, scales curve from location+scale...ppLimit, 1: keep linear light data |
|ppSigma        |1           | apply post-peak gamma this amount of scales from the peak (to avoid scaling background noise) |
|scaleBlack     |0.0         | move black point so histogram peak location is given value in %, 0=don't |
|cpuprofile     |            | write cpu profile to `file` |
|memprofile     |            | write memory profile to `file` |

## Build instructions

Linux and Mac already have a proper shell. On Windows, installing [Msys2](https://www.msys2.org/) is recommended to give you that. Msys2 currently needs a small [workaround](https://gist.github.com/k-takata/9b8d143f0f3fef5abdab) for the shell to run quickly. While symbolic links are not required for Nightlight, they are convenient to link into your camera capture folders. They can be enabled for Msys2 under Windows with this [settings change](https://superuser.com/a/1400340).  

If you haven't done so already, install golang via your operating system package manager, or from the [golang repository](https://golang.org/doc/install]).

Then run `GO111MODULE=on go get -u github.com/mlnoga/nightlight/cmd/nightlight`, and Nightlight will be ready for your use in `$GOPATH/bin/nightlight`.

## License

Nightlight is free software licensed under GPL3.0. See [LICENSE](./LICENSE).

The binary version of this program uses several open source libraries and components, which come with their own licensing terms. See below for an overview, and [LICENSE](./LICENSE) for details.

| Library attribution | License type |
|---------------------|--------------|
| [gonum/gonum](https://github.com/gonum/gonum) | BSD 3-Clause "New" or "Revised" License | 
| [pbnjay/memory](https://github.com/pbnjay/memory) | BSD 3-Clause "New" or "Revised" License |
| [valyala/fastrand](https://github.com/valyala/fastrand) | MIT License |
| [lucasb-eyer/go-colorful](https://github.com/lucasb-eyer/go-colorful) | MIT License |
| [klauspost/cpuid](https://github.com/klauspost/cpuid) | MIT License |
