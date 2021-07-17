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

package hashutil

import (
	"encoding/hex"
	"hash"
	"io"
)

type TeeHasher struct {
	w io.Writer
	h hash.Hash
}

func NewTeeHasher(w io.Writer, h hash.Hash) *TeeHasher {
	return &TeeHasher{w, h}
}

func (h *TeeHasher) Write(p []byte) (int, error) {
	h.h.Write(p)
	return h.w.Write(p)
}

func (h *TeeHasher) SumString() string {
	return hex.EncodeToString(h.h.Sum(nil))
}
