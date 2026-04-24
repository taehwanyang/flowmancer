package k8smeta

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	netnsRe = regexp.MustCompile(`net:\[(\d+)\]`)
	// pod<uid> 또는 pod<uid>.slice 류에서 uid 추출
	podUIDRe = regexp.MustCompile(`pod([0-9a-fA-F_\\-]{8,})`)
)

var procRoot = "/proc"

type ProcInfo struct {
	PID      int
	NetnsIno uint32
	PodUID   string
}

func SetProcRoot(root string) {
	if root != "" {
		procRoot = root
	}
}

func ScanProcForPodNetns() ([]ProcInfo, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}

	out := make([]ProcInfo, 0, 256)

	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(ent.Name())
		if err != nil {
			continue
		}

		netnsIno, err := readNetnsIno(pid)
		if err != nil {
			continue
		}

		podUID, err := readPodUIDFromCgroup(pid)
		if err != nil || podUID == "" {
			continue
		}

		out = append(out, ProcInfo{
			PID:      pid,
			NetnsIno: netnsIno,
			PodUID:   podUID,
		})
	}

	return out, nil
}

func readNetnsIno(pid int) (uint32, error) {
	target, err := os.Readlink(filepath.Join(procRoot, strconv.Itoa(pid), "ns/net"))
	if err != nil {
		return 0, err
	}

	m := netnsRe.FindStringSubmatch(target)
	if len(m) != 2 {
		return 0, fmt.Errorf("unexpected netns link: %s", target)
	}

	v, err := strconv.ParseUint(m[1], 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

func readPodUIDFromCgroup(pid int) (string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cgroup"))
	if err != nil {
		return "", err
	}

	s := string(data)
	m := podUIDRe.FindStringSubmatch(s)
	if len(m) != 2 {
		return "", nil
	}

	uid := strings.ReplaceAll(m[1], "_", "-")
	return uid, nil
}
