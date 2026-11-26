// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	akn "github.com/steadybit/action-kit/go/action_kit_commons/network"
	"github.com/steadybit/extension-host-windows/exthostwindows/utils"
	"golang.org/x/sys/windows"
)

func getFamily(net net.IPNet) (Family, error) {
	switch {
	case net.IP.To4() != nil:
		return FamilyV4, nil
	case net.IP.To16() != nil:
		return FamilyV6, nil
	default:
		return "", fmt.Errorf("unknown family for %s", net)
	}
}

func getStartEndIP(ipNet net.IPNet) (net.IP, net.IP, error) {
	family, err := getFamily(ipNet)

	if err != nil {
		return nil, nil, err
	}

	if family == FamilyV4 {
		startIp := ipNet.IP.Mask(ipNet.Mask)

		invertedMask := make(net.IP, len(startIp.To4()))

		for i := range invertedMask {
			invertedMask[i] = ^ipNet.Mask[i]
		}

		endIp := make(net.IP, len(startIp.To4()))
		startIpTo4 := startIp.To4()

		for i := range endIp {
			endIp[i] = startIpTo4[i] | invertedMask[i]
		}

		return startIp, endIp, nil
	}

	if family == FamilyV6 {
		startIp := ipNet.IP.Mask(ipNet.Mask)

		invertedMask := make(net.IP, len(startIp.To16()))

		for i := range invertedMask {
			invertedMask[i] = ^ipNet.Mask[i]
		}

		endIp := make(net.IP, len(startIp.To16()))
		startIpTo16 := startIp.To16()

		for i := range endIp {
			endIp[i] = startIpTo16[i] | invertedMask[i]
		}

		return startIp, endIp, nil
	}

	return nil, nil, fmt.Errorf("not implemented")
}

// addressFields returns the WinDivert destination and source address field
// names for the given family (ip.* for IPv4, ipv6.* for IPv6).
func addressFields(family Family) (dst, src string) {
	if family == FamilyV6 {
		return "ipv6.DstAddr", "ipv6.SrcAddr"
	}
	return "ip.DstAddr", "ip.SrcAddr"
}

const openGroup string = " and ("
const closeGroup string = ")"

func buildWinDivertFilter(f Filter) (string, error) {
	var sb strings.Builder

	// Start from a protocol-agnostic base so portless protocols (e.g. ICMP) are
	// matched as well. The include/exclude clauses restrict to tcp/udp only where
	// a port is specified; when no port is given they match every protocol.
	sb.WriteString("true")

	if f.ExcludeLoopback {
		sb.WriteString(" and not loopback")
	}

	if f.Direction != DirectionAll {
		writeDirectionFilter(&sb, f.Direction)
	}

	if len(f.Interfaces) > 0 {
		writeInterfaceFilter(&sb, f.Interfaces)
	}

	if len(f.Include) > 0 {
		err := writeIncludeFilter(&sb, f)
		if err != nil {
			return "", err
		}
	}

	if len(f.Exclude) > 0 {
		err := writeExcludeFilter(&sb, f)
		if err != nil {
			return "", err
		}
	}

	return sb.String(), nil
}

func writeDirectionFilter(sb *strings.Builder, direction Direction) {
	sb.WriteString(" and ")
	if direction == DirectionIncoming {
		sb.WriteString("inbound")
	} else {
		sb.WriteString("outbound")
	}
}

func writeInterfaceFilter(sb *strings.Builder, ifaces []Interface) {
	sb.WriteString(openGroup)
	ifIdxStatements := make([]string, len(ifaces))
	for i, iface := range ifaces {
		ifIdxStatements[i] = fmt.Sprintf("ifIdx == %d", iface.Index)
	}
	sb.WriteString(strings.Join(ifIdxStatements, " or "))
	sb.WriteString(closeGroup)
}

// writeIncludeFilter restricts the attack to the configured addresses and ports.
// Each entry is matched against both the destination and the source, so an
// include affects traffic *to and from* the address/port -- what the action
// parameters promise ("Restrict to/from which ports the traffic is affected")
// and what the Linux attacks do, emitting one rule for dst/dport and one for
// src/sport.
//
// The destination alone is not enough: a port-scoped attack on a server host
// must also affect the responses the host sends *out of* that port, whose
// DstPort is the client's ephemeral port. Matching only DstPort silently spares
// them.
//
// Restricting to inbound or outbound packets is a separate concern, applied by
// writeDirectionFilter, so this clause is direction-independent -- as the
// matching exclude clause already was.
func writeIncludeFilter(sb *strings.Builder, filter Filter) error {
	sb.WriteString(openGroup)
	for i, ran := range filter.Include {
		family, err := getFamily(ran.Net)
		if err != nil {
			return err
		}
		dstAddr, srcAddr := addressFields(family)

		startIp, endIp, err := getStartEndIP(ran.Net)
		if err != nil {
			return err
		}

		sb.WriteString(includeClause(dstAddr, "tcp.DstPort", "udp.DstPort", ran.PortRange, startIp, endIp))
		sb.WriteString(" or ")
		sb.WriteString(includeClause(srcAddr, "tcp.SrcPort", "udp.SrcPort", ran.PortRange, startIp, endIp))

		if i < len(filter.Include)-1 {
			sb.WriteString(" or ")
		}
	}
	sb.WriteString(closeGroup)
	return nil
}

