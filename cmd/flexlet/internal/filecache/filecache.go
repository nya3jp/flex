// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filecache

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

type Manager struct {
	dir string
}

func NewManager(dir string) *Manager {
	return &Manager{dir}
}

func (m *Manager) Open(key string, create func(w io.Writer) error) (io.ReadCloser, error) {
	cachePath := filepath.Join(m.dir, sha256sum(key))

	lock, err := os.OpenFile(cachePath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	defer lock.Close()

	dupFile := func() (*os.File, error) {
		nfd, err := syscall.Dup(int(lock.Fd()))
		if err != nil {
			return nil, err
		}
		f := os.NewFile(uintptr(nfd), lock.Name())
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			f.Close()
			return nil, err
		}
		return f, nil
	}

	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_SH); err != nil {
		return nil, err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	if pos, err := lock.Seek(0, io.SeekEnd); err != nil {
		return nil, err
	} else if pos > 0 {
		// TODO: Deal with 0-byte case.
		return dupFile()
	}

	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}

	if pos, err := lock.Seek(0, io.SeekEnd); err != nil {
		return nil, err
	} else if pos > 0 {
		// TODO: Deal with 0-byte case.
		return dupFile()
	}

	w, err := dupFile()
	if err != nil {
		return nil, err
	}
	defer w.Close()

	if err := create(w); err != nil {
		lock.Truncate(0)
		return nil, err
	}

	return dupFile()
}

func sha256sum(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}
