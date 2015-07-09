// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"github.com/juju/errors"
	"github.com/juju/names"

	"github.com/juju/juju/process"
)

// TODO(ericsnow) Track juju-level status in the status collection.

// TODO(ericsnow) Rename Register to Add and Unregister to Remove.

// TODO(ericsnow) Distinguish between marking as unregistered and
// removing a proc from state?

// UnitProcesses exposes high-level interaction with workload processes
// for a unit.
type UnitProcesses interface {
	// Register registers a workload process in state. If a process is
	// already registered for the same (unit, proc name, plugin ID)
	// then the request will fail. The unit must also be "alive".
	Register(info process.Info) error
	// SetStatus sets the raw status of a workload process. The process
	// ID is in the format provided by process.Info.ID()
	// ("<proc name>/<plugin ID>"). If the process is not in state then
	// the request will fail.
	SetStatus(id string, status process.PluginStatus) error
	// List builds the list of workload processes registered for
	// the given unit and IDs. If no IDs are provided then all
	// registered processes for the unit are returned. In the case that
	// IDs are provided, any that are not in state are ignored and only
	// the found ones are returned. It is up to the caller to
	// extrapolate the list of missing IDs.
	List(ids ...string) ([]process.Info, error)
	// Unregister marks the identified process as unregistered. If the
	// given ID is not in state then the request will fail.
	Unregister(id string) error
}

// TODO(ericsnow) Use a more generic component registration mechanism?

type newUnitProcessesFunc func(persist Persistence, unit names.UnitTag) (UnitProcesses, error)

var (
	newUnitProcesses newUnitProcessesFunc
)

// SetProcessesComponent registers the functions that provide the state
// functionality related to workload processes.
func SetProcessesComponent(upFunc newUnitProcessesFunc) {
	newUnitProcesses = upFunc
}

type unitProcesses struct {
	UnitProcesses
	st *State
}

// UnitProcesses exposes interaction with workload processes in state
// for a the given unit.
func (st *State) UnitProcesses(unit *Unit) (UnitProcesses, error) {
	if newUnitProcesses == nil {
		return nil, errors.Errorf("workload processes not supported")
	}

	persist := st.newPersistence()
	unitProcs, err := newUnitProcesses(persist, unit.UnitTag())
	if err != nil {
		return nil, errors.Trace(err)
	}

	return unitProcs, nil
}
