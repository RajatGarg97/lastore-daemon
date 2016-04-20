/**
 * Copyright (C) 2015 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/
package main

import "net/http"
import "time"
import "encoding/json"
import "fmt"
import "github.com/apcera/termtables"
import "strings"
import "io"
import "internal/utils"

type URLChecker struct {
	workQueue   chan string
	workerQueue chan chan string
	result      map[string]chan *URLCheckResult
	nDone       int
}
type URLCheckResult struct {
	URL        string
	Result     bool
	ResultCode int
	StartTime  time.Time
	Latency    time.Duration
}

func (c *URLChecker) Check(urls ...string) {
	for _, url := range urls {
		c.result[url] = make(chan *URLCheckResult, 1)
	}
}

func (c *URLChecker) Result(url string) *URLCheckResult {
	return <-c.result[url]
}

func NewURLChecker(thread int) *URLChecker {
	c := &URLChecker{
		workerQueue: make(chan chan string),
		result:      make(map[string]chan *URLCheckResult),
	}

	for i := 0; i < thread; i++ {
		go func() {
			worker := make(chan string)
			for {
				c.workerQueue <- worker
				select {
				case url := <-worker:
					r := CheckURLExists(url)
					c.nDone = c.nDone + 1
					c.result[url] <- r
					fmt.Printf("\r\n%0.1f%%  %q --> %v %v",
						float64(c.nDone)/float64(len(c.result))*100,
						url, r.Latency, r.Result)
					<-time.After(time.Millisecond * 100)
				}
			}
		}()
	}
	return c
}
func (u *URLChecker) Wait() {
	for url := range u.result {
		worker := <-u.workerQueue
		worker <- url
	}
}

func CheckURLExists(url string) *URLCheckResult {
	n := time.Now().UTC()
	resp, err := http.Get(url)
	if err != nil {
		return &URLCheckResult{url, false, 0, n, time.Since(n)}
	}
	defer resp.Body.Close()

	switch resp.StatusCode / 100 {
	case 4, 5:
		return &URLCheckResult{url, false, resp.StatusCode, n, time.Since(n)}
	case 3, 2, 1:
		return &URLCheckResult{url, true, resp.StatusCode, n, time.Since(n)}
	}
	return &URLCheckResult{url, false, resp.StatusCode, n, time.Since(n)}
}

func ParseIndex(indexUrl string) ([]string, error) {
	resp, err := http.Get(indexUrl)
	if err != nil {
		fmt.Println("E:", resp)
		return nil, err
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)
	var lines []string
	err = d.Decode(&lines)

	return lines, err
}

type MirrorInfo struct {
	Name        string
	Support2014 bool
	Support2015 bool
	Progress    float64
	LastSync    time.Time
	Latency     time.Duration
	Detail      []URLCheckResult
}

func (MirrorInfo) String() {
	fmt.Sprint("%s 2014:%s")
}

func SaveMirrorInfos(infos []MirrorInfo, w io.Writer) error {
	return json.NewEncoder(w).Encode(infos)
}

func ShowMirrorInfos(infos []MirrorInfo) {
	termtables.EnableUTF8PerLocale()

	t := termtables.CreateTable()
	t.AddHeaders("Name", "2014", "Latency", "2015", "LastSync")
	t.AddTitle(fmt.Sprintf("Report at %v", time.Now()))

	sym := map[bool]string{
		true:  "✔",
		false: "✖",
	}
	for _, info := range infos {
		name := info.Name
		if len(name) > 47 {
			name = name[0:47] + "..."
		}
		var lm string = info.LastSync.Format(time.ANSIC)
		if info.LastSync.IsZero() {
			lm = "?"
		}
		t.AddRow(name,
			sym[info.Support2014],
			fmt.Sprintf("%5.0fms", info.Latency.Seconds()*1000),
			fmt.Sprintf("%7.0f%%", info.Progress*100),
			lm,
		)
	}

	fmt.Println("\n")
	fmt.Println(t.Render())
}

func u2014(server string) string {
	return utils.AppendSuffix(server, "/") + "dists/trusty/Release"
}
func u2015(server string) string {
	return utils.AppendSuffix(server, "/") + "dists/unstable/Release"
}
func uGuards(server string, guards []string) []string {
	var r []string
	// Just need precise of 5%. (Currently has 1%)
	for i, g := range guards {
		if i%5 == 0 {
			r = append(r, utils.AppendSuffix(server, "/")+g)
		}
	}
	return r
}

func DetectServer(parallel int, indexName string, official string, mlist []string) []MirrorInfo {
	indexUrl := utils.AppendSuffix(official, "/") + indexName
	index, err := ParseIndex(indexUrl)
	if err != nil || len(index) == 0 {
		fmt.Println("E:", err)
		return nil
	}
	mlist = append([]string{official}, mlist...)

	checker := NewURLChecker(parallel)
	for _, s := range mlist {
		checker.Check(u2014(s))
		checker.Check(u2015(s))
		checker.Check(uGuards(s, index)...)
	}
	checker.Wait()

	var infos []MirrorInfo
	for _, s := range mlist {
		info := MirrorInfo{
			Name:     s,
			LastSync: fetchLastSync(utils.AppendSuffix(s, "/") + indexName),
		}
		p := 0
		guards := uGuards(s, index)
		var latency time.Duration
		for _, u := range guards {
			r := checker.Result(u)
			if r.Result {
				p = p + 1
			}
			if strings.HasPrefix(r.URL, s) {
				info.Detail = append(info.Detail, *r)
			}
			latency = latency + r.Latency
		}

		info.Progress = float64(p) / float64(len(guards))
		info.Support2014 = checker.Result(u2014(s)).Result
		info.Support2015 = checker.Result(u2015(s)).Result
		info.Latency = time.Duration(int64(latency.Nanoseconds() / int64(len(guards))))
		infos = append(infos, info)
	}
	return infos
}

func fetchLastSync(url string) time.Time {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("E:", resp)
		return time.Time{}
	}
	defer resp.Body.Close()
	t, e := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	if e != nil {
		fmt.Println("\nfetchLastSync:", url, e)
	}
	return t
}
