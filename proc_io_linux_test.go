package psn

import (
	"fmt"
	"testing"
)

func TestGetProcIOByPID(t *testing.T) {
	fds, err := ListProcFds()
	if err != nil {
		t.Fatal(err)
	}

	fd := fds[len(fds)/2]
	fmt.Println("Chosen FD:", fd)

	pid, err := pidFromFd(fd)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Chosen PID:", pid)

	nm, err := GetProgram(pid)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("GetProgram:", nm)

	ns, err := GetProcIOByPID(pid)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("GetProcIOByPID: %+v\n", ns)

	if ns.WriteBytes != ns.WriteBytesBytesN {
		t.Fatalf("expected same, got %d, %d", ns.WriteBytes, ns.WriteBytesBytesN)
	}
}
