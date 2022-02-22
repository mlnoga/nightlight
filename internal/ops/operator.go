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


package ops

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"github.com/mlnoga/nightlight/internal/fits"
)


// An operator sourcing a single FITS image
type OperatorSource interface {
	Apply(logWriter io.Writer) (fOut *fits.Image, err error) 
}

// An operator working on a single FITS image, transforming/overwriting it and its data
type OperatorUnary interface {
	Apply(f *fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) 
}

// An operator working on many FITS images in parallel, transforming/overwriting them
type OperatorParallel interface {
	ApplyToFiles(sources []*OpLoadFile, logWriter io.Writer) (fOut []*fits.Image, err error) 
	ApplyToFITS(sources []*fits.Image, logWriter io.Writer) (fOut []*fits.Image, err error) 
}

type OperatorJoin interface {
	Apply(f []*fits.Image, logWriter io.Writer) (result *fits.Image, err error)
}

type OperatorJoinFiles interface {
	Apply(opLoadFiles []*OpLoadFile, logWriter io.Writer) (result *fits.Image, err error)
}



type OpInMemory struct {
	Fits        *fits.Image    `json:"-"`
}
var _ OperatorSource = (*OpInMemory)(nil) // Compile time assertion: type implements the interface

func NewInMemory(fits *fits.Image) *OpInMemory {
	return &OpInMemory{
		Fits : fits,
	}
}

// Start processing from an in-memory fits
func (op *OpInMemory) Apply(logWriter io.Writer) (fOut *fits.Image, err error) {
	return op.Fits, nil
}


type OpLoadFile struct {
	ID 		    int     `json:"id"`
	FileName    string  `json:"fileName"`
}
var _ OperatorSource = (*OpLoadFile)(nil) // Compile time assertion: type implements the interface


func NewOpLoadFile(id int, fileName string) *OpLoadFile {
	return &OpLoadFile{
		ID : id,
		FileName : fileName,
	}
}

// Load image from a file
func (op *OpLoadFile) Apply(logWriter io.Writer) (fOut *fits.Image, err error) {
	f, err:=fits.NewImageFromFile(op.FileName, op.ID, logWriter)
	if err!=nil { return nil, err }
	fmt.Fprintf(logWriter, "%d: Loaded %s pixel light frame with %v from %s\n", f.ID, f.DimensionsToString(), f.Stats, f.FileName)
	return f, nil	
}


// Turn filename wildcards into list of file load operators
func NewOpLoadFiles(args []string, logWriter io.Writer) (loaders []*OpLoadFile, err error) {
	if len(args)<1 { return nil, errors.New("No frames to process.") }
	ops:=[]*OpLoadFile{}
	for _, pattern := range args {
		matches, err := filepath.Glob(pattern)
		if err!=nil { return nil, err }
		for _,match:=range(matches) {
			ops=append(ops, NewOpLoadFile(len(ops), match))
		}
	}
	fmt.Fprintf(logWriter, "Found %d files:\n", len(ops))
	for _, op:=range(ops) {
		fmt.Fprintf(logWriter, "%d: %s\n",op.ID, op.FileName)
	}
	return ops, nil
}


// Load frame from FITS file and calculate basic stats and noise
func LoadAndCalcStats(fileName string, id int, role string, logWriter io.Writer) (f *fits.Image, err error) {
	f, err=fits.NewImageFromFile(fileName, id, logWriter)
	if err!=nil { return nil, err }

	fmt.Fprintf(logWriter, "%d: Loaded %s pixel %s frame with %v from %s\n", id, f.DimensionsToString(), role, f.Stats, fileName)
	if f.Stats.Max()-f.Stats.Min()<1e-8 {
		fmt.Fprintf(logWriter, "Warnining: %s file %d may be degenerate\n", role, id)
	}

	return f, nil
}


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
func (op *OpSave) Apply(f *fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) {
	if !op.Active || op.FilePattern=="" { return f, nil }

	fileName:=op.FilePattern
	if strings.Contains(fileName, "%d") {
		fileName=fmt.Sprintf(op.FilePattern, f.ID)
	}
	fnLower:=strings.ToLower(fileName)

	if strings.HasSuffix(fnLower,".fits")      || strings.HasSuffix(fnLower,".fit")      || strings.HasSuffix(fnLower,".fts")     ||
	   strings.HasSuffix(fnLower,".fits.gz")   || strings.HasSuffix(fnLower,".fit.gz")   || strings.HasSuffix(fnLower,".fts.gz")  ||
	   strings.HasSuffix(fnLower,".fits.gzip") || strings.HasSuffix(fnLower,".fit.gzip") || strings.HasSuffix(fnLower,".fts.gzip") {     
		fmt.Fprintf(logWriter,"%d: Writing %s pixel FITS to %s\n", f.ID, f.DimensionsToString(), fileName)
		err=f.WriteFile(fileName)
	} else if strings.HasSuffix(fnLower,".jpeg") || strings.HasSuffix(fnLower,".jpg") {
		if len(f.Naxisn)==2 {
			fmt.Fprintf(logWriter, "%d: Writing %s pixel mono JPEG to %s ...\n", f.ID, f.DimensionsToString(), fileName)
			f.WriteMonoJPGToFile(fileName, 95)
		} else if len(f.Naxisn)==3 && f.Naxisn[2]==3 {
			fmt.Fprintf(logWriter, "%d: Writing %s pixel color JPEG to %s ...\n", f.ID, f.DimensionsToString(), fileName)
			f.WriteJPGToFile(fileName, 95)
		} else {
			return nil, errors.New(fmt.Sprintf("%d: Unable to write %s pixel image as JPEG to %s\n", f.ID, f.DimensionsToString(), fileName))
		}
	} else {
		err=errors.New("Unknown suffix")
	}
	if err!=nil { return nil, errors.New(fmt.Sprintf("%d: Error writing to file %s: %s\n", f.ID, fileName, err.Error())) }
	return f, nil;
}


