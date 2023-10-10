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

package fits

// Convert star color index (blue mag minus visual mag, range -0.4 ... +2.0) to RGB (0.0 ... 1.0).
// Interpolating the table from http://www.vendian.org/mncharity/dir3/starcolor/details.html
func bv2rgb(bv float32) RGB {
	if bv < -0.4 {
		bv = -0.4
	}
	if bv > 2.0 {
		bv = 2.0
	}

	index := (bv + 0.4) * 20
	floor := int(index)
	tFloor := bv2rgbTable[floor]

	ceil := floor + 1
	if ceil >= len(bv2rgbTable) {
		return RGB{tFloor.R, tFloor.G, tFloor.B}
	}

	tCeil := bv2rgbTable[ceil]
	fraction := index - float32(floor)

	r := tFloor.R*(1-fraction) + tCeil.R*fraction
	g := tFloor.G*(1-fraction) + tCeil.G*fraction
	b := tFloor.B*(1-fraction) + tCeil.B*fraction

	return RGB{r, g, b}
}

var bv2rgbTable = []RGB{
	{0.60784, 0.69804, 1.00000}, // -0.40
	{0.61961, 0.70980, 1.00000}, // -0.35
	{0.63922, 0.72549, 1.00000}, // -0.30
	{0.66667, 0.74902, 1.00000}, // -0.25
	{0.69804, 0.77255, 1.00000}, // -0.20
	{0.73333, 0.80000, 1.00000}, // -0.15
	{0.76863, 0.82353, 1.00000}, // -0.10
	{0.80000, 0.84706, 1.00000}, // -0.05
	{0.82745, 0.86667, 1.00000}, // 0.00
	{0.85490, 0.88627, 1.00000}, // 0.05
	{0.87451, 0.89804, 1.00000}, // 0.10
	{0.89412, 0.91373, 1.00000}, // 0.15
	{0.91373, 0.92549, 1.00000}, // 0.20
	{0.93333, 0.93725, 1.00000}, // 0.25
	{0.95294, 0.94902, 1.00000}, // 0.30
	{0.97255, 0.96471, 1.00000}, // 0.35
	{0.99608, 0.97647, 1.00000}, // 0.40
	{1.00000, 0.97647, 0.98431}, // 0.45
	{1.00000, 0.96863, 0.96078}, // 0.50
	{1.00000, 0.96078, 0.93725}, // 0.55
	{1.00000, 0.95294, 0.91765}, // 0.60
	{1.00000, 0.94510, 0.89804}, // 0.65
	{1.00000, 0.93725, 0.87843}, // 0.70
	{1.00000, 0.92941, 0.85882}, // 0.75
	{1.00000, 0.92157, 0.83922}, // 0.80
	{1.00000, 0.91373, 0.82353}, // 0.85
	{1.00000, 0.90980, 0.80784}, // 0.90
	{1.00000, 0.90196, 0.79216}, // 0.95
	{1.00000, 0.89804, 0.77647}, // 1.00
	{1.00000, 0.89020, 0.76471}, // 1.05
	{1.00000, 0.88627, 0.74902}, // 1.10
	{1.00000, 0.87843, 0.73333}, // 1.15
	{1.00000, 0.87451, 0.72157}, // 1.20
	{1.00000, 0.86667, 0.70588}, // 1.25
	{1.00000, 0.85882, 0.69020}, // 1.30
	{1.00000, 0.85490, 0.67843}, // 1.35
	{1.00000, 0.84706, 0.66275}, // 1.40
	{1.00000, 0.83922, 0.64706}, // 1.45
	{1.00000, 0.83529, 0.63137}, // 1.50
	{1.00000, 0.82353, 0.61176}, // 1.55
	{1.00000, 0.81569, 0.58824}, // 1.60
	{1.00000, 0.80000, 0.56078}, // 1.65
	{1.00000, 0.78431, 0.52157}, // 1.70
	{1.00000, 0.75686, 0.47059}, // 1.75
	{1.00000, 0.71765, 0.39608}, // 1.80
	{1.00000, 0.66275, 0.29412}, // 1.85
	{1.00000, 0.58431, 0.13725}, // 1.90
	{1.00000, 0.48235, 0.00000}, // 1.95
	{1.00000, 0.32157, 0.00000}, // 2.00
}
