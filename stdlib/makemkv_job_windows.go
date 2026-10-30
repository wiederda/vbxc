//go:build windows

package stdlib

import (
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

var makeMKVJob windows.Handle

func makeMKVInitJob() {
	if makeMKVJob != 0 {
		return
	}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		windows.CloseHandle(job)
		return
	}

	makeMKVJob = job
}

func makeMKVAttachJob(cmd *exec.Cmd) {
	if makeMKVJob == 0 || cmd.Process == nil {
		return
	}

	h, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)

	windows.AssignProcessToJobObject(makeMKVJob, h)
}
