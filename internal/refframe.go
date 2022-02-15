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
)



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

