// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package flex

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// WorkerClient is the client API for Worker service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type WorkerClient interface {
	RunTask(ctx context.Context, in *RunTaskRequest, opts ...grpc.CallOption) (Worker_RunTaskClient, error)
}

type workerClient struct {
	cc grpc.ClientConnInterface
}

func NewWorkerClient(cc grpc.ClientConnInterface) WorkerClient {
	return &workerClient{cc}
}

func (c *workerClient) RunTask(ctx context.Context, in *RunTaskRequest, opts ...grpc.CallOption) (Worker_RunTaskClient, error) {
	stream, err := c.cc.NewStream(ctx, &Worker_ServiceDesc.Streams[0], "/flex.Worker/RunTask", opts...)
	if err != nil {
		return nil, err
	}
	x := &workerRunTaskClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Worker_RunTaskClient interface {
	Recv() (*RunTaskResponse, error)
	grpc.ClientStream
}

type workerRunTaskClient struct {
	grpc.ClientStream
}

func (x *workerRunTaskClient) Recv() (*RunTaskResponse, error) {
	m := new(RunTaskResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// WorkerServer is the server API for Worker service.
// All implementations must embed UnimplementedWorkerServer
// for forward compatibility
type WorkerServer interface {
	RunTask(*RunTaskRequest, Worker_RunTaskServer) error
	mustEmbedUnimplementedWorkerServer()
}

// UnimplementedWorkerServer must be embedded to have forward compatible implementations.
type UnimplementedWorkerServer struct {
}

func (UnimplementedWorkerServer) RunTask(*RunTaskRequest, Worker_RunTaskServer) error {
	return status.Errorf(codes.Unimplemented, "method RunTask not implemented")
}
func (UnimplementedWorkerServer) mustEmbedUnimplementedWorkerServer() {}

// UnsafeWorkerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to WorkerServer will
// result in compilation errors.
type UnsafeWorkerServer interface {
	mustEmbedUnimplementedWorkerServer()
}

func RegisterWorkerServer(s grpc.ServiceRegistrar, srv WorkerServer) {
	s.RegisterService(&Worker_ServiceDesc, srv)
}

func _Worker_RunTask_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(RunTaskRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(WorkerServer).RunTask(m, &workerRunTaskServer{stream})
}

type Worker_RunTaskServer interface {
	Send(*RunTaskResponse) error
	grpc.ServerStream
}

type workerRunTaskServer struct {
	grpc.ServerStream
}

func (x *workerRunTaskServer) Send(m *RunTaskResponse) error {
	return x.ServerStream.SendMsg(m)
}

// Worker_ServiceDesc is the grpc.ServiceDesc for Worker service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Worker_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "flex.Worker",
	HandlerType: (*WorkerServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "RunTask",
			Handler:       _Worker_RunTask_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "flex.proto",
}
