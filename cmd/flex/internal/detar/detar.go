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

package detar

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Create(ctx context.Context, out io.Writer, paths ...string) (retErr error) {
	// Resolve dot names first.
	for _, path := range paths {
		if path == "" {
			return errors.New("empty file name")
		}
		name := filepath.Base(path)
		if name == "." || name == ".." {
			subs, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			for _, sub := range subs {
				paths = append(paths, filepath.Join(path, sub.Name()))
			}
		}
	}

	topToPath := make(map[string]string)
	for _, path := range paths {
		top := filepath.Base(path)
		if _, ok := topToPath[top]; ok {
			return fmt.Errorf("duplicated file name: %s", top)
		}
		topToPath[top] = path
	}

	var tops []string
	for top := range topToPath {
		tops = append(tops, top)
	}
	sort.Strings(tops)

	gz := gzip.NewWriter(out)
	defer func() {
		if err := gz.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	tw := tar.NewWriter(gz)
	defer func() {
		if err := tw.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	for _, top := range tops {
		start := topToPath[top]
		strip := filepath.Dir(start)
		if err := filepath.Walk(start, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}

			relPath := strings.TrimLeft(strings.TrimPrefix(path, strip), string(filepath.Separator))
			hdr, err := headerFor(relPath, info)
			if err != nil {
				return err
			}

			logTarHeader(hdr)

			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}

			if hdr.Size == 0 {
				return nil
			}

			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.CopyN(tw, f, hdr.Size)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

func headerFor(path string, fi fs.FileInfo) (*tar.Header, error) {
	var typeFlag byte
	var size int64
	var mode int64
	var link string
	tarPath := path

	switch fi.Mode().Type() {
	case 0:
		typeFlag = tar.TypeReg
		size = fi.Size()
		if fi.Mode()&0100 != 0 {
			mode = 0755
		} else {
			mode = 0644
		}
	case os.ModeDir:
		typeFlag = tar.TypeDir
		mode = 0755
		tarPath += "/"
	case os.ModeSymlink:
		l, err := os.Readlink(path)
		if err != nil {
			return nil, err
		}
		typeFlag = tar.TypeSymlink
		mode = 0755
		link = l
	default:
		return nil, fmt.Errorf("unsupported file type %v", fi.Mode().Type())
	}

	return &tar.Header{
		Typeflag: typeFlag,
		Name:     tarPath,
		Linkname: link,
		Size:     size,
		Mode:     mode,
		Format:   tar.FormatPAX,
	}, nil
}

func logTarHeader(hdr *tar.Header) {
	name := hdr.Name
	mode := os.FileMode(hdr.Mode)
	switch hdr.Typeflag {
	case tar.TypeDir:
		mode |= os.ModeDir
	case tar.TypeSymlink:
		name = fmt.Sprintf("%s -> %s", name, hdr.Linkname)
		mode |= os.ModeSymlink
	}
	log.Printf("%s %10d %s", mode, hdr.Size, name)
}
