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

import (
	"runtime"
	"sync"
)

// Don´t you wish for generic types in golang? Sigh.


// Pool of constant sized arrays of given type, to reduce memory allocation overhead
var poolByte   =struct{
    sync.RWMutex
    m map[int]*sync.Pool
}{m: make(map[int]*sync.Pool)}

// Pool of constant sized arrays of given type, to reduce memory allocation overhead
var poolInt8   =struct{
    sync.RWMutex
    m map[int]*sync.Pool
}{m: make(map[int]*sync.Pool)}

// Pool of constant sized arrays of given type, to reduce memory allocation overhead
var poolInt16  =struct{
    sync.RWMutex
    m map[int]*sync.Pool
}{m: make(map[int]*sync.Pool)}

// Pool of constant sized arrays of given type, to reduce memory allocation overhead
var poolInt32  =struct{
    sync.RWMutex
    m map[int]*sync.Pool
}{m: make(map[int]*sync.Pool)}

// Pool of constant sized arrays of given type, to reduce memory allocation overhead
var poolInt64  =struct{
    sync.RWMutex
    m map[int]*sync.Pool
}{m: make(map[int]*sync.Pool)}

// Pool of constant sized arrays of given type, to reduce memory allocation overhead
var poolFloat32=struct{
    sync.RWMutex
    m map[int]*sync.Pool
}{m: make(map[int]*sync.Pool)}

// Pool of constant sized arrays of given type, to reduce memory allocation overhead
var poolFloat64=struct{
    sync.RWMutex
    m map[int]*sync.Pool
}{m: make(map[int]*sync.Pool)}

// Pool of constant sized arrays of given type, to reduce memory allocation overhead
var poolStar=struct{
    sync.RWMutex
    m map[int]*sync.Pool
}{m: make(map[int]*sync.Pool)}

// Pool of constant sized arrays of given type, to reduce memory allocation overhead
var poolStarListItem=struct{
    sync.RWMutex
    m map[int]*sync.Pool
}{m: make(map[int]*sync.Pool)}

// Pool of constant sized arrays of given type, to reduce memory allocation overhead
var poolPointerToStarListItem=struct{
    sync.RWMutex
    m map[int]*sync.Pool
}{m: make(map[int]*sync.Pool)}


// Clears all memory pools and triggers garbage collection
func ClearPools() {
	poolByte   =struct{
	    sync.RWMutex
	    m map[int]*sync.Pool
	}{m: make(map[int]*sync.Pool)}

	poolInt8   =struct{
	    sync.RWMutex
	    m map[int]*sync.Pool
	}{m: make(map[int]*sync.Pool)}

	poolInt16  =struct{
	    sync.RWMutex
	    m map[int]*sync.Pool
	}{m: make(map[int]*sync.Pool)}

	poolInt32  =struct{
	    sync.RWMutex
	    m map[int]*sync.Pool
	}{m: make(map[int]*sync.Pool)}

	poolInt64  =struct{
	    sync.RWMutex
	    m map[int]*sync.Pool
	}{m: make(map[int]*sync.Pool)}

	poolFloat32=struct{
	    sync.RWMutex
	    m map[int]*sync.Pool
	}{m: make(map[int]*sync.Pool)}

	poolFloat64=struct{
	    sync.RWMutex
	    m map[int]*sync.Pool
	}{m: make(map[int]*sync.Pool)}

	poolStar=struct{
	    sync.RWMutex
	    m map[int]*sync.Pool
	}{m: make(map[int]*sync.Pool)}

	poolStarListItem=struct{
	    sync.RWMutex
	    m map[int]*sync.Pool
	}{m: make(map[int]*sync.Pool)}

	poolPointerToStarListItem=struct{
	    sync.RWMutex
	    m map[int]*sync.Pool
	}{m: make(map[int]*sync.Pool)}

	runtime.GC()
}


// Returns a pool for byte arrays of the given size
func getSizedPoolByte(size int) *sync.Pool {
	poolByte.RLock()
	pool:=poolByte.m[size]
	poolByte.RUnlock()
	if pool==nil {
		pool=&sync.Pool{
			New: func() interface{} {
				return make([]byte, size);
			},
		}
		poolByte.Lock()
		poolByte.m[size]=pool
		poolByte.Unlock()
	}
	return pool
}

// Retrieves an array of given size and type from pool
func GetArrayOfByteFromPool(size int) []byte {
	pool:=getSizedPoolByte(size)
	return pool.Get().([]byte)
}

// Returns an array of given size and type to the pool
func PutArrayOfByteIntoPool(arr []byte) {
	pool:=getSizedPoolByte(cap(arr))
	pool.Put(arr[:cap(arr)])
	arr=nil
}