func writeExcludeFilter(sb *strings.Builder, filter Filter) error {
	sb.WriteString(openGroup)
	for i, ran := range filter.Exclude {
		family, err := getFamily(ran.Net)
		if err != nil {
			return err
		}
		dstAddr, srcAddr := addressFields(family)

		startIp, endIp, err := getStartEndIP(ran.Net)
		if err != nil {
			return err
		}

		sb.WriteString(excludeClause(dstAddr, "tcp.DstPort", "udp.DstPort", ran.PortRange, startIp, endIp))
		sb.WriteString(" and ")
		sb.WriteString(excludeClause(srcAddr, "tcp.SrcPort", "udp.SrcPort", ran.PortRange, startIp, endIp))

		if i < len(filter.Exclude)-1 {
			sb.WriteString(" and ")
		}
	}

	sb.WriteString(closeGroup)
	return nil
}

// addressMatch renders the `<addrField> ...` fragment matching a single address
// or an inclusive address range.
func addressMatch(addrField string, startIp, endIp net.IP) string {
	if startIp.String() == endIp.String() {
		return fmt.Sprintf("%s == %s", addrField, startIp.String())
	}
	return fmt.Sprintf("%s >= %s and %s <= %s", addrField, startIp.String(), addrField, endIp.String())
}

// includeClause builds a WinDivert sub-expression matching traffic to/from the
// given address range. For the any-port wildcard it matches every protocol
// (including portless ones such as ICMP); otherwise it restricts to the tcp/udp
// packets whose port falls in the range.
func includeClause(addrField, tcpPortField, udpPortField string, portRange akn.PortRange, startIp, endIp net.IP) string {
	addr := addressMatch(addrField, startIp, endIp)
	if portRange == akn.PortRangeAny {
		return fmt.Sprintf("( %s )", addr)
	}

	var portFilter string
	if portRange.From == portRange.To {
		portFilter = fmt.Sprintf("(( %s == %d ) or ( %s == %d ))", tcpPortField, portRange.From, udpPortField, portRange.From)
	} else {
		portFilter = fmt.Sprintf("(( %s >= %d and %s <= %d ) or ( %s >= %d and %s <= %d ))", tcpPortField, portRange.From, tcpPortField, portRange.To, udpPortField, portRange.From, udpPortField, portRange.To)
	}
	return fmt.Sprintf("( %s and %s)", addr, portFilter)
}

// excludeClause builds a WinDivert sub-expression that spares traffic to/from
// the given address range. For the any-port wildcard it excludes every protocol
// on that address; otherwise it only excludes the tcp/udp packets whose port
// falls in the range.
func excludeClause(addrField, tcpPortField, udpPortField string, portRange akn.PortRange, startIp, endIp net.IP) string {
	addr := addressMatch(addrField, startIp, endIp)
	spare := "false"
	if portRange != akn.PortRangeAny {
		// A port-scoped exclude must spare only the tcp/udp packets whose port is
		// in the range. Each port test is guarded with `not tcp`/`not udp` so that
		// portless protocols (e.g. ICMP) stay subject to the attack — a bare port
		// comparison is false for a packet that has no port, which would otherwise
		// spare all ICMP to/from the excluded address.
		if portRange.From == portRange.To {
			spare = fmt.Sprintf("(( not tcp or %s != %d ) and ( not udp or %s != %d ))", tcpPortField, portRange.From, udpPortField, portRange.From)
		} else {
			spare = fmt.Sprintf("(( not tcp or %s < %d or %s > %d ) and ( not udp or %s < %d or %s > %d ))", tcpPortField, portRange.From, tcpPortField, portRange.To, udpPortField, portRange.From, udpPortField, portRange.To)
		}
	}
	return fmt.Sprintf("(( %s )? %s: true)", addr, spare)
}

// winDivertFilterFile is named by the execution: the attack state does not carry the file name from start to stop.
func winDivertFilterFile(executionId uuid.UUID) string {
	return filepath.Join(os.TempDir(), "wdna-filter-"+executionId.String()+".txt")
}

func buildWinDivertFilterFile(executionId uuid.UUID, f Filter) (string, error) {
	filterContent, err := buildWinDivertFilter(f)
	if err != nil {
		return "", err
	}
	filterFile := winDivertFilterFile(executionId)
	return filterFile, os.WriteFile(filterFile, []byte(filterContent), 0o600)
}

const (
	// The Steadybit build of WinDivert has its own driver service, apart from the one of upstream WinDivert.
	winDivertServiceName    = "SteadybitWinDivert"
	winDivertServiceTimeout = 15 * time.Second
)

