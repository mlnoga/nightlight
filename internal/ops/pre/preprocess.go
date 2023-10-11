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
	"sync"

	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/ops"
	"github.com/mlnoga/nightlight/internal/star"
)

type OpCalibrate struct {
	ops.OpUnaryBase
	Dark  string     `json:"dark"`
	Flat  string     `json:"flat"`
	mutex sync.Mutex `json:"-"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpCalibrateDefaults() }) } // register the operator for JSON decoding

func NewOpCalibrateDefaults() *OpCalibrate { return NewOpCalibrate("", "") }

func NewOpCalibrate(dark, flat string) *OpCalibrate {
	op := &OpCalibrate{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "calibrate"}},
		Dark:        dark,
		Flat:        flat,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpCalibrate) UnmarshalJSON(data []byte) error {
	type defaults OpCalibrate
	def := defaults(*NewOpCalibrateDefaults())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	// *op = OpCalibrate(def)   // triggers linter error "mutex passed by value", hence:
	op.OpUnaryBase = def.OpUnaryBase
	op.Dark = def.Dark
	op.Flat = def.Flat
	op.mutex = sync.Mutex{}

	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpCalibrate) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if err = op.init(c); err != nil {
		return nil, err
	} // lazy init of dark and flat frames

	if c.DarkFrame != nil {
		if !fits.EqualInt32Slice(f.Naxisn, c.DarkFrame.Naxisn) {
			return nil, fmt.Errorf("%d: Light dimensions %v differ from dark dimensions %v",
				f.ID, f.Naxisn, c.DarkFrame.Naxisn)
		}
		Subtract(f.Data, f.Data, c.DarkFrame.Data)
		f.Stats.Clear()
	}

	if c.FlatFrame != nil {
		if !fits.EqualInt32Slice(f.Naxisn, c.FlatFrame.Naxisn) {
			return nil, fmt.Errorf("%d: Light dimensions %v differ from flat dimensions %v",
				f.ID, f.Naxisn, c.FlatFrame.Naxisn)
		}
		Divide(f.Data, f.Data, c.FlatFrame.Data, c.FlatFrame.Stats.Max())
		f.Stats.Clear()
	}
	return f, nil
}

// Load dark and flat frames if applicable
func (op *OpCalibrate) init(c *ops.Context) error {
	op.mutex.Lock()
	defer op.mutex.Unlock()
	if !((op.Dark != "" && c.DarkFrame == nil) ||
		(op.Flat != "" && c.FlatFrame == nil)) {
		return nil
	}

	var promises []ops.Promise
	for i, name := range []string{op.Dark, op.Flat} {
		if name != "" {
			promise, err := ops.NewOpLoad(-(i+1), name).MakePromises(nil, c)
			if err != nil {
				return err
			}
			if len(promise) != 1 {
				return errors.New("load operator did not create exactly one promise")
			}
			promises = append(promises, promise[0])
		}
	}

	images, err := ops.MaterializeAll(promises, c.MaxThreads, false)
	if err != nil {
		return err
	}

	if op.Dark != "" {
		c.DarkFrame = images[0]
		if op.Flat != "" {
			c.FlatFrame = images[1]
		}
	} else if op.Flat != "" {
		c.FlatFrame = images[0]
	}

	if c.DarkFrame != nil && c.FlatFrame != nil && !fits.EqualInt32Slice(c.DarkFrame.Naxisn, c.FlatFrame.Naxisn) {
		return fmt.Errorf("dark dimensions %v differ from flat dimensions %v",
			c.DarkFrame.Naxisn, c.FlatFrame.Naxisn)
	}
	return nil
}

type OpBadPixel struct {
	ops.OpUnaryBase
	SigmaLow  float32    `json:"sigmaLow"`
	SigmaHigh float32    `json:"sigmaHigh"`
	Debayer   *OpDebayer `json:"-"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpBadPixelDefaults() }) } // register the operator for JSON decoding

func NewOpBadPixelDefaults() *OpBadPixel { return NewOpBadPixel(3, 5, nil) }