// Returns a pool for []int8 arrays of the given size
func getSizedPoolInt8(size int) *sync.Pool {
	poolInt8.RLock()
	pool:=poolInt8.m[size]
	poolInt8.RUnlock()
	if pool==nil {
		pool=&sync.Pool{
			New: func() interface{} {
				return make([]int8, size);
			},
		}
		poolInt8.Lock()
		poolInt8.m[size]=pool
		poolInt8.Unlock()
	}
	return pool
}

// Retrieves an array of given size and type from pool
func GetArrayOfInt8FromPool(size int) []int8 {
	pool:=getSizedPoolInt8(size)
	return pool.Get().([]int8)
}

// Returns an array of given size and type to the pool
func PutArrayOfInt8IntoPool(arr []int8) {
	pool:=getSizedPoolInt8(cap(arr))
	pool.Put(arr[:cap(arr)])
	arr=nil
}


// Returns a pool for []int16 arrays of the given size
func getSizedPoolInt16(size int) *sync.Pool {
	poolInt16.RLock()
	pool:=poolInt16.m[size]
	poolInt16.RUnlock()
	if pool==nil {
		pool=&sync.Pool{
			New: func() interface{} {
				return make([]int16, size);
			},
		}
		poolInt16.Lock()
		poolInt16.m[size]=pool
		poolInt16.Unlock()
	}
	return pool
}

// Retrieves an array of given size and type from pool
func GetArrayOfInt16FromPool(size int) []int16 {
	pool:=getSizedPoolInt16(size)
	return pool.Get().([]int16)
}

// Returns an array of given size and type to the pool
func PutArrayOfInt16IntoPool(arr []int16)  {
	pool:=getSizedPoolInt16(cap(arr))
	pool.Put(arr[:cap(arr)])
	arr=nil
}


// Returns a pool for []int32 arrays of the given size
func getSizedPoolInt32(size int) *sync.Pool {
	poolInt32.RLock()
	pool:=poolInt32.m[size]
	poolInt32.RUnlock()
	if pool==nil {
		pool=&sync.Pool{
			New: func() interface{} {
				return make([]int32, size);
			},
		}
		poolInt32.Lock()
		poolInt32.m[size]=pool
		poolInt32.Unlock()
	}
	return pool
}

// Retrieves an array of given size and type from pool
func GetArrayOfInt32FromPool(size int) []int32 {
	pool:=getSizedPoolInt32(size)
	return pool.Get().([]int32)
}

// Returns an array of given size and type to the pool
func PutArrayOfInt32IntoPool(arr []int32) {
	pool:=getSizedPoolInt32(cap(arr))
	pool.Put(arr[:cap(arr)])
	arr=nil
}


// Returns a pool for []int64 arrays of the given size
func getSizedPoolInt64(size int) *sync.Pool {
	poolInt64.RLock()
	pool:=poolInt64.m[size]
	poolInt64.RUnlock()
	if pool==nil {
		pool=&sync.Pool{
			New: func() interface{} {
				return make([]int64, size);
			},
		}
		poolInt64.Lock()
		poolInt64.m[size]=pool
		poolInt64.Unlock()
	}
	return pool
}

// Retrieves an array of given size and type from pool
func GetArrayOfInt64FromPool(size int) []int64 {
	pool:=getSizedPoolInt64(size)
	return pool.Get().([]int64)
}

// Returns an array of given size and type to the pool
func PutArrayOfInt64IntoPool(arr []int64) {
	pool:=getSizedPoolInt64(cap(arr))
	pool.Put(arr[:cap(arr)])
	arr=nil
}



// Returns a pool for []float32 arrays of the given size
func getSizedPoolFloat32(size int) *sync.Pool {
	poolFloat32.RLock()
	pool:=poolFloat32.m[size]
	poolFloat32.RUnlock()
	if pool==nil {
		pool=&sync.Pool{
			New: func() interface{} {
				res:=make([]float32, size);
				m:=runtime.MemStats{}
				runtime.ReadMemStats(&m)
				if size>10000000 {
					LogPrintf("make %d %d alloc %d totalAlloc %d sys %d (all MiB)\n", size, (size*4)/1024/1024, m.Alloc/1024/1024, m.TotalAlloc/1024/1024, m.Sys/1024/1024)
				}
				return res
			},
		}
		poolFloat32.Lock()
		poolFloat32.m[size]=pool
		poolFloat32.Unlock()
	}
	return pool
}

// Retrieves an array of given size and type from pool
func GetArrayOfFloat32FromPool(size int) []float32 {
	pool:=getSizedPoolFloat32(size)
	res:=pool.Get().([]float32)
	m:=runtime.MemStats{}
	runtime.ReadMemStats(&m)
	if size>10000000 {
		LogPrintf("get  %d %d alloc %d totalAlloc %d sys %d (all MiB)\n", size, (size*4)/1024/1024, m.Alloc/1024/1024, m.TotalAlloc/1024/1024, m.Sys/1024/1024)
	}
	return res
}

