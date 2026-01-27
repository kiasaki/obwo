//go:build linux
// +build linux

package main

import (
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func main() {
	switch os.Args[1] {
	case "run":
		run(os.Args[2:]...)
	case "child":
		child(os.Args[2:]...)
	default:
		log.Fatal("unknown command")
	}
}

func run(command ...string) {
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, command...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}
	check(cmd.Run())
}

func child(command ...string) {
	name := "run" + strconv.Itoa(rand.Intn(1000))
	if _, err := os.Stat("/tmp/" + name); os.IsNotExist(err) {
		check(os.Mkdir("/tmp/"+name, os.ModePerm))
	}
	f, err := os.Open(command[0])
	check(err)
	defer f.Close()
	t, err := os.Create("/tmp/" + name + "/main")
	check(err)
	defer t.Close()
	_, err := io.Copy(t, f)
	check(err)

	cg(name)
	cmd := exec.Command("/main", command[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	check(syscall.Sethostname([]byte("container")))
	check(syscall.Chroot("./root"))
	check(os.Chdir("/"))
	check(syscall.Mount("proc", "proc", "proc", 0, ""))
	check(cmd.Run())
	check(syscall.Unmount("proc", 0))
}

func cg(name string) {
	c := "/sys/fs/cgroup/pids/" + name
	os.Mkdir(c, 0755)
	check(ioutil.WriteFile(filepath.Join(c, "pids.max"), []byte("32"), 0700))
	check(ioutil.WriteFile(filepath.Join(c, "notify_on_release"), []byte("1"), 0700))
	check(ioutil.WriteFile(filepath.Join(c, "cgroup.procs"), []byte(strconv.Itoa(os.Getpid())), 0700))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
