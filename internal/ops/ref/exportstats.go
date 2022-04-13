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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"github.com/mlnoga/nightlight/internal/fits"
	"github.com/mlnoga/nightlight/internal/ops"
)



type OpExportStats struct {
	ops.OpBase
    FileName        string             `json:"fileName"`
		mutex           sync.Mutex         `json:"-"`
		materialized    []*fits.Image      `json:"-"`
		opError         error              `json:"-"`          
}

func init() { ops.SetOperatorFactory(func() ops.Operator { return NewOpExportStatsDefault() })} // register the operator for JSON decoding

func NewOpExportStatsDefault() *OpExportStats { return NewOpExportStats("out.html") }

func NewOpExportStats(fileName string) *OpExportStats {  
	op:=&OpExportStats{
	  	OpBase   : ops.OpBase{Type: "exportStats"},
	    FileName : fileName,
	}
	return op	
}

// Unmarshal the type from JSON with default values for missing entries
func (op *OpExportStats) UnmarshalJSON(data []byte) error {
	type defaults OpExportStats
	def:=defaults( *NewOpExportStatsDefault() )
	err:=json.Unmarshal(data, &def)
	if err!=nil { return err }
	*op=OpExportStats(def)
	return nil
}

// Selects a reference for all given input promises using the specified mode.
// This creates separate output promises for each input promise.
// The first of them to acquire the reference mutex evaluates all images
func (op *OpExportStats) MakePromises(ins []ops.Promise, c *ops.Context) (outs []ops.Promise, err error) {
	if len(ins)==0 { return nil, errors.New(fmt.Sprintf("%s operator needs inputs", op.Type)) }

	outs=make([]ops.Promise, len(ins))
	for i,_:=range(ins) {
		outs[i]=op.makePromise(i, ins, c)
	}
	return outs, nil
}

func (op *OpExportStats) makePromise(i int, ins []ops.Promise, c *ops.Context) ops.Promise {
	return func() (f *fits.Image, err error) {
		if op.FileName=="" { return ins[i]() }

		op.mutex.Lock()                 // lock so a single thread is active
		if op.opError!=nil {            // if failed in a prior thread
			op.mutex.Unlock()             // return immediately with the same error
			return nil, errors.New("same error") 
		}
		if op.materialized!=nil {
			op.mutex.Unlock()           // unlock immediately to allow ...
			if op.materialized[i]==nil {
				return ins[i]()           // ... materializations to be parallelized by the caller
			} else {
				mat:=op.materialized[i]
				op.materialized[i]=nil    // remove reference to free memory
				return mat, nil
			}
		}
		defer op.mutex.Unlock()		    // else release lock later when reference frame is computed

		// otherwise, materialize the input promises
		op.materialized,op.opError=ops.MaterializeAll(ins, c.MaxThreads, false)
		if op.opError!=nil { return nil, op.opError }

		// write stats
		err=op.writeStats(c)

		// return promise for the materialized image of this instance
		mat:=op.materialized[i]
		op.materialized[i]=nil // remove reference
		return mat, err
	}
}


func (op *OpExportStats) writeStats(c *ops.Context) (err error) {
	fmt.Fprintf(c.Log, "Writing statistics to file %s ...\n", op.FileName)
	f,err :=os.Create(op.FileName)
	if err!=nil { return errors.New(fmt.Sprintf("error creating file %s: %s", op.FileName, err.Error())) }
	defer f.Close()

	f.WriteString(sessionStatsHeader)
	fmt.Fprintf(f, "[ ['ID','Min','Mean','Max','Location','Scale','Stars','HFR'],\n")

	for id,mat:=range(op.materialized) {
		s:=mat.Stats
		separator:=","
		if id==len(op.materialized)-1 { separator="" }
		fmt.Fprintf(f, "  [%d,%f,%f,%f,%f,%f,%d,%f]%s\n", 
			          mat.ID, s.Min(), s.Mean(), s.Max(), s.Location(), s.Scale(), len(mat.Stars), mat.HFR, separator)
	}

	fmt.Fprintf(f,"]")
	f.WriteString(sessionStatsTrailer)

	return nil
}


const sessionStatsHeader=`<html>
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
`;


const sessionStatsTrailer=`;

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
