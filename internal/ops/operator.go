// Copyright (C) 2020 Markus L. Noga
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed ins the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package ops

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/star"
	"github.com/mlnoga/nightlight/internal/stats"
	"github.com/pbnjay/memory"
)

// An execution context for operators
type Context struct {
	Log             io.Writer
	LSEstimatorMode stats.LSEstimatorMode
	MemoryMB        int // memory.TotalMemory()/1024/1024
	StackMemoryMB   int // MemoryMB*7/10
	MaxThreads      int `json:"maxThreads"`
	DarkFrame       *fits.Image
	FlatFrame       *fits.Image
	AlignNaxisn     []int32
	AlignStars      []star.Star
	AlignHFR        float32
	MatchHisto      *stats.Stats
	RefFrameError   error
	LumFrame        *fits.Image

	StatsTotal     int
	StatsProcessed int
	StatsFile      *os.File      `json:"-"` // the output file being written to. do not use directly
	StatsBufWriter *bufio.Writer `json:"-"` // buffered writer for the output file. use this
}

func NewContext(log io.Writer, stMemory int, lsEstimatorMode stats.LSEstimatorMode) *Context {
	memoryMB := int(memory.TotalMemory() / 1024 / 1024)
	return &Context{
		Log:             log,
		LSEstimatorMode: lsEstimatorMode,
		MemoryMB:        memoryMB,
		StackMemoryMB:   stMemory,
		MaxThreads:      runtime.GOMAXPROCS(0),
	}
}

// A promise for a FITS image. Returns a materialized image, or an error
type Promise func() (f *fits.Image, err error)

// Materializes all promises with given concurrency limit
func MaterializeAll(ins []Promise, maxThreads int, forget bool) (outs []*fits.Image, err error) {
	if len(ins) == 0 {
		return nil, nil
	}
	if !forget {
		outs = make([]*fits.Image, len(ins))
	}
	limiter := make(chan bool, maxThreads)
	errs := make(chan error, len(ins))
	for i, in := range ins {
		limiter <- true
		go func(i int, theIn Promise) {
			defer func() { <-limiter }()
			f, err := theIn() // materialize the promise
			if err != nil {
				if !forget {
					outs[i] = nil
				}
				errs <- err
				return
			}
			if !forget {
				outs[i] = f
			}
			errs <- nil
		}(i, in)
	}
	for i := 0; i < cap(limiter); i++ { // wait for goroutines to finish
		limiter <- true
	}
	for i := 0; i < len(ins); i++ { // collect errors
		e := <-errs
		if e != nil {
			if err == nil {
				err = e
			} else if err.Error() == e.Error() {
				// do nothing
			} else {
				err = fmt.Errorf("%s; %s", err.Error(), e.Error())
			}
		}
	}
	return RemoveNils(outs), err
}

// Remove nils from an array of fits.Images, editing the underlying array in place
func RemoveNils(lights []*fits.Image) []*fits.Image {
	o := 0
	for i := 0; i < len(lights); i += 1 {
		if lights[i] != nil {
			lights[o] = lights[i]
			o += 1
		}
	}
	for i := o; i < len(lights); i++ {
		lights[i] = nil
	}
	return lights[:o]
}

// An general image processing operator: takes n promises as inputs,
// and produces m promises as output or an error
type Operator interface {
	GetType() string
	MakePromises(ins []Promise, c *Context) (outs []Promise, err error)
}

// Base type for operators, including type information for JSON serializing/deserializing
type OpBase struct {
	Type string `json:"type"`
}

func (op *OpBase) GetType() string { return op.Type }

// Factory method for subclasses of unary operators. For JSON serializing/deserializing
type OperatorFactory func() Operator

// Mapping from unary operator type strings to factory method for the type
var operatorFactories = map[string]OperatorFactory{}

// Returns the operator factory for a given type string
func GetOperatorFactory(t string) OperatorFactory {
	return operatorFactories[t]
}

// Registers a given type string for a given type of UnaryOperator, identified via an exemplar generator
func SetOperatorFactory(f OperatorFactory) {
	op := f()
	t := op.GetType()
	if GetOperatorFactory(t) != nil {
		panic(fmt.Sprintf("error: re-registering operator key %s\n", t))
	}
	operatorFactories[t] = f
}

// A unary image processing operator: given n promises as inputs,
// applies itself to each of them individually and returns n output promises or an error
type OperatorUnary interface {
	Operator
	Apply(f *fits.Image, c *Context) (fOut *fits.Image, err error)
}

