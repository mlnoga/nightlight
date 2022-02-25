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


package rest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"syscall"
	"github.com/gin-gonic/gin"

	"github.com/mlnoga/nightlight/internal/ops"
	"github.com/mlnoga/nightlight/internal/stats"
)

// Secures the current process by creating a chroot environment
// (requires root) and changing the user ID to something without
// elevated rights.
func MakeSandbox(chroot string, setuid int) {
	if len(chroot)>0 {
		fmt.Printf("Changing filesystem root to %s...\n", chroot)
		if err:=syscall.Chroot(chroot); err!=nil {
			panic(fmt.Sprintf("error chroot(%s): %s\n", chroot, err.Error()))
		}
		if err:=os.Chdir(chroot); err!=nil {
			panic(fmt.Sprintf("error chdir(%s): %s\n", chroot, err.Error()))
		}
	}
	if setuid>=0 {
		fmt.Printf("Setting user id from %d/%d to  %d\n", syscall.Getuid(), syscall.Geteuid(), setuid)
		if err:=syscall.Setuid(setuid); err!=nil {
			panic(fmt.Sprintf("error setuid(%d): %s\n", setuid, err.Error()))
		}
	}
}

// Serve APIs and static files via HTTP
func Serve() {
	r := gin.Default()
	api := r.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			v1.GET ("/ping",    getPing)
			v1.POST("/job",     postJob)
			v1.StaticFS("/files",http.Dir(".") )
		}
	}
	r.Run() // listen and serve on 0.0.0.0:8080	
}

func getPing(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "pong",
	})
}

func printOp(logWriter io.Writer, prefix, suffix string, op interface{}) error {
	m,err:=json.MarshalIndent(op, "", "  ")
	if err!=nil { return err }
	fmt.Fprintf(logWriter, "%s%s%s", prefix, string(m), suffix)
	return nil
}

func postJob(c *gin.Context)  {
	{
		// bind POST arguments to a sequence operator
		var op ops.OpSequence
		if err:=c.ShouldBind(&op); err!=nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error() } )
			return
		}

		// prepare headers
		logWriter := c.Writer
		header := logWriter.Header()
		//header.Set("Transfer-Encoding", "chunked")
		header.Set("Content-Type", "text/plain")
		logWriter.WriteHeader(http.StatusOK)

		// play back arguments for debugging
		if err:=printOp(logWriter, "Arguments:\n", "\n", op); err!=nil {
			fmt.Fprintf(logWriter, "Error printing arguments: %s\n", err.Error())
			return
		}

		// create promises for the given command sequence
		oc:=ops.NewContext(logWriter, stats.LSESCMedianQn)
		promises, err:=op.MakePromises(nil, oc)
		if err!=nil { 
			fmt.Fprintf(logWriter, "Error making promises: %s\n", err.Error())
			return
		}

		// materialize all promises for their side effects, and forget the values 
		_, err=ops.MaterializeAll(promises, oc.MaxThreads, true)
		if err!=nil { 
			fmt.Fprintf(logWriter, "Error materializing promises: %s\n", err.Error())
			return
		}
		logWriter.(http.Flusher).Flush()
	}
	debug.FreeOSMemory()	

	return
}

