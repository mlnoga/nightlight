// +build windows

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
	"fmt"
)


// Secures the current process by creating a chroot environment
// (requires root) and changing the user ID to something without
// elevated rights.
func MakeSandbox(chroot string, setuid int) {
	if len(chroot)>0 {
		fmt.Printf("Warning: Ignoring chroot argument %s on Windows...\n", chroot)
	}
	if setuid>=0 {
		fmt.Printf("Warning: Ignoring setuid argument %d on Windows...\n", setuid)
	}
}
