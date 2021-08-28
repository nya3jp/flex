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

package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"strconv"
	"sync"

	"github.com/alessio/shellescape"
	"github.com/julienschmidt/httprouter"
	"github.com/nya3jp/flex"
)

//go:embed templates
var templatesFS embed.FS

var funcs = template.FuncMap{
	"shellquote": shellescape.QuoteCommand,
}

var templateIndex = template.Must(template.New("index.html").Funcs(funcs).ParseFS(templatesFS, "templates/index.html", "templates/base.html"))
var templateJobs = template.Must(template.New("jobs.html").Funcs(funcs).ParseFS(templatesFS, "templates/jobs.html", "templates/base.html"))
var templateJob = template.Must(template.New("job.html").Funcs(funcs).ParseFS(templatesFS, "templates/job.html", "templates/base.html"))
var templateFlexlets = template.Must(template.New("flexlets.html").Funcs(funcs).ParseFS(templatesFS, "templates/flexlets.html", "templates/base.html"))

type section string

const (
	sectionIndex    section = "index"
	sectionJobs     section = "jobs"
	sectionFlexlets section = "flexlets"
)

type baseValues struct {
	Section section
	HubURL  string
}

type indexValues struct {
	Base       baseValues
	Stats      *flex.Stats
	TotalCores int32
	HubURL     string
}

type jobsValues struct {
	Base            baseValues
	Jobs            []*flex.JobStatus
	NextBeforeJobID int64
}

type jobValues struct {
	Base        baseValues
	Job         *flex.JobStatus
	Stdout      string
	StdoutError string
	Stderr      string
	StderrError string
}

type flexletsValues struct {
	Base     baseValues
	Flexlets []*flex.FlexletStatus
}

func respond(w http.ResponseWriter, r *http.Request, f func(ctx context.Context) error) {
	if err := f(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderHTML(w http.ResponseWriter, tmpl *template.Template, values interface{}) {
	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, values); err != nil {
		fmt.Fprintf(w, "ERROR: %v", err)
	}
}

func readJobOutput(ctx context.Context, cl flex.FlexServiceClient, id int64, t flex.GetJobOutputRequest_JobOutputType) (string, error) {
	rres, err := cl.GetJobOutput(ctx, &flex.GetJobOutputRequest{Id: id, Type: t})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rres.GetLocation().GetPresignedUrl(), nil)
	if err != nil {
		return "", err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type server struct {
	cl     flex.FlexServiceClient
	hubURL string
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	respond(w, r, func(ctx context.Context) error {
		res, err := s.cl.GetStats(ctx, &flex.GetStatsRequest{})
		if err != nil {
			return err
		}

		stats := res.GetStats()
		values := &indexValues{
			Base:       baseValues{Section: sectionIndex, HubURL: s.hubURL},
			Stats:      stats,
			TotalCores: stats.GetFlexlet().GetIdleCores() + stats.GetFlexlet().GetBusyCores(),
		}
		renderHTML(w, templateIndex, values)
		return nil
	})
}

func (s *server) handleJobs(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	respond(w, r, func(ctx context.Context) error {
		query := r.URL.Query()
		beforeJobID := int64(math.MaxInt64)
		if i, err := strconv.ParseInt(query.Get("before"), 10, 64); err == nil {
			beforeJobID = i
		}

		res, err := s.cl.ListJobs(ctx, &flex.ListJobsRequest{Limit: 100, BeforeId: beforeJobID})
		if err != nil {
			return err
		}

		jobs := res.GetJobs()
		var nextBeforeJobID int64
		if len(jobs) > 0 {
			nextBeforeJobID = jobs[len(jobs)-1].GetJob().GetId()
		}
		values := &jobsValues{
			Base:            baseValues{Section: sectionJobs, HubURL: s.hubURL},
			Jobs:            jobs,
			NextBeforeJobID: nextBeforeJobID,
		}
		renderHTML(w, templateJobs, values)
		return nil
	})
}

func (s *server) handleJob(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	respond(w, r, func(ctx context.Context) error {
		jobID, err := strconv.ParseInt(p.ByName("jobID"), 10, 64)
		if err != nil {
			return fmt.Errorf("invalid job ID: %w", err)
		}

		res, err := s.cl.GetJob(ctx, &flex.GetJobRequest{Id: jobID})
		if err != nil {
			return err
		}
		job := res.GetJob()

		var stdout, stdoutError, stderr, stderrError string
		if job.GetState() == flex.JobState_FINISHED {
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				s, err := readJobOutput(ctx, s.cl, jobID, flex.GetJobOutputRequest_STDOUT)
				if err != nil {
					stdoutError = fmt.Sprintf("Failed to load stdout: %v", err)
				} else {
					stdout = s
				}
			}()
			go func() {
				defer wg.Done()
				s, err := readJobOutput(ctx, s.cl, jobID, flex.GetJobOutputRequest_STDERR)
				if err != nil {
					stderrError = fmt.Sprintf("Failed to load stderr: %v", err)
				} else {
					stderr = s
				}
			}()
			wg.Wait()
		}

		values := &jobValues{
			Base:        baseValues{Section: sectionJobs, HubURL: s.hubURL},
			Job:         job,
			Stdout:      stdout,
			StdoutError: stdoutError,
			Stderr:      stderr,
			StderrError: stderrError,
		}
		renderHTML(w, templateJob, values)
		return nil
	})
}

func (s *server) handleFlexlets(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	respond(w, r, func(ctx context.Context) error {
		res, err := s.cl.ListFlexlets(ctx, &flex.ListFlexletsRequest{})
		if err != nil {
			return err
		}

		values := &flexletsValues{
			Base:     baseValues{Section: sectionFlexlets, HubURL: s.hubURL},
			Flexlets: res.GetFlexlets(),
		}
		renderHTML(w, templateFlexlets, values)
		return nil
	})
}

func newRouter(cl flex.FlexServiceClient, hubURL string) *httprouter.Router {
	srv := &server{
		cl:     cl,
		hubURL: hubURL,
	}
	router := httprouter.New()
	router.GET("/", srv.handleIndex)
	router.GET("/jobs/", srv.handleJobs)
	router.GET("/jobs/:jobID/", srv.handleJob)
	router.GET("/flexlets/", srv.handleFlexlets)
	return router
}
