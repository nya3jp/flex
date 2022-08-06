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

package restfix

import (
	"errors"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/nya3jp/flex"
)

func JobLimits(limits *flex.JobLimits) error {
	if limits == nil {
		return errors.New("nil JobLimits")
	}
	if limits.Time == nil {
		limits.Time = durationpb.New(0)
	}
	return nil
}

func JobSpec(spec *flex.JobSpec) error {
	if spec == nil {
		return errors.New("nil JobSpec")
	}
	if spec.Command == nil {
		spec.Command = &flex.JobCommand{}
	}
	if spec.Inputs == nil {
		spec.Inputs = &flex.JobInputs{}
	}
	if spec.Limits == nil {
		spec.Limits = &flex.JobLimits{}
	}
	if err := JobLimits(spec.Limits); err != nil {
		return err
	}
	if spec.Constraints == nil {
		spec.Constraints = &flex.JobConstraints{}
	}
	if spec.Annotations == nil {
		spec.Annotations = &flex.JobAnnotations{}
	}
	return nil
}

func Job(job *flex.Job) error {
	if job == nil {
		return errors.New("nil Job")
	}
	if err := JobSpec(job.Spec); err != nil {
		return err
	}
	return nil
}

func JobStatus(job *flex.JobStatus) error {
	if job == nil {
		return errors.New("nil JobStatus")
	}
	if err := Job(job.Job); err != nil {
		return err
	}
	return nil
}

func Flexlet(flexlet *flex.Flexlet) error {
	if flexlet == nil {
		return errors.New("nil Flexlet")
	}
	if flexlet.Spec == nil {
		flexlet.Spec = &flex.FlexletSpec{}
	}
	return nil
}

func FlexletStatus(flexlet *flex.FlexletStatus) error {
	if flexlet == nil {
		return errors.New("nil FlexletStatus")
	}
	if flexlet.Flexlet == nil {
		flexlet.Flexlet = &flex.Flexlet{}
	}
	if err := Flexlet(flexlet.Flexlet); err != nil {
		return err
	}
	for _, job := range flexlet.CurrentJobs {
		if err := Job(job); err != nil {
			return err
		}
	}
	return nil
}