type OpSequence struct {
	Active      bool
	Steps       []OperatorUnary   `json:"steps"`
}
var _ OperatorUnary = (*OpSequence)(nil) // Compile time assertion: type implements the interface

func NewOpSequence(steps []OperatorUnary) *OpSequence {
	return &OpSequence{Active: steps!=nil && len(steps)>0, Steps: steps}
}

func (op *OpSequence) Apply(f *fits.Image, logWriter io.Writer) (fOut *fits.Image, err error) {
	if !op.Active { return f, nil }
	for _,step:=range op.Steps {
		var err error
		f, err=step.Apply(f, logWriter)
		if err!=nil { return nil, err}
	}
	return f, nil
}




type OpParallel struct {
	Operator    OperatorUnary  `json:"operator"`
	MaxThreads  int64          `json:"MaxThreads"`
}
var _ OperatorParallel = (*OpParallel)(nil) // Compile time assertion: type implements the interface

func NewOpParallel(operator OperatorUnary, maxThreads int64) *OpParallel {
	return &OpParallel{operator, maxThreads} 
}

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func (op *OpParallel) ApplyToFiles(sources []*OpLoadFile, logWriter io.Writer) (fOuts []*fits.Image, err error) {
	fOuts =make([]*fits.Image, len(sources))
	sem  :=make(chan bool, op.MaxThreads)
	res  :=make(chan error, len(sources))
	for i, src := range(sources) {
		sem <- true 
		go func(i int, source *OpLoadFile) {
			defer func() { <-sem }()
			f, err:=source.Apply(logWriter)
			if err!=nil {
				fOuts[i]=nil
				res <- err 
				return
			}
			f, err=op.Operator.Apply(f, logWriter)
			if err!=nil { 
				fOuts[i]=nil
				res <- err 
				return
			}
			fOuts[i]=f
			res <- nil
		}(i, src)
	}
	for i:=0; i<cap(sem); i++ {  // wait for goroutines to finish
		sem <- true
	}
	for i:=0; i<len(sources); i++ {
		r := <- res
		if r!=nil {
			if err==nil { 
				err = r
			} else {
				err = errors.New(fmt.Sprintf("Multiple errors: %s, %s", err.Error(), r.Error()))
			}
		}
	}
	return fOuts, err 
}


// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func (op *OpParallel) ApplyToFITS(sources []*fits.Image, logWriter io.Writer) (fOuts []*fits.Image, err error) {
	fOuts =make([]*fits.Image, len(sources))
	sem  :=make(chan bool, op.MaxThreads)
	res  :=make(chan error, len(sources))
	for i, src := range(sources) {
		sem <- true 
		go func(i int, f *fits.Image) {
			defer func() { <-sem }()
			f, err=op.Operator.Apply(f, logWriter)
			if err!=nil {
				fOuts[i]=nil
				res <- err 
				return
			}
			fOuts[i]=f
			res <- nil
		}(i, src)
	}
	for i:=0; i<cap(sem); i++ {  // wait for goroutines to finish
		sem <- true
	}
	for i:=0; i<len(sources); i++ {
		r := <- res
		if r!=nil {
			if err==nil { 
				err = r
			} else {
				err = errors.New(fmt.Sprintf("Multiple errors: %s, %s", err.Error(), r.Error()))
			}
		}
	}
	return fOuts, err
}
