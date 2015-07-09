// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package persistence

// TODO(ericsnow) Eliminate the mongo-related imports here.

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/names"
	jujutxn "github.com/juju/txn"
	"gopkg.in/mgo.v2/txn"

	"github.com/juju/juju/process"
)

var logger = loggo.GetLogger("juju.process.persistence")

// TODO(ericsnow) Implement persistence using a TXN abstraction (used
// in the business logic) with ops factories available from the
// persistence layer.

// PersistenceBase exposes the core persistence functionality needed
// for workload processes.
type PersistenceBase interface {
	// One populates doc with the document corresponding to the given
	// ID. Missing documents result in errors.NotFound.
	One(collName, id string, doc interface{}) error
	// All populates docs with the list of the documents corresponding
	// to the provided query.
	All(collName string, query, docs interface{}) error
	// Run runs the transaction generated by the provided factory
	// function. It may be retried several times.
	Run(transactions jujutxn.TransactionSource) error
}

// Persistence exposes the high-level persistence functionality
// related to workload processes in Juju.
type Persistence struct {
	st   PersistenceBase
	unit names.UnitTag
}

// NewPersistence builds a new Persistence based on the provided info.
func NewPersistence(st PersistenceBase, unit names.UnitTag) *Persistence {
	return &Persistence{
		st:   st,
		unit: unit,
	}
}

// Insert adds records for the process to persistence. If the process
// is already there then false gets returned (true if inserted).
// Existing records are not checked for consistency.
func (pp Persistence) Insert(info process.Info) (bool, error) {
	var okay bool
	var ops []txn.Op
	// TODO(ericsnow) Add unitPersistence.newEnsureAliveOp(pp.unit)?
	ops = append(ops, pp.newInsertProcessOps(info)...)
	buildTxn := func(attempt int) ([]txn.Op, error) {
		if attempt > 0 {
			okay = false
			return nil, jujutxn.ErrNoOperations
		}
		okay = true
		return ops, nil
	}
	if err := pp.st.Run(buildTxn); err != nil {
		return false, errors.Trace(err)
	}
	return okay, nil
}

// SetStatus updates the raw status for the identified process in
// persistence. The return value corresponds to whether or not the
// record was found in persistence. Any other problem results in
// an error. The process is not checked for inconsistent records.
func (pp Persistence) SetStatus(id string, status process.PluginStatus) (bool, error) {
	var found bool
	var ops []txn.Op
	// TODO(ericsnow) Add unitPersistence.newEnsureAliveOp(pp.unit)?
	ops = append(ops, pp.newSetRawStatusOps(id, status)...)
	buildTxn := func(attempt int) ([]txn.Op, error) {
		if attempt > 0 {
			found = false
			return nil, jujutxn.ErrNoOperations
		}
		found = true
		return ops, nil
	}
	if err := pp.st.Run(buildTxn); err != nil {
		return false, errors.Trace(err)
	}
	return found, nil
}

// List builds the list of processes found in persistence which match
// the provided IDs. The lists of IDs with missing records is also
// returned.
func (pp Persistence) List(ids ...string) ([]process.Info, []string, error) {
	// TODO(ericsnow) Ensure that the unit is Alive?

	procDocs, err := pp.procs(ids)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	var results []process.Info
	var missing []string
	for _, id := range ids {
		proc, ok := pp.extractProc(id, procDocs)
		if !ok {
			missing = append(missing, id)
			continue
		}
		results = append(results, *proc)
	}
	return results, missing, nil
}

// ListAll builds the list of all processes found in persistence.
// Inconsistent records result in errors.NotValid.
func (pp Persistence) ListAll() ([]process.Info, error) {
	// TODO(ericsnow) Ensure that the unit is Alive?

	procDocs, err := pp.allProcs()
	if err != nil {
		return nil, errors.Trace(err)
	}

	var results []process.Info
	for id := range procDocs {
		proc, _ := pp.extractProc(id, procDocs)
		results = append(results, *proc)
	}
	return results, nil
}

// TODO(ericsnow) Add procs to state/cleanup.go.

// TODO(ericsnow) How to ensure they are completely removed from state?

// Remove removes all records associated with the identified process
// from persistence. Also returned is whether or not the process was
// found. If the records for the process are not consistent then
// errors.NotValid is returned.
func (pp Persistence) Remove(id string) (bool, error) {
	var found bool
	var ops []txn.Op
	// TODO(ericsnow) Add unitPersistence.newEnsureAliveOp(pp.unit)?
	ops = append(ops, pp.newRemoveProcessOps(id)...)
	buildTxn := func(attempt int) ([]txn.Op, error) {
		if attempt > 0 {
			found = false
			return nil, jujutxn.ErrNoOperations
		}
		found = true
		return ops, nil
	}
	if err := pp.st.Run(buildTxn); err != nil {
		return false, errors.Trace(err)
	}
	return found, nil
}
