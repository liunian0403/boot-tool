//go:build windows

package gui

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/liunian0403/boot-tool/internal/diag"
)

// createToast builds the startup popup hidden and arms the settle delay. It is
// deliberately created before it is shown so that the stage sequence can fail
// loudly (a popup that never appears must not be silent, PRD F8 / AC0).
func createToast(o Options) error {
	if err := ensureToastClass(); err != nil {
		return err
	}

	hwnd, _, err := procCreateWindowExW.Call(
		wsExTopmost|wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(utf16(toastClassName))),
		0,
		wsPopup,
		0, 0,
		toastW, toastH,
		0, 0,
		hInstance,
		0,
	)
	if hwnd == 0 {
		return diag.FailStage("toast", fmt.Errorf("CreateWindowExW(%s): %w", toastClassName, err))
	}
	hToast = hwnd

	// Control heights are sized to the line height of each font (Segoe UI at
	// -17/-21/-28 units): a static control vertically centres its text and
	// clips it once the line is taller than the rect, so these must not shrink.
	t := view.ToastView()
	hToastHeader = createStatic(hwnd, t.Header, 18, 12, toastW-36, 26, ssNotify)
	hToastDuration = createStatic(hwnd, t.Duration, 16, 34, toastW-32, 44, ssNotify)
	hToastDone = createStatic(hwnd, t.Done, 18, 84, toastW-36, 32, ssNotify)
	hToastHint = createStatic(hwnd, t.Hint, 18, 118, toastW-36, 26, ssNotify)
	if hToastHeader == 0 || hToastDuration == 0 || hToastDone == 0 || hToastHint == 0 {
		return diag.FailStage("toast-controls", fmt.Errorf(
			"CreateWindowExW(STATIC): header=%d duration=%d boot=%d hint=%d, %w",
			hToastHeader, hToastDuration, hToastDone, hToastHint, syscall.GetLastError()))
	}
	diag.Logf("stage: toast-controls ok")

	if fontSmall != 0 {
		sendWMSetFont(hToastHeader, fontSmall)
		sendWMSetFont(hToastHint, fontSmall)
	}
	if fontNormal != 0 {
		sendWMSetFont(hToastDone, fontNormal)
	}
	if fontToastLarge != 0 {
		sendWMSetFont(hToastDuration, fontToastLarge)
	}

	positionToast(hwnd)
	diag.Logf("stage: toast ok (hwnd=%d)", hwnd)

	toastVisible = false
	scheduleToastAuto(hwnd, o.ToastDelay, o.ToastTimeout)
	diag.Logf("stage: toast-delay %s", o.ToastDelay)
	return nil
}

// scheduleToastAuto drives the popup's two phases from a plain goroutine and
// posted messages.
//
// Why not SetTimer: WM_TIMER is the lowest-priority message — Windows only
// generates it when the queue holds nothing else — so a busy desktop (other
// windows repainting, a screenshot tool, antivirus work) can starve it for many
// seconds, leaving the popup on screen far past its deadline. That was observed
// while testing this popup. A goroutine sleep plus PostMessage has no such
// failure mode: posted messages are ordinary messages, always delivered in
// order, and a window that was destroyed in the meantime just makes the post
// fail harmlessly.
func scheduleToastAuto(hwnd uintptr, delay, timeout time.Duration) {
	go func() {
		time.Sleep(delay)
		if r, _, _ := procPostMessageW.Call(hwnd, wmAppToastShow, 0, 0); r == 0 {
			return // popup already closed (clicked or replaced by the main window)
		}
		time.Sleep(timeout)
		procPostMessageW.Call(hwnd, wmAppToastHide, 0, 0)
	}()
}

// ensureToastClass registers the popup window class exactly once.
func ensureToastClass() error {
	if toastClassOn || toastClassTried {
		return nil
	}
	toastClassTried = true
	if err := registerClass(toastClassName, csDropShadow); err != nil {
		return diag.FailStage("toast-class", err)
	}
	toastClassOn = true
	diag.Logf("stage: toast-class ok")
	return nil
}

// showToast re-reads the view (so the popup reports the duration at the moment
// it appears, not the one read before the settle delay) and shows it.
func showToast() {
	if hToast == 0 || view == nil {
		return
	}
	t := view.ToastView()
	setText(hToastHeader, t.Header)
	setText(hToastDuration, t.Duration)
	setText(hToastDone, t.Done)
	setText(hToastHint, t.Hint)

	positionToast(hToast)
	procShowWindow.Call(hToast, swShowNoActivate)
	toastVisible = true
	diag.Logf("stage: toast-shown duration=%s", t.Duration)
}

// closeToast hides and destroys the popup. With no main window the process then
// exits (wmDestroy posts the quit message).
func closeToast() {
	if hToast == 0 {
		return
	}
	hwnd := hToast
	toastVisible = false
	procShowWindow.Call(hwnd, 0) // SW_HIDE
	procDestroyWindow.Call(hwnd)
}

// openMainFromToast is the popup's click action: the main window replaces it.
func openMainFromToast() {
	if err := showMainWindow(); err != nil {
		diag.Logf("opening the main window from the popup failed: %v", err)
		return
	}
	closeToast()
}

// positionToast pins the popup to the bottom-right corner of the primary
// monitor's work area (above the taskbar), clamped so it can never land
// off-screen.
func positionToast(hwnd uintptr) {
	var wa rect
	ok, _, _ := procSystemParametersInfo.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&wa)), 0)
	if ok == 0 {
		w, _, _ := procGetSystemMetrics.Call(smCXScreen)
		h, _, _ := procGetSystemMetrics.Call(smCYScreen)
		wa = rect{left: 0, top: 0, right: int32(w), bottom: int32(h)}
	}

	x := wa.right - toastW - toastMargin
	y := wa.bottom - toastH - toastMargin
	if x < wa.left {
		x = wa.left
	}
	if y < wa.top {
		y = wa.top
	}
	procSetWindowPos.Call(
		hwnd,
		hwndTopmost,
		uintptr(uint32(x)),
		uintptr(uint32(y)),
		0, 0,
		swpNoSize|swpNoActivate,
	)
}
