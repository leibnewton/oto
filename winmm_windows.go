// Copyright 2017 Hajime Hoshi
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !js

package oto

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winmm = windows.NewLazySystemDLL("winmm")
)

var (
	procWaveOutOpen          = winmm.NewProc("waveOutOpen")
	procWaveOutClose         = winmm.NewProc("waveOutClose")
	procWaveOutPrepareHeader = winmm.NewProc("waveOutPrepareHeader")
	procWaveOutWrite         = winmm.NewProc("waveOutWrite")
	procWaveOutGetNumDevs    = winmm.NewProc("waveOutGetNumDevs")
	procWaveOutGetDevCapsW   = winmm.NewProc("waveOutGetDevCapsW")
)

type wavehdr struct {
	lpData          uintptr
	dwBufferLength  uint32
	dwBytesRecorded uint32
	dwUser          uintptr
	dwFlags         uint32
	dwLoops         uint32
	lpNext          uintptr
	reserved        uintptr
}

type waveformatex struct {
	wFormatTag      uint16
	nChannels       uint16
	nSamplesPerSec  uint32
	nAvgBytesPerSec uint32
	nBlockAlign     uint16
	wBitsPerSample  uint16
	cbSize          uint16
}

type wavecap struct {
	Mid           uint16     // manufacturer ID
	Pid           uint16     // product ID
	DriverVersion uint32     // version of the driver
	Pname         [32]uint16 // product name (NULL terminated string)
	Formats       uint32     // formats supported
	Channels      uint16     // number of sources supported
	Reserved1     uint16     // packing
	Support       uint32     // functionality supported by driver
}

const (
	waveFormatPCM = 1
	whdrInqueue   = 16
)

type mmresult uint

const (
	mmsyserrNoerror       mmresult = 0
	mmsyserrError         mmresult = 1
	mmsyserrBaddeviceid   mmresult = 2
	mmsyserrAllocated     mmresult = 4
	mmsyserrInvalidhandle mmresult = 5
	mmsyserrNodriver      mmresult = 6
	mmsyserrNomem         mmresult = 7
	waveerrBadformat      mmresult = 32
	waveerrStillplaying   mmresult = 33
	waveerrUnprepared     mmresult = 34
	waveerrSync           mmresult = 35
)

func (m mmresult) String() string {
	switch m {
	case mmsyserrNoerror:
		return "MMSYSERR_NOERROR"
	case mmsyserrError:
		return "MMSYSERR_ERROR"
	case mmsyserrBaddeviceid:
		return "MMSYSERR_BADDEVICEID"
	case mmsyserrAllocated:
		return "MMSYSERR_ALLOCATED"
	case mmsyserrInvalidhandle:
		return "MMSYSERR_INVALIDHANDLE"
	case mmsyserrNodriver:
		return "MMSYSERR_NODRIVER"
	case mmsyserrNomem:
		return "MMSYSERR_NOMEM"
	case waveerrBadformat:
		return "WAVEERR_BADFORMAT"
	case waveerrStillplaying:
		return "WAVEERR_STILLPLAYING"
	case waveerrUnprepared:
		return "WAVEERR_UNPREPARED"
	case waveerrSync:
		return "WAVEERR_SYNC"
	}
	return fmt.Sprintf("MMRESULT (%d)", m)
}

type winmmError struct {
	fname    string
	errno    windows.Errno
	mmresult mmresult
}

func (e *winmmError) Error() string {
	if e.mmresult != mmsyserrNoerror {
		return fmt.Sprintf("winmm error at %s: %s Errno: %d", e.fname, e.mmresult, e.errno)
	}
	return fmt.Sprintf("winmm error at %s: Errno: %d", e.fname, e.errno)
}

func waveOutOpen(f *waveformatex, deviceNum int) (uintptr, error) {
	const (
		waveMapper   = 0xffffffff
		callbackNull = 0
	)
	var w uintptr
	var dev uintptr = waveMapper
	if deviceNum >= 0 {
		dev = uintptr(deviceNum)
	}
	r, _, e := procWaveOutOpen.Call(uintptr(unsafe.Pointer(&w)), dev, uintptr(unsafe.Pointer(f)),
		0, 0, callbackNull)
	runtime.KeepAlive(f)
	if mmresult(r) != mmsyserrNoerror {
		return 0, &winmmError{
			fname:    "waveOutOpen",
			mmresult: mmresult(r),
			errno:    e.(windows.Errno),
		}
	}
	return w, nil
}

func waveOutClose(hwo uintptr) error {
	r, _, e := procWaveOutClose.Call(hwo)
	// WAVERR_STILLPLAYING is ignored.
	if mmresult(r) != mmsyserrNoerror && mmresult(r) != waveerrStillplaying {
		return &winmmError{
			fname:    "waveOutClose",
			mmresult: mmresult(r),
			errno:    e.(windows.Errno),
		}
	}
	return nil
}

func waveOutPrepareHeader(hwo uintptr, pwh *wavehdr) error {
	r, _, e := procWaveOutPrepareHeader.Call(hwo, uintptr(unsafe.Pointer(pwh)), unsafe.Sizeof(wavehdr{}))
	runtime.KeepAlive(pwh)
	if mmresult(r) != mmsyserrNoerror {
		return &winmmError{
			fname:    "waveOutPrepareHeader",
			mmresult: mmresult(r),
			errno:    e.(windows.Errno),
		}
	}
	return nil
}

func waveOutWrite(hwo uintptr, pwh *wavehdr) error {
	r, _, e := procWaveOutWrite.Call(hwo, uintptr(unsafe.Pointer(pwh)), unsafe.Sizeof(wavehdr{}))
	runtime.KeepAlive(pwh)
	if mmresult(r) != mmsyserrNoerror {
		return &winmmError{
			fname:    "waveOutWrite",
			mmresult: mmresult(r),
			errno:    e.(windows.Errno),
		}
	}
	return nil
}

func waveOutGetNumDevs() (int, error) {
	r, _, e := procWaveOutGetNumDevs.Call()
	if r == 0 && e.(windows.Errno) != 0 {
		return 0, &winmmError{
			fname: "waveOutGetNumDevs",
			errno: e.(windows.Errno),
		}
	}
	return int(r), nil
}

func waveOutGetDevCaps(uDeviceID uint32) (*Device, error) {
	pwoc := &wavecap{}
	r, _, e := procWaveOutGetDevCapsW.Call(uintptr(uDeviceID), uintptr(unsafe.Pointer(pwoc)), unsafe.Sizeof(wavecap{}))
	runtime.KeepAlive(pwoc)
	if mmresult(r) != mmsyserrNoerror {
		return nil, &winmmError{
			fname:    "waveOutGetDevCaps",
			mmresult: mmresult(r),
			errno:    e.(windows.Errno),
		}
	}
	return &Device{
		Mid:      pwoc.Mid,
		Pid:      pwoc.Pid,
		Name:     syscall.UTF16ToString(pwoc.Pname[:]),
		Formats:  pwoc.Formats,
		Channels: int(pwoc.Channels),
		Support:  pwoc.Support,
	}, nil
}
