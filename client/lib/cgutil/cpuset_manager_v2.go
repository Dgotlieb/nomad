//go:build linux

package cgutil

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/lib/cpuset"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fs2"
	"github.com/opencontainers/runc/libcontainer/configs"
)

const (
	// V2defaultCgroupParent is the name of Nomad's default parent cgroup, under which
	// all other cgroups are managed. This can be changed with client configuration
	// in case for e.g. Nomad tasks should be further constrained by an externally
	// configured systemd cgroup.
	V2defaultCgroupParent = "nomad.slice"

	// v2CgroupRoot is hard-coded in the cgroups.v2 specification.
	v2CgroupRoot = "/sys/fs/cgroup"

	// v2isRootless is (for now) always false; Nomad clients require root.
	v2isRootless = false

	// v2CreationPID is a special PID in libcontainer used to denote a cgroup
	// should be created, but with no process added.
	v2CreationPID = -1
)

// identifier is the "<allocID>.<taskName>" string that uniquely identifies an
// individual instance of a task within the flat cgroup namespace
type identifier = string

// nothing is used for treating a map like a set with no values
type nothing struct{}

var null = nothing{}

type cpusetManagerV2 struct {
	ctx    context.Context
	logger hclog.Logger

	parent    string        // relative to cgroup root (e.g. "nomad.slice")
	parentAbs string        // absolute path (e.g. "/sys/fs/cgroup/nomad.slice")
	initial   cpuset.CPUSet // set of initial cores (never changes)

	lock      sync.RWMutex                 // hold this with regard to tracking fields
	pool      cpuset.CPUSet                // cores being shared among across all tasks
	sharing   map[identifier]nothing       // sharing tasks which use only cpus in the pool
	isolating map[identifier]cpuset.CPUSet // remember which task goes where to avoid context switching
}

func NewCpusetManagerV2(parent string, logger hclog.Logger) CpusetManager {
	cgroupParent := v2GetParent(parent)
	return &cpusetManagerV2{
		ctx:       context.TODO(),
		parent:    cgroupParent,
		parentAbs: filepath.Join(v2CgroupRoot, cgroupParent),
		logger:    logger,
		sharing:   make(map[identifier]nothing),
		isolating: make(map[identifier]cpuset.CPUSet),
	}
}

func (c *cpusetManagerV2) Init(cores []uint16) error {
	c.logger.Info("initializing with", "cores", cores)
	if err := c.ensureParent(); err != nil {
		c.logger.Error("failed to init cpuset manager", "err", err)
		return err
	}
	c.initial = cpuset.New(cores...)
	return nil
}

func (c *cpusetManagerV2) AddAlloc(alloc *structs.Allocation) {
	if alloc == nil || alloc.AllocatedResources == nil {
		return
	}

	c.logger.Trace("add allocation", "name", alloc.Name, "id", alloc.ID)

	// grab write lock while we recompute
	c.lock.Lock()

	// first update our tracking of isolating and sharing tasks
	for task, resources := range alloc.AllocatedResources.Tasks {
		id := makeID(alloc.ID, task)
		if len(resources.Cpu.ReservedCores) > 0 {
			c.isolating[id] = cpuset.New(resources.Cpu.ReservedCores...)
		} else {
			c.sharing[id] = null
		}
	}

	// recompute the available sharable cpu cores
	c.recalculate()

	// let go of write lock, reconcile only needs read lock
	c.lock.Unlock()

	// now write out the entire cgroups space
	c.reconcile()
}

func (c *cpusetManagerV2) RemoveAlloc(allocID string) {
	c.logger.Info("remove allocation", "id", allocID)

	// grab write lock while we recompute
	c.lock.Lock()

	// remove tasks of allocID from the sharing set
	for id := range c.sharing {
		if strings.HasPrefix(id, allocID) {
			delete(c.sharing, id)
		}
	}

	// remove tasks of allocID from the isolating set
	for id := range c.isolating {
		if strings.HasPrefix(id, allocID) {
			delete(c.isolating, id)
		}
	}

	// recompute available sharable cpu cores
	c.recalculate()

	// let go of write lock, reconcile only needs read lock
	c.lock.Unlock()

	// now write out the entire cgroups space
	c.reconcile()
}

// recalculate the number of cores sharable by non-isolating tasks (and isolating tasks)
//
// must be called while holding c.lock
func (c *cpusetManagerV2) recalculate() {
	remaining := c.initial.Copy()
	for _, set := range c.isolating {
		remaining = remaining.Difference(set)
	}
	c.pool = remaining
}

