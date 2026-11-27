// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package utils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/windows"
)

// KillChildProcessesOnExit puts the extension into a job object that kills all its processes once the extension exits,
// crash included. Processes the extension starts join the job, so no attack process outlives the extension.
func KillChildProcessesOnExit() error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to create the job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return fmt.Errorf("failed to configure the job object: %w", err)
	}
	if err := windows.AssignProcessToJobObject(job, windows.CurrentProcess()); err != nil {
		_ = windows.CloseHandle(job)
		return fmt.Errorf("failed to assign the extension to the job object: %w", err)
	}
	// The handle stays open for the lifetime of the extension: closing it would kill the extension itself.
	return nil
}

// attackJob is a job object for the processes of one attack, nested in the job of KillChildProcessesOnExit. Stopping
// it ends exactly those processes, including the ones they start themselves, and no other process with the same name.
type attackJob struct {
	// mu is held while the job is used, so a concurrent stop of the same attack cannot close the handle in use.
	mu     sync.Mutex
	closed bool
	handle windows.Handle
}

func newAttackJob() (*attackJob, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create the job object of the attack: %w", err)
	}
	return &attackJob{handle: job}, nil
}

// Start starts cmd in the job. A process that cannot be added to the job is killed: nothing could stop it.
func (j *attackJob) Start(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// Suspended until it is in the job: running, it could start processes outside the job or exit before it is added.
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	if err := cmd.Start(); err != nil {
		return err
	}
	err := j.assign(cmd.Process)
	if err == nil {
		err = resumeProcess(cmd.Process.Pid)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("failed to start %s in the job object of the attack: %w", filepath.Base(cmd.Path), err)
	}
	return nil
}

func (j *attackJob) assign(p *os.Process) error {
	var err error
	if handleErr := p.WithHandle(func(h uintptr) { err = windows.AssignProcessToJobObject(j.handle, windows.Handle(h)) }); handleErr != nil {
		return handleErr
	}
	return err
}

// resumeProcess resumes the threads of a process created suspended; os/exec does not keep the handle of its thread.
func resumeProcess(pid int) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != uint32(pid) {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}
		_, err = windows.ResumeThread(thread)
		_ = windows.CloseHandle(thread)
		if err != nil {
			return err
		}
	}
	if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil
	}
	return err
}

func (j *attackJob) ProcessIds() ([]int, error) {
	var list struct {
		assigned uint32
		listed   uint32
		pids     [256]uintptr
	}
	if err := windows.QueryInformationJobObject(j.handle, windows.JobObjectBasicProcessIdList, uintptr(unsafe.Pointer(&list)), uint32(unsafe.Sizeof(list)), nil); err != nil {
		return nil, fmt.Errorf("failed to list the processes of the attack: %w", err)
	}
	pids := make([]int, list.listed)
	for i, pid := range list.pids[:list.listed] {
		pids[i] = int(pid)
	}
	return pids, nil
}

// Stop terminates the processes in the job and waits until they have exited, so files they write can be removed
// afterward. A grace period gives them time to exit on their own first, after the caller asked them to. The job stays
// usable when Stop fails, so the stop can be retried.
func (j *attackJob) Stop(grace time.Duration) error {
	// The job counts a process as gone while it is still exiting, e.g. dd with writes in flight that keep its file open;
	// only the process itself tells when it has exited.
	processes, openErr := j.openProcesses()
	defer func() {
		for _, h := range processes {
			_ = windows.CloseHandle(h)
		}
	}()
	if grace > 0 {
		if awaitExit(processes, time.Now().Add(grace)) && openErr == nil {
			return nil
		}
		log.Warn().Dur("grace", grace).Msg("processes of the attack did not exit in time, terminating them")
	}
	if err := windows.TerminateJobObject(j.handle, 1); err != nil {
		return fmt.Errorf("failed to terminate the processes of the attack: %w", err)
	}
	if !awaitExit(processes, time.Now().Add(terminateWaitTimeout)) {
		return errors.New("the processes of the attack did not exit after they were terminated")
	}
	return openErr
}

// Close releases the job; processes still in it keep running.
func (j *attackJob) Close() {
	_ = windows.CloseHandle(j.handle)
}

func (j *attackJob) openProcesses() ([]windows.Handle, error) {
	pids, err := j.ProcessIds()
	if err != nil {
		return nil, err
	}
	var processes []windows.Handle
	for _, pid := range pids {
		h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			continue
		}
		if err != nil {
			return processes, fmt.Errorf("failed to open process %d of the attack: %w", pid, err)
		}
		processes = append(processes, h)
	}
	return processes, nil
}

func awaitExit(processes []windows.Handle, deadline time.Time) bool {
	for _, h := range processes {
		wait := max(time.Until(deadline), 0)
		if event, _ := windows.WaitForSingleObject(h, uint32(wait.Milliseconds())); event != windows.WAIT_OBJECT_0 {
			return false
		}
	}
	return true
}

// attackJobs holds the job of each attack by its execution id: the action state cannot carry a handle. After a restart
// there is nothing left to stop, the job of KillChildProcessesOnExit ended it all.
var attackJobs = struct {
	sync.Mutex
	byExecution map[uuid.UUID]*attackJob
}{byExecution: map[uuid.UUID]*attackJob{}}

// lockAttackJob returns the job of the attack locked, nil if there is none and create is false.
func lockAttackJob(executionId uuid.UUID, create bool) (*attackJob, error) {
	for {
		attackJobs.Lock()
		job, ok := attackJobs.byExecution[executionId]
		if !ok && create {
			var err error
			if job, err = newAttackJob(); err != nil {
				attackJobs.Unlock()
				return nil, err
			}
			attackJobs.byExecution[executionId] = job
		}
		attackJobs.Unlock()
		if job == nil {
			return nil, nil
		}
		job.mu.Lock()
		if !job.closed {
			return job, nil
		}
		// Stopped meanwhile and already removed from attackJobs.
		job.mu.Unlock()
	}
}

func StartAttackProcess(executionId uuid.UUID, cmd *exec.Cmd) error {
	job, err := lockAttackJob(executionId, true)
	if err != nil {
		return err
	}
	err = job.Start(cmd)
	job.mu.Unlock()
	if err != nil {
		// The platform need not call stop after a failed start, e.g. dd failing after devzero started.
		_ = StopAttackProcesses(executionId, 0)
		return err
	}
	return nil
}

func AttackProcessesRunning(executionId uuid.UUID) (bool, error) {
	pids, err := AttackProcessIds(executionId)
	return len(pids) > 0, err
}

func AttackProcessIds(executionId uuid.UUID) ([]int, error) {
	job, _ := lockAttackJob(executionId, false)
	if job == nil {
		return nil, nil
	}
	defer job.mu.Unlock()
	return job.ProcessIds()
}

// StopAttackProcesses ends the processes of an attack, see attackJob.Stop for grace.
func StopAttackProcesses(executionId uuid.UUID, grace time.Duration) error {
	job, _ := lockAttackJob(executionId, false)
	if job == nil {
		return nil
	}
	defer job.mu.Unlock()
	if err := job.Stop(grace); err != nil {
		return err
	}
	attackJobs.Lock()
	delete(attackJobs.byExecution, executionId)
	attackJobs.Unlock()
	job.closed = true
	job.Close()
	return nil
}