func NewOpBadPixel(bpSigLow, bpSigHigh float32, debayer *OpDebayer) *OpBadPixel {
	op := &OpBadPixel{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "badPixel"}},
		SigmaLow:    bpSigLow,
		SigmaHigh:   bpSigHigh,
		Debayer:     debayer,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpBadPixel) UnmarshalJSON(data []byte) error {
	type defaults OpBadPixel
	def := defaults(*NewOpBadPixelDefaults())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpBadPixel(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpBadPixel) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.SigmaLow == 0 || op.SigmaHigh == 0 {
		return f, nil
	}

	if op.Debayer == nil || op.Debayer.Channel == "" {
		var bpm []int32
		bpm, f.MedianDiffStats = BadPixelMap(f.Data, f.Naxisn[0], op.SigmaLow, op.SigmaHigh)
		mask := star.CreateMask(f.Naxisn[0], 1.5)
		MedianFilterSparse(f.Data, bpm, mask)
		fmt.Fprintf(c.Log, "%d: Removed %d bad pixels (%.2f%%) with sigma low=%.2f high=%.2f\n",
			f.ID, len(bpm), 100.0*float32(len(bpm))/float32(f.Pixels), op.SigmaLow, op.SigmaHigh)
	} else {
		numRemoved, err := CosmeticCorrectionBayer(f.Data, f.Naxisn[0], op.Debayer.Channel, op.Debayer.ColorFilterArray, op.SigmaLow, op.SigmaHigh)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(c.Log, "%d: Removed %d bad bayer pixels (%.2f%%) with sigma low=%.2f high=%.2f\n",
			f.ID, numRemoved, 100.0*float32(numRemoved)/float32(f.Pixels), op.SigmaLow, op.SigmaHigh)
	}
	return f, nil
}

type OpDebayer struct {
	ops.OpUnaryBase
	Channel          string `json:"channel"`
	ColorFilterArray string `json:"colorFilterArray"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpDebayerDefaults() }) } // register the operator for JSON decoding

func NewOpDebayerDefaults() *OpDebayer { return NewOpDebayer("", "RGGB") }

func NewOpDebayer(channel, cfa string) *OpDebayer {
	op := &OpDebayer{
		OpUnaryBase:      ops.OpUnaryBase{OpBase: ops.OpBase{Type: "debayer"}},
		Channel:          channel,
		ColorFilterArray: cfa,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpDebayer) UnmarshalJSON(data []byte) error {
	type defaults OpDebayer
	def := defaults(*NewOpDebayerDefaults())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpDebayer(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpDebayer) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.Channel == "" || op.ColorFilterArray == "" {
		return f, nil
	}
	f.Data, f.Naxisn[0], err = DebayerBilinear(f.Data, f.Naxisn[0], op.Channel, op.ColorFilterArray)
	if err != nil {
		return nil, err
	}
	f.Pixels = int32(len(f.Data))
	f.Naxisn[1] = f.Pixels / f.Naxisn[0]
	fmt.Fprintf(c.Log, "%d: Debayered channel %s from cfa %s, new size %dx%d\n", f.ID, op.Channel, op.ColorFilterArray, f.Naxisn[0], f.Naxisn[1])

	return f, nil
}

type OpScaleOffset struct {
	ops.OpUnaryBase
	Scale  float32 `json:"scale"`
	Offset float32 `json:"offset"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpScaleOffsetDefault() }) } // register the operator for JSON decoding

func NewOpScaleOffsetDefault() *OpScaleOffset { return NewOpScaleOffset(1, 0) }

func NewOpScaleOffset(scale, offset float32) *OpScaleOffset {
	op := &OpScaleOffset{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "scaleOffset"}},
		Scale:       scale,
		Offset:      offset,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpScaleOffset) UnmarshalJSON(data []byte) error {
	type defaults OpScaleOffset
	def := defaults(*NewOpScaleOffsetDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpScaleOffset(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpScaleOffset) Apply(f *fits.Image, c *ops.Context) (fOut *fits.Image, err error) {
	if op.Scale == 1 && op.Offset == 0 {
		return f, nil
	}
	fmt.Fprintf(c.Log, "%d: Applying pixel math x = x * %.3f + %.3f%%\n", f.ID, op.Scale, op.Offset*100)
	f.ApplyScaleOffset(op.Scale, op.Offset)
	return f, nil
}

type OpBin struct {
	ops.OpUnaryBase
	BinSize int32 `json:"binSize"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpBinDefaults() }) } // register the operator for JSON decoding

func NewOpBinDefaults() *OpBin { return NewOpBin(1) }

func NewOpBin(binning int32) *OpBin {
	op := &OpBin{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "bin"}},
		BinSize:     binning,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpBin) UnmarshalJSON(data []byte) error {
	type defaults OpBin
	def := defaults(*NewOpBinDefaults())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpBin(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpBin) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.BinSize <= 1 {
		return f, nil
	}
	f = fits.NewImageBinNxN(f, op.BinSize)
	fmt.Fprintf(c.Log, "%d: After %xx%d binning, new image size %dx%d\n", f.ID, op.BinSize, op.BinSize, f.Naxisn[0], f.Naxisn[1])
	return f, nil
}

type OpBackExtract struct {
	ops.OpUnaryBase
	GridSize  int32       `json:"gridSize"`
	HFRFactor float32     `json:"hfrFactor"`
	Sigma     float32     `json:"sigma"`
	Clip      int32       `json:"clip"`
	Save      *ops.OpSave `json:"save"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpBackExtractDefault() }) } // register the operator for JSON decoding

