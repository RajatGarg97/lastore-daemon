/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package main

import (
	"fmt"
	log "github.com/cihub/seelog"
	"internal/system"
	"pkg.deepin.io/lib/dbus"
)

// StartSystemJob start job
// 1. Dispatch Job by type
// 2. Check whether the work queue is empty
func StartSystemJob(sys system.System, j *Job) error {
	if j == nil {
		panic("StartSystemJob with nil")
	}

	if err := TransitionJobState(j, system.RunningStatus); err != nil {
		return err
	}

	switch j.Type {
	case system.DownloadJobType:
		return sys.Download(j.Id, j.Packages)

	case system.InstallJobType:
		return sys.Install(j.Id, j.Packages)

	case system.DistUpgradeJobType:
		return sys.DistUpgrade(j.Id)

	case system.RemoveJobType:
		return sys.Remove(j.Id, j.Packages)

	case system.UpdateSourceJobType:
		return sys.UpdateSource(j.Id)

	case system.UpdateJobType:
		return sys.Install(j.Id, j.Packages)
	default:
		return system.NotFoundError
	}
}

func ValidTransitionJobState(from system.Status, to system.Status) bool {
	validtion := map[system.Status][]system.Status{
		system.ReadyStatus: []system.Status{
			system.RunningStatus,
			system.PausedStatus,
		},
		system.RunningStatus: []system.Status{
			system.FailedStatus,
			system.SucceedStatus,
			system.PausedStatus,
		},
		system.FailedStatus: []system.Status{
			system.ReadyStatus,
			system.EndStatus,
		},
		system.SucceedStatus: []system.Status{
			system.EndStatus,
		},
		system.PausedStatus: []system.Status{
			system.ReadyStatus,
			system.EndStatus,
		},
	}

	tos, ok := validtion[from]
	if !ok {
		return false
	}
	for _, v := range tos {
		if v == to {
			return true
		}
	}
	return false
}

func TransitionJobState(j *Job, to system.Status) error {
	if !ValidTransitionJobState(j.Status, to) {
		return fmt.Errorf("Can't transition the status of Job %v to %q", j, to)
	}
	log.Infof("%q transition state from %q to %q (Cancelable:%v)\n", j.Id, j.Status, to, j.Cancelable)

	j.Status = to

	if j.Status == system.FailedStatus && j.retry > 0 {
		return nil
	}

	dbus.NotifyChange(j, "Status")

	if j.Status == system.SucceedStatus {
		return TransitionJobState(j, system.EndStatus)
	}
	return nil
}