func startWinDivert(ctx context.Context, executionId uuid.UUID, opts *WinDivertOpts) error {
	if opts.FlushDNS {
		if out, err := exec.CommandContext(ctx, utils.SystemExecutable("ipconfig.exe"), "/flushdns").CombinedOutput(); err != nil {
			log.Warn().Err(err).Str("output", strings.TrimSpace(string(out))).Msg("failed to flush the DNS cache")
		}
	}

	filterFile, err := buildWinDivertFilterFile(executionId, opts.Filter)
	if err != nil {
		return err
	}

	// Not bound to ctx: wdna has to outlive the request that started the attack.
	cmd := exec.Command("wdna.exe", opts.args(filterFile)...) //NOSONAR arguments are passed without a shell
	// The extension runs as a service without a console, so the output of wdna is only kept by capturing it.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := utils.StartAttackProcess(executionId, cmd); err != nil {
		return fmt.Errorf("failed to start wdna.exe: %w", err)
	}
	awaitCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	go func() {
		err := cmd.Wait()
		log.Info().Err(err).Str("stdout", strings.TrimSpace(stdout.String())).Str("stderr", strings.TrimSpace(stderr.String())).Msg("wdna.exe exited")
		cancel(wdnaExitError(err, stdout.String(), stderr.String()))
	}()

	return utils.AwaitServiceRunning(awaitCtx, winDivertServiceName, winDivertServiceTimeout)
}

// statusDllNotFound is the exit code of a process that Windows could not start because a DLL is missing.
const statusDllNotFound = 0xC0000135

func wdnaExitError(err error, stdout, stderr string) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && uint32(exitErr.ExitCode()) == statusDllNotFound {
		return errors.New("wdna.exe could not start because a DLL is missing, e.g. VCRUNTIME140.dll of the Microsoft Visual C++ Redistributable")
	}
	msg := "wdna.exe exited before the WinDivert service started"
	// wdna reports a failed start on stderr, but invalid arguments on stdout.
	if reason := cmp.Or(lastLine(stderr), lastLine(stdout)); reason != "" {
		msg += ": " + reason
	}
	if err == nil {
		return errors.New(msg)
	}
	return fmt.Errorf("%s: %w", msg, err)
}

func lastLine(output string) string {
	output = strings.TrimSpace(output)
	return strings.TrimSpace(output[strings.LastIndexByte(output, '\n')+1:])
}

func (o *WinDivertOpts) args(filterFile string) []string {
	args := []string{"--file=" + filterFile, "--mode=" + string(o.Mode), fmt.Sprintf("--duration=%d", int(o.Duration.Seconds()))}
	switch o.Mode {
	case WinDivertModeDelay:
		args = append(args, fmt.Sprintf("--time=%d", o.Delay.Milliseconds()))
		if o.Jitter {
			args = append(args, "--jitter")
		}
	case WinDivertModeDrop, WinDivertModeCorrupt:
		args = append(args, fmt.Sprintf("--percentage=%d", o.Percentage))
	}
	return args
}

// stopWinDivert ends the wdna of the attack and stops the driver service. WinDivert marks the service for deletion right
// after starting it, so stopping it also unloads and removes the driver; a missing service therefore counts as stopped.
func stopWinDivert(executionId uuid.UUID) error {
	wdnaErr := stopWdna(executionId, wdnaStopTimeout)
	_ = os.Remove(winDivertFilterFile(executionId))

	err := utils.StopService(winDivertServiceName, winDivertServiceTimeout)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		err = nil
	}
	return errors.Join(wdnaErr, err)
}

const (
	// wdna creates this event with its own process id and ends once it is set, see examples/wdna/wdna.c in
	// steadybit/WinDivert.
	wdnaStopEventFormat = `Global\steadybit-wdna-stop-%d`
	// wdna sends the packets it still holds right away when it stops, which takes well under a second. Killing it also
	// restores the traffic, but drops those packets.
	wdnaStopTimeout = 10 * time.Second
)

// stopWdna asks the wdna of the attack to exit through its stop event and terminates it after timeout.
func stopWdna(executionId uuid.UUID, timeout time.Duration) error {
	pids, err := utils.AttackProcessIds(executionId)
	for _, pid := range pids {
		if setErr := setWdnaStopEvent(pid); setErr != nil {
			// Expected when wdna is just exiting because its duration elapsed; otherwise it is terminated below.
			log.Debug().Err(setErr).Int("pid", pid).Msg("failed to set the stop event of wdna.exe")
		}
	}
	return errors.Join(err, utils.StopAttackProcesses(executionId, timeout))
}

func setWdnaStopEvent(pid int) error {
	name, err := windows.UTF16PtrFromString(fmt.Sprintf(wdnaStopEventFormat, pid))
	if err != nil {
		return err
	}
	event, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, name)
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(event) }()
	return windows.SetEvent(event)
}
