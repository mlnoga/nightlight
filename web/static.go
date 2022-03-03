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

package web

import (
	"embed"
 	"io/fs"
 	"net/http"
)

//go:embed index.html
var IndexHTML []byte      // static web assets

//go:embed blockly
var blocklyFS embed.FS      // static web assets

func BlocklyFS() http.FileSystem {
    fsys, err := fs.Sub(blocklyFS, "blockly")
    if err != nil {
        panic(err)
    }
    return http.FS(fsys)
}

//go:embed js
var javascriptFS embed.FS      // static web assets

func JavascriptFS() http.FileSystem {
    fsys, err := fs.Sub(javascriptFS, "js")
    if err != nil {
        panic(err)
    }
    return http.FS(fsys)
}

