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
	"fmt"
	"io"
	"strings"

	"github.com/alessio/shellescape"

	"github.com/nya3jp/flex"
)

type Text struct {
	w io.Writer
}

func NewText(w io.Writer) *Text {
	return &Text{w: w}
}

func (f *Text) JobStatus(jobStatus *flex.JobStatus) {
	spec := jobStatus.GetJob().GetSpec()
	fmt.Fprintf(f.w, "Job ID: %d\n", jobStatus.GetJob().GetId())
	fmt.Fprintf(f.w, "Command: %s\n", shellescape.QuoteCommand(spec.GetCommand().GetArgs()))
	for _, pkg := range spec.GetInputs().GetPackages() {
		var name string
		if tag := pkg.GetTag(); tag != "" {
			name = fmt.Sprintf("%s(%s)", tag, pkg.GetHash())
		} else {
			name = pkg.GetHash()
		}
		fmt.Fprintf(f.w, "Package: %s at %s\n", name, "/"+strings.TrimLeft(pkg.GetInstallDir(), "/"))
	}
	fmt.Fprintf(f.w, "Priority: %d\n", spec.GetConstraints().GetPriority())
	fmt.Fprintf(f.w, "Time Limit: %s\n", spec.GetLimits().GetTime().AsDuration().String())
	fmt.Fprintf(f.w, "Labels: %s\n", strings.Join(spec.GetAnnotations().GetLabels(), ", "))
	fmt.Fprintf(f.w, "State: %s\n", jobStatus.GetState().String())
	fmt.Fprintf(f.w, "Task ID: %s\n", jobStatus.GetTaskId())
	fmt.Fprintf(f.w, "Assigned Flexlet Name: %s\n", jobStatus.GetFlexletName())
	if res := jobStatus.GetResult(); res != nil {
		fmt.Fprintf(f.w, "Execution Result: %s\n", res.GetMessage())
		fmt.Fprintf(f.w, "Execution Time: %d\n", res.GetTime().AsDuration())
		fmt.Fprintf(f.w, "Exit Code: %d\n", res.GetExitCode())
	}
	fmt.Fprintf(f.w, "Created Time: %s\n", jobStatus.GetCreated().AsTime().String())
	fmt.Fprintf(f.w, "Started Time: %s\n", jobStatus.GetStarted().AsTime().String())
	fmt.Fprintf(f.w, "Finished Time: %s\n", jobStatus.GetFinished().AsTime().String())
}

func (f *Text) JobStatuses(jobStatuses []*flex.JobStatus) {
	for _, jobStatus := range jobStatuses {
		state := jobStatus.GetState().String()
		if res := jobStatus.GetResult(); res != nil {
			state = fmt.Sprintf("%s(%d)", state, res.GetExitCode())
		}
		fmt.Fprintf(f.w, "%d\t%s\t%s\n", jobStatus.GetJob().GetId(), state, shellescape.QuoteCommand(jobStatus.GetJob().GetSpec().GetCommand().GetArgs()))
	}
}

func (f *Text) Package(pkg *flex.Package) {
	fmt.Fprintf(f.w, "%s\n", pkg.GetHash())
}

func (f *Text) Tag(tag *flex.Tag) {
	fmt.Fprintf(f.w, "%s\t%s\n", tag.GetName(), tag.GetHash())
}

func (f *Text) Tags(tags []*flex.Tag) {
	for _, tag := range tags {
		f.Tag(tag)
	}
}
