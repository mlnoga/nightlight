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

package ref

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/ops"
)

type OpExportStats struct {
	ops.OpUnaryBase
	FileName     string        `json:"fileName"`
	mutex        sync.Mutex    `json:"-"`
	materialized []*fits.Image `json:"-"`
	opError      error         `json:"-"`
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpExportStatsDefault() }) } // register the operator for JSON decoding

func NewOpExportStatsDefault() *OpExportStats { return NewOpExportStats("out.html") }

func NewOpExportStats(fileName string) *OpExportStats {
	op := &OpExportStats{
		OpUnaryBase: ops.OpUnaryBase{OpBase: ops.OpBase{Type: "exportStats"}},
		FileName:    fileName,
	}
	op.OpUnaryBase.Apply = op.Apply // assign class method to superclass abstract method
	return op
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpExportStats) UnmarshalJSON(data []byte) error {
	type defaults OpExportStats
	def := defaults(*NewOpExportStatsDefault())
	err := json.Unmarshal(data, &def)
	if err != nil {
		return err
	}
	*op = OpExportStats(def)
	op.OpUnaryBase.Apply = op.Apply // make method receiver point to op, not def
	return nil
}

func (op *OpExportStats) Apply(f *fits.Image, c *ops.Context) (result *fits.Image, err error) {
	if op.FileName == "" {
		fmt.Fprintf(c.Log, "%d: exportStats empty fileName\n", f.ID)
		return f, nil
	}

	op.mutex.Lock()         // lock so a single thread is active
	defer op.mutex.Unlock() // always release lock on exit

	// write stats
	if c.StatsProcessed == 0 {
		err = op.writeHeader(c)
		if err != nil {
			return nil, err
		}
	}
	op.writeStats(f, c)
	c.StatsProcessed++
	if c.StatsProcessed == c.StatsTotal {
		op.writeFooter(c)
	}

	return f, nil
}

func (op *OpExportStats) writeHeader(c *ops.Context) (err error) {
	fmt.Fprintf(c.Log, "Writing statistics header to file %s ...\n", op.FileName)
	c.StatsFile, err = os.Create(op.FileName)
	if err != nil {
		return fmt.Errorf("error creating file %s: %s", op.FileName, err.Error())
	}
	c.StatsBufWriter = bufio.NewWriter(c.StatsFile)

	c.StatsBufWriter.WriteString(sessionStatsHeader)
	fmt.Fprintf(c.StatsBufWriter, "[  ['ID','Min','Mean','Max','Location','Scale','Stars','HFR']\n")

	return nil
}

func (op *OpExportStats) writeStats(f *fits.Image, c *ops.Context) {
	fmt.Fprintf(c.Log, "%d: writing statistics to file %s ...\n", f.ID, op.FileName)
	s := f.Stats
	fmt.Fprintf(c.StatsBufWriter, "  ,[%d,%f,%f,%f,%f,%f,%d,%f]\n",
		f.ID, s.Min(), s.Mean(), s.Max(), s.Location(), s.Scale(), len(f.Stars), f.HFR)
}

func (op *OpExportStats) writeFooter(c *ops.Context) {
	fmt.Fprintf(c.Log, "Writing statistics footer to file %s ...\n", op.FileName)
	fmt.Fprintf(c.StatsBufWriter, "]")
	c.StatsBufWriter.WriteString(sessionStatsTrailer)
	c.StatsBufWriter.Flush()
	c.StatsBufWriter = nil
	c.StatsFile.Close()
	c.StatsFile = nil
}

const sessionStatsHeader = `<html>
  <head>
    <script type="text/javascript" src="https://www.gstatic.com/charts/loader.js"></script>
  </head>
  <body>
    <table height="100%" width="100%"><tr height="100%">
      <td width="90%"><div id="sessionStatsChart" style="width: 100%; height: 100%"></div></td>
      <td width="10%"><form><input type="checkbox" id="normalize" name="normalize" checked="true" onchange="toggleNormalize()"><label for="normalize">Normalize</label></form></td>
    </tr></table>
  </body>
  <script type="text/javascript">
google.charts.load('current', {'packages':['corechart']});
google.charts.setOnLoadCallback(drawChart);

var dataArray =
`

const sessionStatsTrailer = `;

function sortByFirstElement(a, b) {
	return a[0] - b[0];
}
dataHeader=dataArray[0];
dataRows=dataArray.slice(1);
dataRows.sort(sortByFirstElement);
dataArray = [dataHeader].concat(dataRows);

var columnMedians=calcColumnMedians(dataArray);

var normDataArray=normalizeYAxisValues(dataArray, columnMedians);

var normalizeCheckbox=document.getElementById('normalize');

function getData() {
  return normalizeCheckbox.checked ? normDataArray : dataArray;
}

var options = {
  title: 'Session statistics',
  // curveType: 'function', // smooth curves
  explorer: {
    axis: 'horizontal',
    action: ['dragToPan'],
    keepInBounds: true,
    maxZoomIn: 0.001,
    maxZoomOut: 1.0
  },
  crosshair: { trigger: 'both' }, // Display crosshairs on focus and selection
  legend: { position: 'bottom' }
};

var chart;

function toggleNormalize() {
  data = google.visualization.arrayToDataTable(getData())
  chart.draw(data, options);
}

function drawChart() {
  chart = new google.visualization.LineChart(document.getElementById('sessionStatsChart'));
  toggleNormalize();
}

function calcColumnMedians(d) {
  var numRows=d.length-1;
  var buffer=new Array(numRows);
  var numColumns=d[0].length;
  var medians=new Array(numColumns);

  for(let col=0; col<numColumns; col++) {
    for(let row=1; row<=numRows; row++) {
      buffer[row]=d[row][col];
    }
    medians[col]=median(buffer);
  }

  return medians;
}

function normalizeYAxisValues(d, m) {
  var numRows=d.length-1;
  var numColumns=d[0].length;

  var norm=new Array(numRows);
  norm[0]=d[0]; // header
  for(let r=1; r<=numRows; r++) {
    thisRow=new Array(numColumns);
    thisRow[0]=d[r][0]; // x axis values, don't normalize
    for(let c=1; c<numColumns; c++) {
      thisRow[c]=d[r][c] / m[c];
    }
    norm[r]=thisRow;
  }
  return norm;
}

function median(numbers) {
    const sorted = numbers.slice().sort((a, b) => a - b);
    const middle = Math.floor(sorted.length / 2);
    if (sorted.length % 2 === 0) {
        return (sorted[middle - 1] + sorted[middle]) / 2;
    }
    return sorted[middle];
}

  </script>
</html>
`