// Returns an array of given size and type to the pool
func PutArrayOfFloat32IntoPool(arr []float32) {
	pool:=getSizedPoolFloat32(cap(arr))
	pool.Put(arr[:cap(arr)])
	m:=runtime.MemStats{}
	runtime.ReadMemStats(&m)
	if len(arr)>10000000 {
		LogPrintf("put  %d %d alloc %d totalAlloc %d sys %d (all MiB)\n", len(arr), (len(arr)*4)/1024/1024, m.Alloc/1024/1024, m.TotalAlloc/1024/1024, m.Sys/1024/1024)
	}
	arr=nil
}


// Returns a pool for []float64 arrays of the given size
func getSizedPoolFloat64(size int) *sync.Pool {
	poolFloat64.RLock()
	pool:=poolFloat64.m[size]
	poolFloat64.RUnlock()
	if pool==nil {
		pool=&sync.Pool{
			New: func() interface{} {
				return make([]float64, size);
			},
		}
		poolFloat64.Lock()
		poolFloat64.m[size]=pool
		poolFloat64.Unlock()
	}
	return pool
}

// Retrieves an array of given size and type from pool
func GetArrayOfFloat64FromPool(size int) []float64 {
	pool:=getSizedPoolFloat64(size)
	return pool.Get().([]float64)
}

// Returns an array of given size and type to the pool
func PutArrayOfFloat64IntoPool(arr []float64) {
	pool:=getSizedPoolFloat64(cap(arr))
	pool.Put(arr[:cap(arr)])
	arr=nil
}


// Returns a pool for []Star arrays of the given size
func getSizedPoolStar(size int) *sync.Pool {
	poolStar.RLock()
	pool:=poolStar.m[size]
	poolStar.RUnlock()
	if pool==nil {
		pool=&sync.Pool{
			New: func() interface{} {
				return make([]Star, size);
			},
		}
		poolStar.Lock()
		poolStar.m[size]=pool
		poolStar.Unlock()
	}
	return pool
}

// Retrieves an array of given size and type from pool
func GetArrayOfStarFromPool(size int) []Star {
	pool:=getSizedPoolStar(size)
	return pool.Get().([]Star)
}

// Returns an array of given size and type to the pool
func PutArrayOfStarIntoPool(arr []Star) {
	pool:=getSizedPoolStar(cap(arr))
	pool.Put(arr[:cap(arr)])
	arr=nil
}


// Returns a pool for []Star arrays of the given size
func getSizedPoolStarListItem(size int) *sync.Pool {
	poolStarListItem.RLock()
	pool:=poolStarListItem.m[size]
	poolStarListItem.RUnlock()
	if pool==nil {
		pool=&sync.Pool{
			New: func() interface{} {
				return make([]starListItem, size);
			},
		}
		poolStarListItem.Lock()
		poolStarListItem.m[size]=pool
		poolStarListItem.Unlock()
	}
	return pool
}

// Retrieves an array of given size and type from pool
func GetArrayOfStarListItemFromPool(size int) []starListItem {
	pool:=getSizedPoolStarListItem(size)
	return pool.Get().([]starListItem)
}

// Returns an array of given size and type to the pool
func PutArrayOfStarListItemIntoPool(arr []starListItem) {
	pool:=getSizedPoolStarListItem(cap(arr))
	pool.Put(arr[:cap(arr)])
	arr=nil
}


// Returns a pool for []Star arrays of the given size
func getSizedPoolPointerToStarListItem(size int) *sync.Pool {
	poolPointerToStarListItem.RLock()
	pool:=poolPointerToStarListItem.m[size]
	poolPointerToStarListItem.RUnlock()
	if pool==nil {
		pool=&sync.Pool{
			New: func() interface{} {
				return make([]*starListItem, size);
			},
		}
		poolPointerToStarListItem.Lock()
		poolPointerToStarListItem.m[size]=pool
		poolPointerToStarListItem.Unlock()
	}
	return pool
}

// Retrieves an array of given size and type from pool
func GetArrayOfPointerToStarListItemFromPool(size int) []*starListItem {
	pool:=getSizedPoolPointerToStarListItem(size)
	return pool.Get().([]*starListItem)
}

// Returns an array of given size and type to the pool
func PutArrayOfPointerToStarListItemIntoPool(arr []*starListItem) {
	pool:=getSizedPoolPointerToStarListItem(cap(arr))
	pool.Put(arr[:cap(arr)])
	arr=nil
}

