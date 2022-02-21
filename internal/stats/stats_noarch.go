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

// +build !amd64

package stats


// Calculate minimum, mean and maximum of given data
func calcMinMeanMax(data []float32) (min, mean, max float32) {
	return calcMinMeanMaxPureGo(data)
}

// Calculate variance of given data from provided mean. 
func calcVariance(data []float32, mean float32) (result float64) {
	return calcVariancePureGo(data, mean)
}