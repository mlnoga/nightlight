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


package stack

import (
	"github.com/mlnoga/nightlight/internal/ops"
	"github.com/mlnoga/nightlight/internal/ops/pre"
	"github.com/mlnoga/nightlight/internal/ops/ref"
)


func NewOpStackBatch(opPreProc *ops.OpSequence, opSelectReference *ref.OpSelectReference, 
	                 opPostProc *ops.OpSequence, opStack *OpStack, opStarDetect *pre.OpStarDetect, 
	                 opSave *ops.OpSave) *ops.OpSequence {
	return ops.NewOpSequence(
		opPreProc, opSelectReference, opPostProc, opStack, opStarDetect, opSave,
	)
}
