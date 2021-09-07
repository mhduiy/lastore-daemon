/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package system

import (
	"path"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type DirMonitorChangeType string

type DirMonitor struct {
	sync.Mutex
	done    chan bool
	watcher *fsnotify.Watcher

	callbacks map[string]DirMonitorCallback
	baseDir   string
}

type DirMonitorCallback func(fpath string)

func (f *DirMonitor) Add(fn DirMonitorCallback, names ...string) error {
	f.Lock()
	for _, name := range names {
		fpath := path.Join(f.baseDir, name)
		if _, ok := f.callbacks[fpath]; ok {
			return ResourceExitError
		}
		f.callbacks[fpath] = fn
	}
	f.Unlock()
	return nil
}

func NewDirMonitor(baseDir string) *DirMonitor {
	return &DirMonitor{
		baseDir:   baseDir,
		done:      make(chan bool),
		callbacks: make(map[string]DirMonitorCallback),
	}
}

func (f *DirMonitor) Start() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	f.Lock()
	if f.watcher != nil {
		f.watcher.Close()
	}
	f.watcher = w
	f.Unlock()

	err = f.watcher.Add(f.baseDir)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event := <-f.watcher.Events:
				f.tryNotify(event)
			case err := <-f.watcher.Errors:
				logger.Warning(err)
			case <-f.done:
				goto end
			}
		}
	end:
	}()
	return nil
}

func (f *DirMonitor) tryNotify(event fsnotify.Event) {
	f.Lock()
	defer f.Unlock()

	fpath := event.Name
	fn, ok := f.callbacks[fpath]
	if !ok {
		return
	}

	if (event.Op&fsnotify.Remove == fsnotify.Remove) || (event.Op&fsnotify.Chmod == fsnotify.Chmod) || (event.Op&fsnotify.Write == fsnotify.Write) {
		fn(fpath)
	}
}

func (f *DirMonitor) Stop() {
	f.done <- true

	f.Lock()
	if f.watcher != nil {
		f.watcher.Close()
		f.watcher = nil
	}
	f.Unlock()
}
