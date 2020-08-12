// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package haproxy

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
)

type Maps interface {
	AppendRow(key uint64, row string)
	Clean()
	Refresh() (reload bool, err error)
}

type mapFiles map[uint64]*mapFile

var mapDir string

type mapFile struct {
	rows   map[string]bool
	hasNew bool
}

func (mf *mapFile) getContent() string {
	var content strings.Builder
	for r, removed := range mf.rows {
		if removed {
			continue
		}
		content.WriteString(r)
		content.WriteRune('\n')
	}
	return content.String()
}

func (mf *mapFile) isModified() bool {
	for _, removed := range mf.rows {
		if removed {
			return true
		}
	}
	return mf.hasNew
}

func NewMapFiles(path string) Maps {
	mapDir = path
	var maps mapFiles = make(map[uint64]*mapFile)
	return &maps
}

func (m *mapFiles) AppendRow(key uint64, row string) {
	if row == "" {
		return
	}
	if (*m)[key] == nil {
		(*m)[key] = &mapFile{
			rows: make(map[string]bool),
		}
	}
	if _, ok := (*m)[key].rows[row]; !ok {
		(*m)[key].hasNew = true
	}
	(*m)[key].rows[row] = false
}

func (m *mapFiles) Clean() {
	for _, mapFile := range *m {
		for key, removed := range mapFile.rows {
			if removed {
				delete(mapFile.rows, key)
				continue
			}
			mapFile.rows[key] = true
		}
		mapFile.hasNew = false
	}
}

type mapRefreshError struct {
	error
}

func (m *mapRefreshError) add(nErr error) {
	if nErr == nil {
		return
	}
	if m.error == nil {
		m.error = nErr
		return
	}
	m.error = fmt.Errorf("%w\n%s", m.error, nErr)
}

func (m *mapFiles) Refresh() (reload bool, err error) {
	reload = false
	var retErr mapRefreshError
	for key, mapFile := range *m {
		if mapFile.isModified() {
			content := mapFile.getContent()
			var f *os.File
			filename := path.Join(mapDir, strconv.FormatUint(key, 10)) + ".lst"
			if content == "" {
				rErr := os.Remove(filename)
				retErr.add(rErr)
				delete(*m, key)
				continue
			} else if f, err = os.Create(filename); err != nil {
				retErr.add(err)
				continue
			}
			defer f.Close()
			if _, err = f.WriteString(content); err != nil {
				return reload, err
			}
			reload = true
		}
	}
	return reload, retErr.error
}
