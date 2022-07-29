package flexletutil

import (
	"context"
	"log"
	"time"

	"github.com/nya3jp/flex"
	"github.com/nya3jp/flex/internal/ctxutil"
	"github.com/nya3jp/flex/internal/flexletpb"
)

func RunFletletUpdater(ctx context.Context, cl flexletpb.FlexletServiceClient, flexlet *flex.Flexlet) error {
	for {
		status := &flex.FlexletStatus{
			Flexlet: flexlet,
			State:   flex.FlexletState_ONLINE,
		}
		if _, err := cl.UpdateFlexlet(ctx, &flexletpb.UpdateFlexletRequest{Status: status}); err != nil && ctx.Err() == nil {
			log.Printf("WARNING: UpdateTasklet failed: %v", err)
		}
		if err := ctxutil.Sleep(ctx, 10*time.Second); err != nil {
			return err
		}
	}
}

func RunTaskUpdater(ctx context.Context, cl flexletpb.FlexletServiceClient, ref *flexletpb.TaskRef) error {
	for {
		if _, err := cl.UpdateTask(ctx, &flexletpb.UpdateTaskRequest{Ref: ref}); err != nil && ctx.Err() == nil {
			log.Printf("WARNING: UpdateTask failed: %v", err)
		}
		if err := ctxutil.Sleep(ctx, 10*time.Second); err != nil {
			return err
		}
	}
}
