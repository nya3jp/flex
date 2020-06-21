// Copyright 2020 Google LLC
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

package concurrency

import (
	"context"
	"fmt"
)

type Runner struct {
	fs       []func(ctx context.Context) error
	failFast bool
}

type RunnerOption func(*Runner)

func NewRunner(opts ...RunnerOption) *Runner {
	r := &Runner{}
	for _, o := range opts {
		o(r)
	}
	return r
}

func FailFast(r *Runner) {
	r.failFast = true
}

func (r *Runner) Add(f func(ctx context.Context) error) {
	r.fs = append(r.fs, f)
}

func (r *Runner) Run(ctx context.Context) error {
	n := len(r.fs)
	if n == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan error, n)
	for _, f := range r.fs {
		f := f
		go func() {
			done <- func() (err error) {
				if obj := recover(); obj != nil {
					err = fmt.Errorf("panic: %v", obj)
				}
				return f(ctx)
			}()
		}()
	}

	var timeout <-chan struct{}
	if r.failFast {
		timeout = ctx.Done()
	}

	var firstErr error
	c := 0
	for c < n {
		select {
		case err := <-done:
			if err != nil {
				if r.failFast {
					return err
				}
				if firstErr == nil {
					firstErr = err
				}
			}
			c++
		case <-timeout:
			return ctx.Err()
		}
	}
	return firstErr
}
