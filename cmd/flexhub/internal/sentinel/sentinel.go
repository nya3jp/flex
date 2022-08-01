package sentinel

import (
	"context"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/compute/metadata"
	taskspb "google.golang.org/genproto/googleapis/cloud/tasks/v2"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	delay    = 3 * time.Second
	deadline = 30 * time.Minute
)

type Sentinel struct {
	client     *cloudtasks.Client
	email      string
	queuePath  string
	flexletURL string
}

func New(ctx context.Context, queuePath, flexletURL string) (*Sentinel, error) {
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	email, err := metadata.Email("default")
	if err != nil {
		return nil, err
	}
	return &Sentinel{
		client:     client,
		email:      email,
		queuePath:  queuePath,
		flexletURL: flexletURL,
	}, nil
}

func (q *Sentinel) Enqueue(ctx context.Context, jobId int64) (*taskspb.Task, error) {
	req := &taskspb.CreateTaskRequest{
		Parent: q.queuePath,
		Task: &taskspb.Task{
			ScheduleTime:     timestamppb.New(time.Now().Add(delay)),
			DispatchDeadline: durationpb.New(deadline),
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        q.flexletURL,
					AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
						OidcToken: &taskspb.OidcToken{
							ServiceAccountEmail: q.email,
						},
					},
				},
			},
		},
	}
	return q.client.CreateTask(ctx, req)
}
