//go:build linux

package cgutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	lcc "github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

// UseV2 indicates whether only cgroups.v2 is enabled. If cgroups.v2 is not
// enabled or is running in hybrid mode with cgroups.v1, Nomad will make use of
// cgroups.v1
//
// This is a read-only value.
var UseV2 = cgroups.IsCgroup2UnifiedMode()

// GetCgroupParent returns the mount point under the root cgroup in which Nomad
// will create cgroups. If parent is not set, an appropriate name for the version
// of cgroups will be used.
func GetCgroupParent(parent string) string {
	if UseV2 {
		return v2GetParent(parent)
	}
	return getParentV1(parent)
}

// CreateCPUSetManager creates a V1 or V2 CpusetManager depending on system configuration.
func CreateCPUSetManager(parent string, logger hclog.Logger) CpusetManager {
	if UseV2 {
		return NewCpusetManagerV2(v2GetParent(parent), logger.Named("cpuset.v2"))
	}
	return NewCpusetManagerV1(getParentV1(parent), logger.Named("cpuset.v1"))
}

func GetCPUsFromCgroup(group string) ([]uint16, error) {
	if UseV2 {
		return v2GetCPUsFromCgroup(v2GetParent(group))
	}
	return getCPUsFromCgroupV1(getParentV1(group))
}

func CgroupID(allocID, task string) string {
	if allocID == "" || task == "" {
		panic("empty alloc or task")
	}

	if UseV2 {
		return fmt.Sprintf("%s.%s.scope", allocID, task)
	}
	return fmt.Sprintf("%s.%s", task, allocID)
}

func ConfigureBasicCgroups(cgroup string, config *lcc.Config) error {
	if UseV2 {
		panic("do not call me for cgroups.v2")
	}

	// In V1 we must setup the freezer cgroup ourselves
	subsystem := "freezer"
	path, err := getCgroupPathHelper(subsystem, filepath.Join(DefaultCgroupV1Parent, cgroup))
	if err != nil {
		return fmt.Errorf("failed to find %s cgroup mountpoint: %v", subsystem, err)
	}
	if err = os.MkdirAll(path, 0755); err != nil {
		return err
	}
	config.Cgroups.Paths = map[string]string{
		subsystem: path,
	}
	return nil
}

func getCpusetSubsystemSettings(parent string) (cpus, mems string, err error) {
	if cpus, err = cgroups.ReadFile(parent, "cpuset.cpus"); err != nil {
		return
	}
	if mems, err = cgroups.ReadFile(parent, "cpuset.mems"); err != nil {
		return
	}
	return cpus, mems, nil
}

// cpusetEnsureParent makes sure that the parent directories of current
// are created and populated with the proper cpus and mems files copied
// from their respective parent. It does that recursively, starting from
// the top of the cpuset hierarchy (i.e. cpuset cgroup mount point).
//
// todo: v1 only?
func cpusetEnsureParent(current string) error {
	var st unix.Statfs_t

	parent := filepath.Dir(current)
	err := unix.Statfs(parent, &st)
	if err == nil && st.Type != unix.CGROUP_SUPER_MAGIC {
		return nil
	}
	// Treat non-existing directory as cgroupfs as it will be created,
	// and the root cpuset directory obviously exists.
	if err != nil && err != unix.ENOENT {
		return &os.PathError{Op: "statfs", Path: parent, Err: err}
	}

	if err := cpusetEnsureParent(parent); err != nil {
		return err
	}
	if err := os.Mkdir(current, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	return cpusetCopyIfNeeded(current, parent)
}

// cpusetCopyIfNeeded copies the cpuset.cpus and cpuset.mems from the parent
// directory to the current directory if the file's contents are 0
//
// todo: v1 only?
func cpusetCopyIfNeeded(current, parent string) error {
	currentCpus, currentMems, err := getCpusetSubsystemSettings(current)
	if err != nil {
		return err
	}
	parentCpus, parentMems, err := getCpusetSubsystemSettings(parent)
	if err != nil {
		return err
	}

	if isEmptyCpuset(currentCpus) {
		if err := cgroups.WriteFile(current, "cpuset.cpus", parentCpus); err != nil {
			return err
		}
	}
	if isEmptyCpuset(currentMems) {
		if err := cgroups.WriteFile(current, "cpuset.mems", parentMems); err != nil {
			return err
		}
	}
	return nil
}

func isEmptyCpuset(str string) bool {
	return str == "" || str == "\n"
}

func getCgroupPathHelper(subsystem, cgroup string) (string, error) {
	fmt.Println("FindCgroupMountPointAndRoot, subsystem:", subsystem, "cgroup:", cgroup)
	mnt, root, err := cgroups.FindCgroupMountpointAndRoot("", subsystem)
	if err != nil {
		return "", err
	}

	// This is needed for nested containers, because in /proc/self/cgroup we
	// see paths from host, which don't exist in container.
	relCgroup, err := filepath.Rel(root, cgroup)
	if err != nil {
		return "", err
	}

	result := filepath.Join(mnt, relCgroup)
	return result, nil
}

// FindCgroupMountpointDir is used to find the cgroup mount point on a Linux
// system.
func FindCgroupMountpointDir() (string, error) {
	mount, err := cgroups.GetCgroupMounts(false)
	if err != nil {
		return "", err
	}
	// It's okay if the mount point is not discovered
	if len(mount) == 0 {
		return "", nil
	}
	return mount[0].Mountpoint, nil
}
