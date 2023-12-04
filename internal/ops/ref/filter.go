package ref

import (
	"encoding/json"
	"fmt"

	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/ops"
)

// Filter out images with too few stars
type OpFilter struct {
	ops.OpUnaryBase
	MinStars int `json:"minStars"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpFilterDefault() }) } // register the operator for JSON decoding

func NewOpFilterDefault() *OpFilter { return NewOpFilter(0) }

// Preprocess all light frames with given global settings, limiting concurrency to the number of available CPUs
func NewOpFilter(minStars int) *OpFilter {
	op := OpFilter{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "filter"}},
		MinStars:    minStars,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return &op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpFilter) UnmarshalJSON(data []byte) error {
	type defaults OpFilter
	def := defaults(*NewOpFilterDefault())
	if err := json.Unmarshal(data, &def); err != nil {
		return err
	}
	*op = OpFilter(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpFilter) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.MinStars <= 0 {
		return f, nil
	}

	if f.Stars == nil || len(f.Stars) < op.MinStars {
		fmt.Fprintf(c.Log, "%d: Stars=%d below threshold %d, skipping frame\n", f.ID, len(f.Stars), op.MinStars)
		return nil, nil
	}
	return f, nil
}
