// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package common

import (
	"fmt"

	"github.com/juju/core/state"
	"github.com/juju/core/state/api/params"
)

// StatusSetter implements a common SetStatus method for use by
// various facades.
type StatusSetter struct {
	st           state.EntityFinder
	getCanModify GetAuthFunc
}

// NewStatusSetter returns a new StatusSetter. The GetAuthFunc will be
// used on each invocation of SetStatus to determine current
// permissions.
func NewStatusSetter(st state.EntityFinder, getCanModify GetAuthFunc) *StatusSetter {
	return &StatusSetter{
		st:           st,
		getCanModify: getCanModify,
	}
}

func (s *StatusSetter) setEntityStatus(tag string, status params.Status, info string, data params.StatusData) error {
	entity0, err := s.st.FindEntity(tag)
	if err != nil {
		return err
	}
	entity, ok := entity0.(state.StatusSetter)
	if !ok {
		return NotSupportedError(tag, "setting status")
	}
	return entity.SetStatus(status, info, data)
}

// SetStatus sets the status of each given entity.
func (s *StatusSetter) SetStatus(args params.SetStatus) (params.ErrorResults, error) {
	result := params.ErrorResults{
		Results: make([]params.ErrorResult, len(args.Entities)),
	}
	if len(args.Entities) == 0 {
		return result, nil
	}
	canModify, err := s.getCanModify()
	if err != nil {
		return params.ErrorResults{}, err
	}
	for i, arg := range args.Entities {
		err := ErrPerm
		if canModify(arg.Tag) {
			err = s.setEntityStatus(arg.Tag, arg.Status, arg.Info, arg.Data)
		}
		result.Results[i].Error = ServerError(err)
	}
	return result, nil
}

func (s *StatusSetter) updateEntityStatusData(tag string, data params.StatusData) error {
	entity0, err := s.st.FindEntity(tag)
	if err != nil {
		return err
	}
	statusGetter, ok := entity0.(state.StatusGetter)
	if !ok {
		return NotSupportedError(tag, "getting status")
	}
	existingStatus, existingInfo, existingData, err := statusGetter.Status()
	if err != nil {
		return err
	}
	newData := existingData
	if newData == nil {
		newData = data
	} else {
		for k, v := range data {
			newData[k] = v
		}
	}
	entity, ok := entity0.(state.StatusSetter)
	if !ok {
		return NotSupportedError(tag, "updating status")
	}
	if len(newData) > 0 && existingStatus != params.StatusError {
		return fmt.Errorf("machine %q is not in an error state", tag)
	}
	return entity.SetStatus(existingStatus, existingInfo, newData)
}

// UpdateStatus updates the status data of each given entity.
func (s *StatusSetter) UpdateStatus(args params.SetStatus) (params.ErrorResults, error) {
	result := params.ErrorResults{
		Results: make([]params.ErrorResult, len(args.Entities)),
	}
	if len(args.Entities) == 0 {
		return result, nil
	}
	canModify, err := s.getCanModify()
	if err != nil {
		return params.ErrorResults{}, err
	}
	for i, arg := range args.Entities {
		err := ErrPerm
		if canModify(arg.Tag) {
			err = s.updateEntityStatusData(arg.Tag, arg.Data)
		}
		result.Results[i].Error = ServerError(err)
	}
	return result, nil
}
