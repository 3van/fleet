package ssh

import (
	"bufio"
	"errors"
	"net"
	"os"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/go.crypto/ssh"
	"github.com/coreos/fleet/third_party/code.google.com/p/go.crypto/ssh/terminal"
)

func Execute(client *gossh.ClientConn, cmd string) (*bufio.Reader, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}

	stdout, _ := session.StdoutPipe()
	bstdout := bufio.NewReader(stdout)

	session.Start(cmd)
	go session.Wait()

	return bstdout, nil
}

func Shell(client *gossh.ClientConn) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	modes := gossh.TerminalModes{
		gossh.ECHO:		1,	// enable echoing
		gossh.TTY_OP_ISPEED:	14400,	// input speed = 14.4kbaud
		gossh.TTY_OP_OSPEED:	14400,	// output speed = 14.4kbaud
	}

	fd := int(os.Stdin.Fd())
	oldState, err := terminal.MakeRaw(fd)
	defer terminal.Restore(fd, oldState)

	termWidth, termHeight, err := terminal.GetSize(fd)
	if err != nil {
		return err
	}

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	if err := session.RequestPty("xterm-256color", termHeight, termWidth, modes); err != nil {
		return err
	}

	if err = session.Shell(); err != nil {
		return err
	}

	session.Wait()
	return nil
}

func sshClientConfig(user string) (*gossh.ClientConfig, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("SSH_AUTH_SOCK environment variable is not set. Verify ssh-agent is running. See https://github.com/coreos/fleet/blob/master/Documentation/remote-access.md for help.")
	}

	agent, err := net.Dial("unix", sock)
	if err != nil {
		return nil, err
	}

	cfg := gossh.ClientConfig{
		User:	user,
		Auth:	[]gossh.ClientAuth{
			gossh.ClientAuthAgent(gossh.NewAgentClient(agent)),
		},
	}

	return &cfg, nil
}

func NewSSHClient(user, addr string) (*gossh.ClientConn, error) {
	clientConfig, err := sshClientConfig(user)
	if err != nil {
		return nil, err
	}

	return gossh.Dial("tcp", addr, clientConfig)
}

func NewTunnelledSSHClient(user, tunaddr, tgtaddr string) (*gossh.ClientConn, error) {
	clientConfig, err := sshClientConfig(user)
	if err != nil {
		return nil, err
	}

	tunnelClient, err := gossh.Dial("tcp", tunaddr, clientConfig)
	if err != nil {
		return nil, err
	}

	targetConn, err := tunnelClient.Dial("tcp", tgtaddr)
	if err != nil {
		return nil, err
	}

	targetClient, err := gossh.Client(targetConn, clientConfig)
	if err != nil {
		return nil, err
	}

	return targetClient, nil
}
