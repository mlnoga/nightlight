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


package internal


// Sort an array of float32 in ascending order.
// Array must not contain IEEE NaN
func QSortFloat32(a []float32) {
    if len(a)>1 {
        index := QPartitionFloat32(a)
        QSortFloat32(a[:index+1])
        QSortFloat32(a[index+1:])
    }
}


// Partitions an array of float32 with the middle pivot element, and returns the pivot index.
// Values less than the pivot are moved left of the pivot, those greater are moved right.
// Array must not contain IEEE NaN
func QPartitionFloat32(a []float32) int {
    left, right:=0, len(a)-1
    mid   := (left+right)>>1
    pivot := a[mid]
    l := left -1
    r := right+1
    for {
        for {
            l++
            if a[l]>=pivot { break }
        }
        for {
            r--
            if a[r]<=pivot { break }
        }
        if l >= r { return r }
        a[l], a[r] = a[r], a[l]
    }
}


// Select first quartile of an array of float32. Partially reorders the array.
// Array must not contain IEEE NaN
func QSelectFirstQuartileFloat32(a []float32) float32 {
    return QSelectFloat32(a, (len(a)>>2)+1)
}


// Select median of an array of float32. Partially reorders the array.
// Array must not contain IEEE NaN
func QSelectMedianFloat32(a []float32) float32 {
    return QSelectFloat32(a, (len(a)>>1)+1)
}


// Select kth lowest element from an array of float32. Partially reorders the array.
// Array must not contain IEEE NaN
func QSelectFloat32(a []float32, k int) float32 {
    left, right:=0, len(a)-1
    for left<right {
        // partition
        mid:=(left+right)>>1
        pivot := a[mid]
        l, r  := left-1, right+1
        for {
            for {
                l++
                if a[l]>=pivot { break }
            }
            for {
                r--
                if a[r]<=pivot { break }
            }
            if l >= r { break } // index in r
            a[l], a[r] = a[r], a[l]
        }
        index:=r

        offset:=index-left+1
        if k<=offset {
            right=index
        } else {
            left=index+1
            k=k-offset
        }
    }
    return a[left]
}


// Calculates the median of a float32 slice of length nine
// Modifies the elements in place
// From https://stackoverflow.com/questions/45453537/optimal-9-element-sorting-network-that-reduces-to-an-optimal-median-of-9-network
// See also http://ndevilla.free.fr/median/median/src/optmed.c for other sizes
// Array must not contain IEEE NaN
func MedianFloat32Slice9(a []float32) float32 {       // 30x min/max
    // function swap(i,j) {var tmp = MIN(a[i],a[j]); a[j] = MAX(a[i],a[j]); a[i] = tmp;}
    // function min(i,j) {a[i] = MIN(a[i],a[j]);}
    // function max(i,j) {a[j] = MAX(a[i],a[j]);}

    if a[0]>a[1] { a[0], a[1] = a[1], a[0]}  // swap(a,0,1)
    if a[3]>a[4] { a[3], a[4] = a[4], a[3]}  // swap(a,3,4)
    if a[6]>a[7] { a[6], a[7] = a[7], a[6]}  // swap(a,6,7)
    if a[1]>a[2] { a[1], a[2] = a[2], a[1]}  // swap(a,1,2)
    if a[4]>a[5] { a[4], a[5] = a[5], a[4]}  // swap(a,4,5)
    if a[7]>a[8] { a[7], a[8] = a[8], a[7]}  // swap(a,7,8)
    if a[0]>a[1] { a[0], a[1] = a[1], a[0]}  // swap(a,0,1)
    if a[3]>a[4] { a[3], a[4] = a[4], a[3]}  // swap(a,3,4)
    if a[6]>a[7] { a[6], a[7] = a[7], a[6]}  // swap(a,6,7)
    if a[0]>a[3] { a[3]       = a[0]      }  // max (a,0,3)
    if a[3]>a[6] { a[6]       = a[3]      }  // max (a,3,6)
    if a[1]>a[4] { a[1], a[4] = a[4], a[1]}  // swap(a,1,4)
    if a[4]>a[7] { a[4]       = a[7]      }  // min (a,4,7)
    if a[1]>a[4] { a[4]       = a[1]      }  // max (a,1,4)
    if a[5]>a[8] { a[5]       = a[8]      }  // min (a,5,8)
    if a[2]>a[5] { a[2]       = a[5]      }  // min (a,2,5)
    if a[2]>a[4] { a[2], a[4] = a[4], a[2]}  // swap(a,2,4)
    if a[4]>a[6] { a[4]       = a[6]      }  // min (a,4,6)
    if a[2]>a[4] { a[4]       = a[2]      }  // max (a,2,4)
    return a[4]
}



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