func NewOpBackExtractDefault() *OpBackExtract { return NewOpBackExtract(0, 4.0, 1.5, 0, "") }

func NewOpBackExtract(backGrid int32, hfrFactor, backSigma float32, backClip int32, savePattern string) *OpBackExtract {
	op := &OpBackExtract{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "backExtract"}},
		GridSize:    backGrid,
		HFRFactor:   hfrFactor,
		Sigma:       backSigma,
		Clip:        backClip,
		Save:        ops.NewOpSave(savePattern, ops.EMMinMax, 1),
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpBackExtract) UnmarshalJSON(data []byte) error {
	type defaults OpBackExtract
	def := defaults(*NewOpBackExtractDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpBackExtract(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpBackExtract) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.GridSize <= 0 {
		return f, nil
	}

	bg := NewBackground(f.Data, f.Naxisn[0], op.GridSize, op.Sigma, op.Clip, f.Stars, op.HFRFactor, c.Log)
	fmt.Fprintf(c.Log, "%d: %s\n", f.ID, bg)

	if op.Save == nil || op.Save.FilePattern == "" {
		// faster, does not materialize background image explicitly
		err = bg.Subtract(f.Data)
		if err != nil {
			return nil, err
		}
	} else {
		bgData := bg.Render()
		bgFits := fits.NewImageFromNaxisn(f.Naxisn, bgData)
		promise := func() (f *fits.Image, err error) { return bgFits, nil }
		_, err := op.Save.MakePromises([]ops.Promise{promise}, c)
		if err != nil {
			return nil, err
		}
		Subtract(f.Data, f.Data, bgData)
		bgFits.Data, bgData = nil, nil
	}
	f.Stats.Clear()
	return f, nil
}

type OpStarDetect struct {
	ops.OpUnaryBase
	Radius        int32       `json:"radius"`
	Sigma         float32     `json:"sigma"`
	BadPixelSigma float32     `json:"badPixelSigma"`
	InOutRatio    float32     `json:"inOutRatio"`
	Save          *ops.OpSave `json:"save"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpStarDetectDefault() }) } // register the operator for JSON decoding

func NewOpStarDetectDefault() *OpStarDetect { return NewOpStarDetect(16, 10, 0, 10, "") }

func NewOpStarDetect(starRadius int32, starSig, starBpSig, starInOut float32, savePattern string) *OpStarDetect {
	op := &OpStarDetect{
		OpUnaryBase:   ops.OpUnaryBase{OpBase: ops.OpBase{Type: "starDetect"}},
		Radius:        starRadius,
		Sigma:         starSig,
		BadPixelSigma: starBpSig,
		InOutRatio:    starInOut,
		Save:          ops.NewOpSave(savePattern, ops.EMMinMax, 1),
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpStarDetect) UnmarshalJSON(data []byte) error {
	type defaults OpStarDetect
	def := defaults(*NewOpStarDetectDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpStarDetect(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpStarDetect) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.Radius == 0 || op.Sigma == 0 {
		return f, nil
	}
	if f.Stats == nil {
		return nil, errors.New("missing stats")
	}

	f.Stars, _, f.HFR = star.FindStars(f.Data, f.Naxisn[0], f.Stats.Location(), f.Stats.Scale(), op.Sigma, op.BadPixelSigma, op.InOutRatio, op.Radius, f.MedianDiffStats)
	fmt.Fprintf(c.Log, "%d: Stars %d HFR %.2f %v\n", f.ID, len(f.Stars), f.HFR, f.Stats)

	if op.Save != nil && op.Save.FilePattern != "" {
		stars := fits.NewImageFromStars(f, 2.0)
		promise := func() (f *fits.Image, err error) { return stars, nil }
		promises, err := op.Save.MakePromises([]ops.Promise{promise}, c)
		if err != nil {
			return nil, err
		}
		_, err = promises[0]()
		if err != nil {
			return nil, err
		}
	}

	return f, nil
}
