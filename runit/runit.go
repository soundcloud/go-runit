package runit

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"syscall"
)

const (
	serviceDir = "/etc/service"
	taiOffset  = 4611686018427387914
	statusLen  = 20

	posTimeStart = 0
	posTimeEnd   = 7
	posPidStart  = 12
	posPidEnd    = 15

	posWant  = 17
	posState = 19

	StateDown   = 0
	StateUp     = 1
	StateFinish = 2
)

var (
	ENoRunsv      = errors.New("runsv not running")
	StateToString = map[int]string{
		StateDown:   "down",
		StateUp:     "up",
		StateFinish: "finish",
	}
)

type SvStatus struct {
	Pid        int
	Duration   int
	State      int
	NormallyUp bool
	Want       int
}

type service struct {
	Name       string
	ServiceDir string
}

func GetServices() ([]*service, error) {
	files, err := ioutil.ReadDir(serviceDir)
	if err != nil {
		return nil, err
	}
	services := []*service{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		services = append(services, GetService(file.Name()))
	}
	return services, nil
}

func GetService(name string) *service {
	r := service{Name: name, ServiceDir: serviceDir}
	return &r
}

func (s *service) file(file string) string {
	return fmt.Sprintf("%s/%s/supervise/%s", s.ServiceDir, s.Name, file)
}

func (s *service) runsvRunning() bool {
	file, err := os.OpenFile(s.file("ok"), os.O_WRONLY, 0)
	defer file.Close()
	if err == nil {
		return true
	}
	if err == syscall.ENXIO {
		return false
	}
	panic(err)
}

func (s *service) status() ([]byte, error) {
	file, err := os.Open(s.file("status"))
	defer file.Close()
	if err != nil {
		return nil, err
	}
	status := make([]byte, statusLen)
	_, err = file.Read(status)
	return status, err
}

func (s *service) NormallyUp() bool {
	_, err := os.Stat(s.file("down"))
	return err != nil
}

func (s *service) Status() (*SvStatus, error) {
	if !s.runsvRunning() {
		return nil, ENoRunsv
	}

	status, err := s.status()
	if err != nil {
		return nil, err
	}

	var pid int
	pid = int(status[posPidEnd])
	for i := posPidEnd - 1; i >= posPidStart; i-- {
		pid <<= 8
		pid += int(status[i])
	}

	tai := int64(status[posTimeStart])
	for i := posTimeStart + 1; i <= posTimeEnd; i++ {
		tai <<= 8
		tai += int64(status[i])
	}
	state := status[posState] // 0: down, 1: run, 2: finish

	tv := &syscall.Timeval{}
	if err := syscall.Gettimeofday(tv); err != nil {
		return nil, err
	}
	sS := SvStatus{
		Pid:        pid,
		Duration:   int(tv.Sec - (tai - taiOffset)),
		State:      int(state),
		NormallyUp: s.NormallyUp(),
	}

	switch status[posWant] {
	case 'u':
		sS.Want = StateUp
	case 'd':
		sS.Want = StateDown
	}

	return &sS, nil
}