// Abstract base type for unary operators. Uses golang workaround for abstract classes
// from https://golangbyexample.com/go-abstract-class/
type OpUnaryBase struct {
	OpBase
	Apply func(f *fits.Image, c *Context) (fOut *fits.Image, err error) `json:"-"`
	// Careful: copying an OpUnary base as value into a new instance will lead to subsequent
	// difficult to trace errors, as Apply() will still tie the method receiver to the
	// address of the original. You must set op.OpUnaryBase.Apply=op.Apply after such a copy
	// to avoid this error
}

func (op *OpUnaryBase) MakePromises(ins []Promise, c *Context) (outs []Promise, err error) {
	if len(ins) == 0 {
		return nil, fmt.Errorf("unary operator with %d inputs", len(ins))
	}
	outs = make([]Promise, len(ins))
	for i, in := range ins {
		outs[i] = op.MakePromise(in, c)
	}
	return outs, nil
}

func (op *OpUnaryBase) MakePromise(in Promise, c *Context) (out Promise) {
	return func() (f *fits.Image, err error) {
		if f, err = in(); err != nil {
			return nil, err
		} // materialize input promise
		if f, err = op.Apply(f, c); err != nil {
			return nil, err
		} // apply unary operator
		return f, nil // wrap output in promise
	}
}

// Load a single FITS image from a single filename. Takes zero inputs, produces one output
type OpLoad struct {
	OpBase
	ID       int    `json:"id"`
	FileName string `json:"fileName"`
}

func init() { SetOperatorFactory(func() Operator { return NewOpLoadDefault() }) } // register the operator for JSON decoding

func NewOpLoadDefault() *OpLoad { return NewOpLoad(0, "") }

func NewOpLoad(id int, fileName string) *OpLoad {
	return &OpLoad{
		OpBase:   OpBase{Type: "load"},
		ID:       id,
		FileName: fileName,
	}
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpLoad) UnmarshalJSON(data []byte) error {
	type defaults OpLoad
	def := defaults(*NewOpLoadDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpLoad(def)
	return nil
}

// Load image from a file. Ignores any f argument provided
func (op *OpLoad) MakePromises(ins []Promise, c *Context) (outs []Promise, err error) {
	if len(ins) > 0 {
		return nil, fmt.Errorf("%s operator with non-zero input", op.Type)
	}
	if !isPathAllowed(op.FileName) {
		return nil, errors.New("filename outside current directory tree, aborting")
	}

	out := func() (f *fits.Image, err error) {
		// no inputs to materialize
		return op.Apply(nil, c)
	}
	return []Promise{out}, nil
}

// Returns true if a path is considered safe, i.e. not an absolute path,
// and doesn't contain the ".." characters to change to a parent directory
func isPathAllowed(p string) bool {
	if filepath.IsAbs(p) {
		return false
	} // relative paths only
	if strings.Contains(p, "..") {
		return false
	} // no going outside the tree
	return true
}

func (op *OpLoad) Apply(fUnused *fits.Image, c *Context) (result *fits.Image, err error) {
	f, err := fits.NewImageFromFile(op.FileName, op.ID, c.Log)
	if err != nil {
		return nil, err
	}

	warning := ""
	if f.Stats.Max()-f.Stats.Min() < 1e-8 {
		warning = "; WARNING low dynamic range"
	}

	fmt.Fprintf(c.Log, "%d: Loaded %s image with %v from %s%s\n",
		f.ID, f.DimensionsToString(), f.Stats, f.FileName, warning)
	return f, nil
}

// Load many FITS images from a slice of filename patterns with wildcards.
// Takes zero inputs, produces n outputs
type OpLoadMany struct {
	OpBase
	FilePatterns []string `json:"filePatterns"`
}

func init() { SetOperatorFactory(func() Operator { return NewOpLoadManyDefault() }) } // register the operator for JSON decoding

func NewOpLoadManyDefault() *OpLoadMany { return NewOpLoadMany(nil) }

func NewOpLoadMany(filePatterns []string) *OpLoadMany {
	return &OpLoadMany{
		OpBase:       OpBase{Type: "loadMany"},
		FilePatterns: filePatterns,
	}
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpLoadMany) UnmarshalJSON(data []byte) error {
	type defaults OpLoadMany
	def := defaults(*NewOpLoadManyDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpLoadMany(def)
	return nil
}

// Turn filename wildcards into list of file load operators
func (op *OpLoadMany) MakePromises(ins []Promise, c *Context) (outs []Promise, err error) {
	if len(ins) > 0 {
		return nil, fmt.Errorf("%s operator with non-zero input", op.Type)
	}
	for _, pattern := range op.FilePatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			if !isPathAllowed(match) {
				fmt.Fprintf(c.Log, "Pattern match outside current directory tree, skipping\n")
				continue
			}
			opLoad := NewOpLoad(len(outs), match)
			promises, err := opLoad.MakePromises(nil, c)
			if err != nil {
				return nil, err
			}
			if len(promises) != 1 {
				return nil, fmt.Errorf("%s operator did not return exactly one promise", opLoad.Type)
			}
			outs = append(outs, promises[0])
		}
	}
	if len(outs) == 0 {
		return nil, fmt.Errorf("%s operator with no files to load from pattern %v", op.Type, op.FilePatterns)
	}
	fmt.Fprintf(c.Log, "Found %d files.\n", len(outs))
	return outs, nil
}

