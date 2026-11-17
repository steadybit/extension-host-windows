// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package utils

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procSetSystemTime = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetSystemTime")

// ShiftSystemTime moves the system clock by the given offset.
func ShiftSystemTime(offset time.Duration) error {
	if err := enablePrivilege("SeSystemtimePrivilege"); err != nil {
		return fmt.Errorf("failed to enable the privilege to change the system time: %w", err)
	}

	t := time.Now().Add(offset).UTC()
	st := windows.Systemtime{
		Year:         uint16(t.Year()),
		Month:        uint16(t.Month()),
		DayOfWeek:    uint16(t.Weekday()),
		Day:          uint16(t.Day()),
		Hour:         uint16(t.Hour()),
		Minute:       uint16(t.Minute()),
		Second:       uint16(t.Second()),
		Milliseconds: uint16(t.Nanosecond() / int(time.Millisecond)),
	}
	if r, _, err := procSetSystemTime.Call(uintptr(unsafe.Pointer(&st))); r == 0 {
		return fmt.Errorf("SetSystemTime failed: %w", err)
	}
	return nil
}

func enablePrivilege(name string) error {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer func() { _ = token.Close() }()

	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(name), &luid); err != nil {
		return err
	}

	privileges := windows.Tokenprivileges{PrivilegeCount: 1}
	privileges.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}
	return windows.AdjustTokenPrivileges(token, false, &privileges, 0, nil, nil)
}
