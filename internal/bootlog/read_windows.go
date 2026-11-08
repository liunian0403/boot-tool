//go:build windows

package bootlog

import (
	"encoding/xml"
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Read returns the current boot session from the System event log.
//
// The query is narrow on purpose: only three providers, three event ids and the
// newest 200 records, because every record costs an EvtRender round trip and the
// app must be on screen within a few seconds of logon.
func Read() (Report, error) {
	events, err := readSystemEvents()
	if err != nil {
		return Report{}, err
	}
	return analyze(events)
}

const (
	systemChannel = "System"

	// querySystemBoot keeps the enumeration small: boot markers (12/27) and
	// logon completions (7001) only, from the three providers that own them.
	querySystemBoot = `*[System[(EventID=12 or EventID=27 or EventID=7001)]]`

	evtQueryChannelPath      = 0x00000001
	evtQueryReverseDirection = 0x00000200
	evtRenderEventXML        = 1
	errorInsufficientBuffer  = 122
	errorNoMoreItems         = 259

	maxEvents = 200
	batchSize = 16
)

var (
	wevtapi       = syscall.NewLazyDLL("wevtapi.dll")
	procEvtQuery  = wevtapi.NewProc("EvtQuery")
	procEvtNext   = wevtapi.NewProc("EvtNext")
	procEvtRender = wevtapi.NewProc("EvtRender")
	procEvtClose  = wevtapi.NewProc("EvtClose")
)

// eventXML mirrors the parts of an EvtRender(EventXml) document we need.
type eventXML struct {
	System struct {
		Provider struct {
			Name string `xml:"Name,attr"`
		} `xml:"Provider"`
		EventID     int `xml:"EventID"`
		TimeCreated struct {
			SystemTime string `xml:"SystemTime,attr"`
		} `xml:"TimeCreated"`
	} `xml:"System"`
	EventData struct {
		Data []struct {
			Name  string `xml:"Name,attr"`
			Value string `xml:",chardata"`
		} `xml:"Data"`
	} `xml:"EventData"`
}

// readSystemEvents enumerates the newest matching records, newest first.
func readSystemEvents() ([]event, error) {
	path := utf16(systemChannel)
	query := utf16(querySystemBoot)

	h, _, callErr := procEvtQuery.Call(
		0,
		uintptr(unsafe.Pointer(path)),
		uintptr(unsafe.Pointer(query)),
		evtQueryChannelPath|evtQueryReverseDirection,
	)
	if h == 0 {
		return nil, fmt.Errorf("bootlog: EvtQuery(%s): %w", systemChannel, callErr)
	}
	defer procEvtClose.Call(h)

	out := make([]event, 0, maxEvents)
	handles := make([]uintptr, batchSize)
	for len(out) < maxEvents {
		var returned uint32
		ok, _, callErr := procEvtNext.Call(
			h,
			batchSize,
			uintptr(unsafe.Pointer(&handles[0])),
			0, 0,
			uintptr(unsafe.Pointer(&returned)),
		)
		if returned == 0 {
			if ok == 0 {
				if errno, isErrno := callErr.(syscall.Errno); isErrno && errno == errorNoMoreItems {
					break
				}
				if len(out) == 0 {
					return nil, fmt.Errorf("bootlog: EvtNext: %w", callErr)
				}
			}
			break
		}
		for i := uint32(0); i < returned; i++ {
			ev, err := renderEvent(handles[i])
			procEvtClose.Call(handles[i])
			if err != nil {
				continue // one unreadable record must not lose the whole report
			}
			out = append(out, ev)
			if len(out) >= maxEvents {
				break
			}
		}
	}
	return out, nil
}

// renderEvent turns one event handle into XML and parses it.
func renderEvent(h uintptr) (event, error) {
	var used, props uint32

	// Probe for the buffer size: the call fails with ERROR_INSUFFICIENT_BUFFER
	// and fills `used` with the required byte count.
	if r, _, _ := procEvtRender.Call(0, h, evtRenderEventXML, 0, 0,
		uintptr(unsafe.Pointer(&used)), uintptr(unsafe.Pointer(&props))); r != 0 {
		return event{}, fmt.Errorf("bootlog: EvtRender probe unexpectedly succeeded")
	}
	if used == 0 {
		return event{}, fmt.Errorf("bootlog: EvtRender probe returned an empty size")
	}

	buf := make([]uint16, used/2+1)
	r, _, callErr := procEvtRender.Call(
		0, h, evtRenderEventXML,
		uintptr(len(buf)*2),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&used)),
		uintptr(unsafe.Pointer(&props)),
	)
	if r == 0 {
		return event{}, fmt.Errorf("bootlog: EvtRender: %w", callErr)
	}

	var x eventXML
	if err := xml.Unmarshal([]byte(syscall.UTF16ToString(buf)), &x); err != nil {
		return event{}, fmt.Errorf("bootlog: parse event xml: %w", err)
	}
	at, err := time.Parse(time.RFC3339Nano, x.System.TimeCreated.SystemTime)
	if err != nil {
		return event{}, fmt.Errorf("bootlog: parse event time %q: %w", x.System.TimeCreated.SystemTime, err)
	}
	data := make(map[string]string, len(x.EventData.Data))
	for _, d := range x.EventData.Data {
		if d.Name != "" {
			data[d.Name] = strings.TrimSpace(d.Value)
		}
	}
	return event{
		Provider: x.System.Provider.Name,
		ID:       x.System.EventID,
		At:       at,
		Data:     data,
	}, nil
}

func utf16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}
