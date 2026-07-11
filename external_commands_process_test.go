package main

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestStartAndReapCommandWaitsForChild(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestExternalCommandHelperProcess")
	cmd.Env = append(os.Environ(), "NMF_EXTERNAL_COMMAND_HELPER=1")
	exited := make(chan error, 1)

	if err := startAndReapCommand(cmd, func(err error) { exited <- err }); err != nil {
		t.Fatalf("startAndReapCommand: %v", err)
	}
	select {
	case err := <-exited:
		if err != nil {
			t.Fatalf("child exit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child process was not waited for")
	}
	if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
		t.Fatalf("process state = %#v, want exited child", cmd.ProcessState)
	}
}

func TestExternalCommandHelperProcess(t *testing.T) {
	if os.Getenv("NMF_EXTERNAL_COMMAND_HELPER") != "1" {
		return
	}
}
