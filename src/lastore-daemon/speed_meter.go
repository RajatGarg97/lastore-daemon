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
	"time"
)

type SpeedMeter struct {
	DownloadSize int64

	speed      int64
	updateTime time.Time
	startTime  time.Time

	inited   int
	progress float64
	count    int
}

func (s *SpeedMeter) SetDownloadSize(size int64) {
	if s.DownloadSize == 0 {
		s.DownloadSize = size
	}
}

func (s *SpeedMeter) Speed(newProgress float64) int64 {
	now := time.Now()

	if s.startTime.IsZero() {
		s.startTime = now
	}

	elapsed := now.Sub(s.startTime).Seconds()
	if elapsed < 1 {
		s.updateTime = now
		s.progress = newProgress
		return 0
	}

	if now.Sub(s.updateTime).Seconds() > 3 {
		s.speed = int64(1.024 * (newProgress - s.progress) * float64(s.DownloadSize) / now.Sub(s.updateTime).Seconds())
		s.updateTime = now
		s.progress = newProgress
	}
	return s.speed
}
