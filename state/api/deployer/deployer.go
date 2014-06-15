// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package deployer

import (
	"github.com/juju/juju/state/api/base"
	"github.com/juju/juju/state/api/common"
	"github.com/juju/juju/state/api/params"
)

const deployerFacade = "Deployer"

// State provides access to the deployer worker's idea of the state.
type State struct {
	base.FacadeCaller
	*common.APIAddresser
}

// NewState creates a new State instance that makes API calls
// through the given caller.
func NewState(caller base.Caller) *State {
	facadeCaller := base.GetFacadeCaller(caller, deployerFacade)
	return &State{
		FacadeCaller: facadeCaller,
		APIAddresser: common.NewAPIAddresser(facadeCaller),
	}

}

// unitLife returns the lifecycle state of the given unit.
func (st *State) unitLife(tag string) (params.Life, error) {
	return common.Life(st.RawCaller(), deployerFacade, tag)
}

// Unit returns the unit with the given tag.
func (st *State) Unit(tag string) (*Unit, error) {
	life, err := st.unitLife(tag)
	if err != nil {
		return nil, err
	}
	return &Unit{
		tag:  tag,
		life: life,
		st:   st,
	}, nil
}

// Machine returns the machine with the given tag.
func (st *State) Machine(tag string) (*Machine, error) {
	return &Machine{
		tag: tag,
		st:  st,
	}, nil
}

// StateAddresses returns the list of addresses used to connect to the state.
func (st *State) StateAddresses() ([]string, error) {
	var result params.StringsResult
	err := st.APICall("StateAddresses", nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Result, nil
}

// ConnectionInfo returns all the address information that the deployer task
// needs in one call.
func (st *State) ConnectionInfo() (result params.DeployerConnectionValues, err error) {
	err = st.APICall("ConnectionInfo", nil, &result)
	return result, err
}