// Value range for exporting data to 16-bit TIFF or JPEG
type ExportMode int

const (
	EMMinMax ExportMode = iota
	EM0_1
	EM0_255
	EM0_65535
)

// Saves given promise under a given filename, with pattern expansion for %d based on the image id.
// Takes one input, produces one output (the materialized but unchanged input)
type OpSave struct {
	OpUnaryBase
	FilePattern string     `json:"filePattern"`
	ExportMode  ExportMode `json:"saveMode"`
	Gamma       float32    `json:"gamma"`
}

func init() { SetOperatorFactory(func() Operator { return NewOpSaveDefault() }) } // register the operator for JSON decoding

func NewOpSaveDefault() *OpSave { return NewOpSave("", EMMinMax, 1) }

func NewOpSave(filenamePattern string, exportMode ExportMode, gamma float32) *OpSave {
	op := &OpSave{
		OpUnaryBase: OpUnaryBase{OpBase: OpBase{Type: "save"}},
		FilePattern: filenamePattern,
		ExportMode:  exportMode,
		Gamma:       gamma,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpSave) UnmarshalJSON(data []byte) error {
	type defaults OpSave
	def := defaults(*NewOpSaveDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpSave(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpSave) Apply(f *fits.Image, c *Context) (result *fits.Image, err error) {
	if op.FilePattern == "" {
		return f, nil
	}
	if f == nil {
		return nil, fmt.Errorf("cannot save nil file to %s", op.FilePattern)
	}
	fileName := op.FilePattern
	if strings.Contains(fileName, "%d") {
		fileName = fmt.Sprintf(op.FilePattern, f.ID)
	}
	fnLower := strings.ToLower(fileName)

	if err != nil {
		return nil, err
	}
	var min, max float32
	switch op.ExportMode {
	case EMMinMax:
		min = f.Stats.Min()
		max = f.Stats.Max()
	case EM0_1:
		min = 0
		max = 1
	case EM0_255:
		min = 0
		max = 255
	case EM0_65535:
		min = 0
		max = 65535
	}

	if strings.HasSuffix(fnLower, ".fits") || strings.HasSuffix(fnLower, ".fit") || strings.HasSuffix(fnLower, ".fts") ||
		strings.HasSuffix(fnLower, ".fits.gz") || strings.HasSuffix(fnLower, ".fit.gz") || strings.HasSuffix(fnLower, ".fts.gz") ||
		strings.HasSuffix(fnLower, ".fits.gzip") || strings.HasSuffix(fnLower, ".fit.gzip") || strings.HasSuffix(fnLower, ".fts.gzip") {
		fmt.Fprintf(c.Log, "%d: Writing %s pixel FITS to %s\n", f.ID, f.DimensionsToString(), fileName)
		err = f.WriteFile(fileName)
	} else if strings.HasSuffix(fnLower, ".tiff") || strings.HasSuffix(fnLower, ".tif") {
		if len(f.Naxisn) == 2 {
			fmt.Fprintf(c.Log, "%d: Writing %s pixel mono 16-bit TIFF to %s with min=%g max=%g...\n",
				f.ID, f.DimensionsToString(), fileName, min, max)
			f.WriteMonoTIFF16ToFile(fileName, min, max, op.Gamma)
		} else if len(f.Naxisn) == 3 && f.Naxisn[2] == 3 {
			fmt.Fprintf(c.Log, "%d: Writing %s pixel color 16-bit TIFF to %s with min=%g max=%g...\n",
				f.ID, f.DimensionsToString(), fileName, min, max)
			f.WriteTIFF16ToFile(fileName, min, max, op.Gamma)
		} else {
			return nil, fmt.Errorf("%d: unable to write %s pixel image as 16-bit TIFF to %s", f.ID, f.DimensionsToString(), fileName)
		}
	} else if strings.HasSuffix(fnLower, ".jpeg") || strings.HasSuffix(fnLower, ".jpg") {
		if len(f.Naxisn) == 2 {
			fmt.Fprintf(c.Log, "%d: Writing %s pixel mono JPEG to %s with min=%g max=%g gamma=%g...\n",
				f.ID, f.DimensionsToString(), fileName, min, max, op.Gamma)
			f.WriteMonoJPGToFile(fileName, min, max, op.Gamma, 95)
		} else if len(f.Naxisn) == 3 && f.Naxisn[2] == 3 {
			fmt.Fprintf(c.Log, "%d: Writing %s pixel color JPEG to %s with min=%g max=%g gamma=%g...\n",
				f.ID, f.DimensionsToString(), fileName, min, max, op.Gamma)
			f.WriteJPGToFile(fileName, min, max, op.Gamma, 95)
		} else {
			return nil, fmt.Errorf("%d: unable to write %s pixel image as JPEG to %s", f.ID, f.DimensionsToString(), fileName)
		}
	} else {
		err = fmt.Errorf("unknown suffix \"%s\" for file %s", filepath.Ext(fileName), fileName)
	}
	if err != nil {
		return nil, fmt.Errorf("%d: error writing to file %s: %s", f.ID, fileName, err.Error())
	}
	return f, nil
}

// Applies a sequence of operators to a promise. Number of inputs, outputs as per the chained steps
type OpSequence struct {
	OpBase
	Steps    []Operator        `json:"-"`     // the actual steps
	StepsRaw []json.RawMessage `json:"steps"` // helper for unmarshaling
}

func init() { SetOperatorFactory(func() Operator { return NewOpSequenceDefault() }) } // register the operator for JSON decoding

func NewOpSequenceDefault() *OpSequence { return NewOpSequence() }

func NewOpSequence(steps ...Operator) *OpSequence {
	return &OpSequence{
		OpBase: OpBase{Type: "seq"},
		Steps:  steps,
	}
}

// Unmarshals a sequence of polymorphic operators from JSON.
// Uses temporary op.StepsRaw inspired by https://alexkappa.medium.com/json-polymorphism-in-go-4cade1e58ed1
func (op *OpSequence) UnmarshalJSON(b []byte) error {
	type alias OpSequence
	var tmp alias
	err := json.Unmarshal(b, &tmp)
	if err != nil {
		return err
	}
	op.Type = tmp.Type

	for _, raw := range tmp.StepsRaw {
		var step OpBase
		err = json.Unmarshal(raw, &step)
		if err != nil {
			return err
		}

		var i Operator
		if factory := GetOperatorFactory(step.Type); factory != nil {
			i = factory()
		} else {
			return fmt.Errorf("unknown operator type '%s' in raw JSON message '%s'", step.Type, string(raw))
		}
		err = json.Unmarshal(raw, i)
		if err != nil {
			return err
		}
		op.Steps = append(op.Steps, i)
	}
	return nil
}

// Appends one or more operators to the existing sequence
func (op *OpSequence) Append(steps ...Operator) {
	op.Steps = append(op.Steps, steps...)
}

// Marshals a sequence with polymorphic operators to JSON.
// Uses the actual op.Steps with label "steps", and ignores op.StepsRaw
func (op *OpSequence) MarshalJSON() (bs []byte, err error) {
	buf := bytes.Buffer{}
	buf.WriteString("{\"type\":")
	inner, err := json.Marshal(op.Type)
	if err != nil {
		return nil, err
	}
	buf.Write(inner)
	fmt.Fprintf(&buf, ", \"steps\":")
	inner, err = json.Marshal(op.Steps)
	if err != nil {
		return nil, err
	}
	buf.Write(inner)
	buf.WriteRune('}')
	return buf.Bytes(), nil
}

func (op *OpSequence) MakePromises(ins []Promise, c *Context) (outs []Promise, err error) {
	return op.applyRecursive(op.Steps, ins, c)
}

func (op *OpSequence) applyRecursive(steps []Operator, ins []Promise, c *Context) (outs []Promise, err error) {
	if len(steps) == 0 {
		return ins, nil
	}
	ins, err = steps[0].MakePromises(ins, c)
	if err != nil {
		return nil, err
	}
	return op.applyRecursive(steps[1:], ins, c)
}
