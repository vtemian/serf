package agent

import (
	"github.com/hashicorp/serf/testutil"
	"github.com/mitchellh/cli"
	"log"
	"net/rpc"
	"testing"
	"time"
)

func TestCommand_implements(t *testing.T) {
	var _ cli.Command = new(Command)
}

func TestCommandRun(t *testing.T) {
	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	ui := new(cli.MockUi)
	c := &Command{
		ShutdownCh: shutdownCh,
		Ui:         ui,
	}

	args := []string{
		"-bind", testutil.GetBindAddr().String(),
		"-rpc-addr", getRPCAddr(),
	}

	resultCh := make(chan int)
	go func() {
		resultCh <- c.Run(args)
	}()

	testutil.Yield()

	// Verify it runs "forever"
	select {
	case <-resultCh:
		t.Fatalf("ended too soon, err: %s", ui.ErrorWriter.String())
	case <-time.After(50 * time.Millisecond):
	}

	// Send a shutdown request
	shutdownCh <- struct{}{}

	select {
	case code := <-resultCh:
		if code != 0 {
			t.Fatalf("bad code: %d", code)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timeout")
	}
}

func TestCommandRun_rpc(t *testing.T) {
	doneCh := make(chan struct{})
	shutdownCh := make(chan struct{})
	defer func() {
		close(shutdownCh)
		<-doneCh
	}()

	c := &Command{
		ShutdownCh: shutdownCh,
		Ui:         new(cli.MockUi),
	}

	rpcAddr := getRPCAddr()
	args := []string{
		"-bind", testutil.GetBindAddr().String(),
		"-rpc-addr", rpcAddr,
	}

	go func() {
		code := c.Run(args)
		if code != 0 {
			log.Printf("bad: %d", code)
		}

		close(doneCh)
	}()

	testutil.Yield()

	rpcConn, err := rpc.Dial("tcp", rpcAddr)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer rpcConn.Close()

	client := &RPCClient{Client: rpcConn}
	members, err := client.Members()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(members) != 1 {
		t.Fatalf("bad: %#v", members)
	}
}

func TestCommandRun_join(t *testing.T) {
	a1 := testAgent()
	if err := a1.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}
	defer a1.Shutdown()

	doneCh := make(chan struct{})
	shutdownCh := make(chan struct{})
	defer func() {
		close(shutdownCh)
		<-doneCh
	}()

	c := &Command{
		ShutdownCh: shutdownCh,
		Ui:         new(cli.MockUi),
	}

	args := []string{
		"-bind", testutil.GetBindAddr().String(),
		"-join", a1.SerfConfig.MemberlistConfig.BindAddr,
	}

	go func() {
		code := c.Run(args)
		if code != 0 {
			log.Printf("bad: %d", code)
		}

		close(doneCh)
	}()

	testutil.Yield()

	if len(a1.Serf().Members()) != 2 {
		t.Fatalf("bad: %#v", a1.Serf().Members())
	}
}

func TestCommandRun_joinFail(t *testing.T) {
	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	c := &Command{
		ShutdownCh: shutdownCh,
		Ui:         new(cli.MockUi),
	}

	args := []string{
		"-bind", testutil.GetBindAddr().String(),
		"-join", testutil.GetBindAddr().String(),
	}

	code := c.Run(args)
	if code == 0 {
		t.Fatal("should fail")
	}
}
