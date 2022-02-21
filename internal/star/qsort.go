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


package star


import (
)

// Sort an array of stars in descending order, based on mass
// Array must not contain IEEE NaN
func QSortStarsDesc(a []Star) {
    if len(a)>1 {
        index := QPartitionStarsDesc(a)
        QSortStarsDesc(a[:index+1])
        QSortStarsDesc(a[index+1:])
    }
}


// Partitions an array of stars with the middle pivot element, and returns the pivot index.
// Values greater than the pivot are moved left of the pivot, those less are moved right.
// Array must not contain IEEE NaN
func QPartitionStarsDesc(a []Star) int {
    left, right:=0, len(a)-1
    mid   := (left+right)>>1
    pivot := a[mid].Mass
    l := left -1
    r := right+1
    for {
        for {
            l++
            if a[l].Mass<=pivot { break }
        }
        for {
            r--
            if a[r].Mass>=pivot { break }
        }
        if l >= r { return r }
        a[l], a[r] = a[r], a[l]
    }
}


