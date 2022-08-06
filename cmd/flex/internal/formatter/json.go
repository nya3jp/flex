// Copyright 2022 Google LLC
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

package formatter

import (
	"encoding/json"
	"io"

	"github.com/nya3jp/flex"
)

type JSON struct {
	w io.Writer
}

func NewJSON(w io.Writer) *JSON {
	return &JSON{w: w}
}

func (f *JSON) JobStatus(jobStatus *flex.JobStatus) {
	f.encodeJSON(jobStatus)
}

func (f *JSON) JobStatuses(jobStatuses []*flex.JobStatus) {
	if jobStatuses == nil {
		jobStatuses = make([]*flex.JobStatus, 0)
	}
	f.encodeJSON(jobStatuses)
}

func (f *JSON) Package(pkg *flex.Package) {
	f.encodeJSON(pkg)
}

func (f *JSON) Tag(tag *flex.Tag) {
	f.encodeJSON(tag)
}

func (f *JSON) Tags(tags []*flex.Tag) {
	if tags == nil {
		tags = make([]*flex.Tag, 0)
	}
	f.encodeJSON(tags)
}

func (f *JSON) encodeJSON(val interface{}) {
	enc := json.NewEncoder(f.w)
	enc.SetIndent("", "  ")
	enc.Encode(val)
}
