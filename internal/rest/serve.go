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
	"github.com/gin-gonic/gin"
	"net/http"

	nl "github.com/mlnoga/nightlight/internal"
	"github.com/mlnoga/nightlight/internal/state"
)


func Serve() {
	r := gin.Default()
	api := r.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			v1.GET ("/ping",  getPing)
			v1.GET ("/dark",  getDark)
			v1.POST("/dark",  postDark)
			v1.GET ("/flat",  getFlat)
			v1.POST("/flat",  postFlat)
			v1.GET ("/align", getAlign)
			v1.POST("/align", postAlign)
		}
	}
	r.Run() // listen and serve on 0.0.0.0:8080	
}


func getPing(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "pong",
	})
}


type postDarkFlatAlignArgs struct {
	Name string `json:"name" form:"name" binding:"required"`
}

func postDarkFlatAlign(c *gin.Context) *nl.FITSImage {
	var a postDarkFlatAlignArgs
	if err:=c.ShouldBind(&a); err!=nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error() } )
		return nil
	}

	f,err:=nl.LoadAndCalcStats(a.Name, -1)
	if err!=nil { 
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error() } ) 
		return nil
	}

	res:=gin.H{
	 	"id": f.ID, 
	    "name": f.FileName,
	    "width": f.Naxisn[0],
	    "height": f.Naxisn[1],
	    "stats" : f.Stats,
	}
	if f.Stats.StdDev<1e-8 {
		res["warning"]="File may be degenerate"
	}
	c.JSON(http.StatusCreated, res)
	return f
}

func postDark(c *gin.Context) {
	state.DarkF=postDarkFlatAlign(c)
}

func postFlat(c *gin.Context) {
	state.FlatF=postDarkFlatAlign(c)
}

func postAlign(c *gin.Context) {
	state.AlignF=postDarkFlatAlign(c)
}



func getDarkFlatAlign(c *gin.Context, f *nl.FITSImage) {
	if f==nil {
		c.JSON(http.StatusNotFound, gin.H{} ) 
		return
	}

	res:=gin.H{
	 	"id": f.ID, 
	    "name": f.FileName,
	    "width": f.Naxisn[0],
	    "height": f.Naxisn[1],
	    "stats" : f.Stats,
	}
	if f.Stats.StdDev<1e-8 {
		res["warning"]="File may be degenerate"
	}
	c.JSON(http.StatusOK, res)
}

func getDark(c *gin.Context) {
	getDarkFlatAlign(c, state.DarkF)
}

func getFlat(c *gin.Context) {
	getDarkFlatAlign(c, state.FlatF)
}

func getAlign(c *gin.Context) {
	getDarkFlatAlign(c, state.AlignF)
}
