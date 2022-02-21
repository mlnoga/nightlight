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
	"runtime"
	"github.com/gin-gonic/gin"

	"github.com/mlnoga/nightlight/internal/ops"
	"github.com/mlnoga/nightlight/internal/ops/pre"
	"github.com/mlnoga/nightlight/internal/ops/stack"	
	"github.com/mlnoga/nightlight/internal/ops/stretch"
	"github.com/mlnoga/nightlight/internal/ops/rgb"
)


func Serve() {
	r := gin.Default()
	api := r.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			v1.GET ("/ping",    getPing)
			v1.POST("/stats",   postStats)
			v1.POST("/stack",   postStack)
			v1.POST("/stretch", postStretch)
			v1.POST("/rgbl",    postRGBL)
		}
	}
	r.Run() // listen and serve on 0.0.0.0:8080	
}

func getPing(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "pong",
	})
}

func printArgs(logWriter io.Writer, prefix, suffix string, args interface{}) error {
	m,err:=json.MarshalIndent(args, "", "  ")
	if err!=nil { return err }
	fmt.Fprintf(logWriter, "%s%s%s", prefix, string(m), suffix)
	return nil
}

type postStatsArgs struct {
	FilePatterns []string             `json:"filePatterns"`
	StarDetect    *pre.OpStarDetect    `json:"starDetect"`
}

func postStats(c *gin.Context)  {
	logWriter := c.Writer
	var args postStatsArgs
	if err:=c.ShouldBind(&args); err!=nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error() } )
		return
	}

	header := logWriter.Header()
	//header.Set("Transfer-Encoding", "chunked")
	header.Set("Content-Type", "text/plain")
	logWriter.WriteHeader(http.StatusOK)

	if err:=printArgs(logWriter, "Arguments:\n", "\n", args); err!=nil {
		fmt.Fprintf(logWriter, "Error printing arguments: %s\n", err.Error())
		return
	}

	// glob filename arguments into OpLoadFiles operators
	var err error
	opLoadFiles, err:=ops.NewOpLoadFiles(args.FilePatterns, logWriter)
	if err!=nil {
		fmt.Fprintf(logWriter, "Error globbing filenames: %s\n", err.Error())
		return
	}

	opParallel:=ops.NewOpParallel(args.StarDetect, int64(runtime.NumCPU()))
	_, err=opParallel.ApplyToFiles(opLoadFiles, logWriter)
	if(err!=nil) {
		fmt.Fprintf(logWriter, "error: %s\n", err.Error())		
	}
	logWriter.(http.Flusher).Flush()

	return
}


type postStackArgs struct {
	FilePatterns    []string                `json:"filePatterns"`
	StackMultiBatch  *stack.OpStackMultiBatch  `json:"stackMultiBatch"`	
}

func postStack(c *gin.Context) {
	logWriter := c.Writer
	var args postStackArgs
	if err:=c.ShouldBind(&args); err!=nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error() } )
		return
	}

	header := logWriter.Header()
	//header.Set("Transfer-Encoding", "chunked")
	header.Set("Content-Type", "text/plain")
	logWriter.WriteHeader(http.StatusOK)

	if err:=printArgs(logWriter, "Arguments:\n", "\n", args); err!=nil {
		fmt.Fprintf(logWriter, "Error printing arguments: %s\n", err.Error())
		return
	}

	// glob filename arguments into OpLoadFiles operators
	var err error
	opLoadFiles, err:=ops.NewOpLoadFiles(args.FilePatterns, logWriter)
	if err!=nil {
		fmt.Fprintf(logWriter, "Error globbing filenames: %s\n", err.Error())
		return
	}

	_, err=args.StackMultiBatch.Apply(opLoadFiles, logWriter)	
	if(err!=nil) {
		fmt.Fprintf(logWriter, "error: %s\n", err.Error())	
	}
	logWriter.(http.Flusher).Flush()

	return
}


type postStretchArgs struct {
	FilePatterns []string        `json:"filePatterns"`
	Stretch       *stretch.OpStretch  `json:"stretch"`	
}

func postStretch(c *gin.Context) {
	logWriter := c.Writer
	var args postStretchArgs
	if err:=c.ShouldBind(&args); err!=nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error() } )
		return
	}

	header := logWriter.Header()
	//header.Set("Transfer-Encoding", "chunked")
	header.Set("Content-Type", "text/plain")
	logWriter.WriteHeader(http.StatusOK)

	if err:=printArgs(logWriter, "Arguments:\n", "\n", args); err!=nil {
		fmt.Fprintf(logWriter, "Error printing arguments: %s\n", err.Error())
		return
	}

	// glob filename arguments into OpLoadFiles operators
	var err error
	opLoadFiles, err:=ops.NewOpLoadFiles(args.FilePatterns, logWriter)
	if err!=nil {
		fmt.Fprintf(logWriter, "Error globbing filenames: %s\n", err.Error())
		return
	}

   	opParallel:=ops.NewOpParallel(args.Stretch, int64(runtime.GOMAXPROCS(0)))
	_, err=opParallel.ApplyToFiles(opLoadFiles, logWriter)	
	if(err!=nil) {
		fmt.Fprintf(logWriter, "error: %s\n", err.Error())	
	}
	logWriter.(http.Flusher).Flush()

	return
}



type postRGBLArgs struct {
	FilePatterns []string            `json:"filePatterns"`
	RGBLProcess   *rgb.OpRGBLProcess  `json:"rgblProcess"`	
}

func postRGBL(c *gin.Context) {
	logWriter := c.Writer
	var args postRGBLArgs
	if err:=c.ShouldBind(&args); err!=nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error() } )
		return
	}

	header := logWriter.Header()
	//header.Set("Transfer-Encoding", "chunked")
	header.Set("Content-Type", "text/plain")
	logWriter.WriteHeader(http.StatusOK)

	if err:=printArgs(logWriter, "Arguments:\n", "\n", args); err!=nil {
		fmt.Fprintf(logWriter, "Error printing arguments: %s\n", err.Error())
		return
	}

	// glob filename arguments into OpLoadFiles operators
	var err error
	opLoadFiles, err:=ops.NewOpLoadFiles(args.FilePatterns, logWriter)
	if err!=nil {
		fmt.Fprintf(logWriter, "Error globbing filenames: %s\n", err.Error())
		return
	}

	_, err=args.RGBLProcess.Apply(opLoadFiles, logWriter)	
	if(err!=nil) {
		fmt.Fprintf(logWriter, "error: %s\n", err.Error())	
	}
	logWriter.(http.Flusher).Flush()

	return
}