func (c *cpusetManagerV2) CgroupPathFor(allocID, task string) CgroupPathGetter {
	c.logger.Info("cgroup path for", "id", allocID, "task", task)

	// block until cgroup for allocID.task exists

	return func(ctx context.Context) (string, error) {
		ticks, cancel := helper.NewSafeTimer(100 * time.Millisecond)
		defer cancel()

		for {
			path := c.pathOf(makeID(allocID, task))
			mgr, err := fs2.NewManager(nil, path, v2isRootless)
			if err != nil {
				return "", err
			}

			if mgr.Exists() {
				return path, nil
			}

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-ticks.C:
				continue
			}
		}
	}
}

func (c *cpusetManagerV2) reconcile() {
	c.lock.RLock()
	defer c.lock.RUnlock()

	for id := range c.sharing {
		c.write(id, c.pool)
	}

	for id, set := range c.isolating {
		c.write(id, c.pool.Union(set))
	}

	c.cleanup()
}

// must be called while holding c.lock
func (c *cpusetManagerV2) cleanup() {

	// create a map to lookup ids we know about
	size := len(c.sharing) + len(c.isolating)
	ids := make(map[identifier]nothing, size)
	for id := range c.sharing {
		ids[id] = null
	}
	for id := range c.isolating {
		ids[id] = null
	}

	if err := filepath.WalkDir(c.parentAbs, func(path string, entry os.DirEntry, err error) error {
		// a cgroup is a directory
		if !entry.IsDir() {
			return nil
		}

		dir := filepath.Dir(path)
		base := filepath.Base(path)

		// only manage scopes directly under nomad.slice
		if dir != c.parentAbs || !strings.HasSuffix(base, ".scope") {
			return nil
		}

		// only remove the scope if we do not track it
		id := strings.TrimSuffix(base, ".scope")
		_, exists := ids[id]
		if !exists {
			c.remove(path)
		}

		return nil
	}); err != nil {
		c.logger.Error("failed to cleanup cgroup", "err", err)
	}
}

func (c *cpusetManagerV2) pathOf(id string) string {
	return filepath.Join(c.parentAbs, makeScope(id))
}

func (c *cpusetManagerV2) remove(path string) {
	mgr, err := fs2.NewManager(nil, path, v2isRootless)
	if err != nil {
		c.logger.Warn("failed to create manager", "path", path, "err", err)
		return
	}

	// get the list of pids managed by this scope (should be 0 or 1)
	pids, _ := mgr.GetPids()

	// do not destroy the scope if a PID is still present
	// this is a normal condition when an agent restarts with running tasks
	if len(pids) > 0 {
		return
	}

	// remove the cgroup
	if err3 := mgr.Destroy(); err3 != nil {
		c.logger.Warn("failed to cleanup cgroup", "path", path, "err", err)
		return
	}
}

func (c *cpusetManagerV2) write(id string, set cpuset.CPUSet) {
	path := c.pathOf(id)

	// make a manager for the cgroup
	m, err := fs2.NewManager(nil, path, v2isRootless)
	if err != nil {
		c.logger.Error("failed to manage cgroup", "path", path, "err", err)
	}

	// create the cgroup
	if err = m.Apply(v2CreationPID); err != nil {
		c.logger.Error("failed to apply cgroup", "path", path, "err", err)
	}

	// set the cpuset value for the cgroup
	if err = m.Set(&configs.Resources{
		CpusetCpus: set.String(),
	}); err != nil {
		c.logger.Error("failed to set cgroup", "path", path, "err", err)
	}
}

// ensureParentCgroup will create parent cgroup for the manager if it does not
// exist yet. No PIDs are added to any cgroup yet.
func (c *cpusetManagerV2) ensureParent() error {
	mgr, err := fs2.NewManager(nil, c.parentAbs, v2isRootless)
	if err != nil {
		return err
	}

	if err = mgr.Apply(v2CreationPID); err != nil {
		return err
	}

	c.logger.Debug("established initial cgroup hierarchy", "parent", c.parent)
	return nil
}

func v2Root(group string) string {
	return filepath.Join(v2CgroupRoot, group)
}

func v2GetCPUsFromCgroup(group string) ([]uint16, error) {
	path := v2Root(group)
	effective, err := cgroups.ReadFile(path, "cpuset.cpus.effective")
	if err != nil {
		return nil, err
	}
	set, err := cpuset.Parse(effective)
	if err != nil {
		return nil, err
	}
	return set.ToSlice(), nil
}

func v2GetParent(parent string) string {
	if parent == "" {
		return V2defaultCgroupParent
	}
	return parent
}
