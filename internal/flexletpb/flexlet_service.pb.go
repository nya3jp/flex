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

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.17.3
// source: internal/flexletpb/flexlet_service.proto

package flexletpb

import (
	flex "github.com/nya3jp/flex"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type WaitTaskRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	FlexletName string `protobuf:"bytes,1,opt,name=flexlet_name,json=flexletName,proto3" json:"flexlet_name,omitempty"`
}

func (x *WaitTaskRequest) Reset() {
	*x = WaitTaskRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WaitTaskRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WaitTaskRequest) ProtoMessage() {}

func (x *WaitTaskRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WaitTaskRequest.ProtoReflect.Descriptor instead.
func (*WaitTaskRequest) Descriptor() ([]byte, []int) {
	return file_internal_flexletpb_flexlet_service_proto_rawDescGZIP(), []int{0}
}

func (x *WaitTaskRequest) GetFlexletName() string {
	if x != nil {
		return x.FlexletName
	}
	return ""
}

type WaitTaskResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Task *Task `protobuf:"bytes,1,opt,name=task,proto3" json:"task,omitempty"`
}

func (x *WaitTaskResponse) Reset() {
	*x = WaitTaskResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WaitTaskResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WaitTaskResponse) ProtoMessage() {}

func (x *WaitTaskResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WaitTaskResponse.ProtoReflect.Descriptor instead.
func (*WaitTaskResponse) Descriptor() ([]byte, []int) {
	return file_internal_flexletpb_flexlet_service_proto_rawDescGZIP(), []int{1}
}

func (x *WaitTaskResponse) GetTask() *Task {
	if x != nil {
		return x.Task
	}
	return nil
}

type UpdateTaskRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ref *TaskRef `protobuf:"bytes,1,opt,name=ref,proto3" json:"ref,omitempty"`
}

func (x *UpdateTaskRequest) Reset() {
	*x = UpdateTaskRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateTaskRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateTaskRequest) ProtoMessage() {}

func (x *UpdateTaskRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateTaskRequest.ProtoReflect.Descriptor instead.
func (*UpdateTaskRequest) Descriptor() ([]byte, []int) {
	return file_internal_flexletpb_flexlet_service_proto_rawDescGZIP(), []int{2}
}

func (x *UpdateTaskRequest) GetRef() *TaskRef {
	if x != nil {
		return x.Ref
	}
	return nil
}

type UpdateTaskResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *UpdateTaskResponse) Reset() {
	*x = UpdateTaskResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateTaskResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateTaskResponse) ProtoMessage() {}

func (x *UpdateTaskResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateTaskResponse.ProtoReflect.Descriptor instead.
func (*UpdateTaskResponse) Descriptor() ([]byte, []int) {
	return file_internal_flexletpb_flexlet_service_proto_rawDescGZIP(), []int{3}
}

type FinishTaskRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ref       *TaskRef         `protobuf:"bytes,1,opt,name=ref,proto3" json:"ref,omitempty"`
	Result    *flex.TaskResult `protobuf:"bytes,2,opt,name=result,proto3" json:"result,omitempty"`
	NeedRetry bool             `protobuf:"varint,3,opt,name=need_retry,json=needRetry,proto3" json:"need_retry,omitempty"`
}

func (x *FinishTaskRequest) Reset() {
	*x = FinishTaskRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FinishTaskRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FinishTaskRequest) ProtoMessage() {}

func (x *FinishTaskRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FinishTaskRequest.ProtoReflect.Descriptor instead.
func (*FinishTaskRequest) Descriptor() ([]byte, []int) {
	return file_internal_flexletpb_flexlet_service_proto_rawDescGZIP(), []int{4}
}

func (x *FinishTaskRequest) GetRef() *TaskRef {
	if x != nil {
		return x.Ref
	}
	return nil
}

func (x *FinishTaskRequest) GetResult() *flex.TaskResult {
	if x != nil {
		return x.Result
	}
	return nil
}

func (x *FinishTaskRequest) GetNeedRetry() bool {
	if x != nil {
		return x.NeedRetry
	}
	return false
}

type FinishTaskResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *FinishTaskResponse) Reset() {
	*x = FinishTaskResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FinishTaskResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FinishTaskResponse) ProtoMessage() {}

func (x *FinishTaskResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FinishTaskResponse.ProtoReflect.Descriptor instead.
func (*FinishTaskResponse) Descriptor() ([]byte, []int) {
	return file_internal_flexletpb_flexlet_service_proto_rawDescGZIP(), []int{5}
}

type UpdateFlexletRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Status *flex.FlexletStatus `protobuf:"bytes,1,opt,name=status,proto3" json:"status,omitempty"`
}

func (x *UpdateFlexletRequest) Reset() {
	*x = UpdateFlexletRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateFlexletRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateFlexletRequest) ProtoMessage() {}

func (x *UpdateFlexletRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateFlexletRequest.ProtoReflect.Descriptor instead.
func (*UpdateFlexletRequest) Descriptor() ([]byte, []int) {
	return file_internal_flexletpb_flexlet_service_proto_rawDescGZIP(), []int{6}
}

func (x *UpdateFlexletRequest) GetStatus() *flex.FlexletStatus {
	if x != nil {
		return x.Status
	}
	return nil
}

type UpdateFlexletResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *UpdateFlexletResponse) Reset() {
	*x = UpdateFlexletResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateFlexletResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateFlexletResponse) ProtoMessage() {}

func (x *UpdateFlexletResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_flexletpb_flexlet_service_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateFlexletResponse.ProtoReflect.Descriptor instead.
func (*UpdateFlexletResponse) Descriptor() ([]byte, []int) {
	return file_internal_flexletpb_flexlet_service_proto_rawDescGZIP(), []int{7}
}

var File_internal_flexletpb_flexlet_service_proto protoreflect.FileDescriptor

var file_internal_flexletpb_flexlet_service_proto_rawDesc = []byte{
	0x0a, 0x28, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x66, 0x6c, 0x65, 0x78, 0x6c,
	0x65, 0x74, 0x70, 0x62, 0x2f, 0x66, 0x6c, 0x65, 0x78, 0x6c, 0x65, 0x74, 0x5f, 0x73, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x04, 0x66, 0x6c, 0x65, 0x78,
	0x1a, 0x0a, 0x66, 0x6c, 0x65, 0x78, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x20, 0x69, 0x6e,
	0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x66, 0x6c, 0x65, 0x78, 0x6c, 0x65, 0x74, 0x70, 0x62,
	0x2f, 0x66, 0x6c, 0x65, 0x78, 0x6c, 0x65, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x34,
	0x0a, 0x0f, 0x57, 0x61, 0x69, 0x74, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x21, 0x0a, 0x0c, 0x66, 0x6c, 0x65, 0x78, 0x6c, 0x65, 0x74, 0x5f, 0x6e, 0x61, 0x6d,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x66, 0x6c, 0x65, 0x78, 0x6c, 0x65, 0x74,
	0x4e, 0x61, 0x6d, 0x65, 0x22, 0x32, 0x0a, 0x10, 0x57, 0x61, 0x69, 0x74, 0x54, 0x61, 0x73, 0x6b,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1e, 0x0a, 0x04, 0x74, 0x61, 0x73, 0x6b,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0a, 0x2e, 0x66, 0x6c, 0x65, 0x78, 0x2e, 0x54, 0x61,
	0x73, 0x6b, 0x52, 0x04, 0x74, 0x61, 0x73, 0x6b, 0x22, 0x34, 0x0a, 0x11, 0x55, 0x70, 0x64, 0x61,
	0x74, 0x65, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1f, 0x0a,
	0x03, 0x72, 0x65, 0x66, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x66, 0x6c, 0x65,
	0x78, 0x2e, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x65, 0x66, 0x52, 0x03, 0x72, 0x65, 0x66, 0x22, 0x14,
	0x0a, 0x12, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x22, 0x7d, 0x0a, 0x11, 0x46, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x54, 0x61,
	0x73, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1f, 0x0a, 0x03, 0x72, 0x65, 0x66,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x66, 0x6c, 0x65, 0x78, 0x2e, 0x54, 0x61,
	0x73, 0x6b, 0x52, 0x65, 0x66, 0x52, 0x03, 0x72, 0x65, 0x66, 0x12, 0x28, 0x0a, 0x06, 0x72, 0x65,
	0x73, 0x75, 0x6c, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x10, 0x2e, 0x66, 0x6c, 0x65,
	0x78, 0x2e, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x52, 0x06, 0x72, 0x65,
	0x73, 0x75, 0x6c, 0x74, 0x12, 0x1d, 0x0a, 0x0a, 0x6e, 0x65, 0x65, 0x64, 0x5f, 0x72, 0x65, 0x74,
	0x72, 0x79, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x6e, 0x65, 0x65, 0x64, 0x52, 0x65,
	0x74, 0x72, 0x79, 0x22, 0x14, 0x0a, 0x12, 0x46, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x54, 0x61, 0x73,
	0x6b, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x43, 0x0a, 0x14, 0x55, 0x70, 0x64,
	0x61, 0x74, 0x65, 0x46, 0x6c, 0x65, 0x78, 0x6c, 0x65, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x2b, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x13, 0x2e, 0x66, 0x6c, 0x65, 0x78, 0x2e, 0x46, 0x6c, 0x65, 0x78, 0x6c, 0x65, 0x74,
	0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0x17,
	0x0a, 0x15, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x46, 0x6c, 0x65, 0x78, 0x6c, 0x65, 0x74, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0x9f, 0x02, 0x0a, 0x0e, 0x46, 0x6c, 0x65, 0x78,
	0x6c, 0x65, 0x74, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x3b, 0x0a, 0x08, 0x57, 0x61,
	0x69, 0x74, 0x54, 0x61, 0x73, 0x6b, 0x12, 0x15, 0x2e, 0x66, 0x6c, 0x65, 0x78, 0x2e, 0x57, 0x61,
	0x69, 0x74, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e,
	0x66, 0x6c, 0x65, 0x78, 0x2e, 0x57, 0x61, 0x69, 0x74, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x41, 0x0a, 0x0a, 0x55, 0x70, 0x64, 0x61, 0x74,
	0x65, 0x54, 0x61, 0x73, 0x6b, 0x12, 0x17, 0x2e, 0x66, 0x6c, 0x65, 0x78, 0x2e, 0x55, 0x70, 0x64,
	0x61, 0x74, 0x65, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x18,
	0x2e, 0x66, 0x6c, 0x65, 0x78, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x54, 0x61, 0x73, 0x6b,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x41, 0x0a, 0x0a, 0x46, 0x69,
	0x6e, 0x69, 0x73, 0x68, 0x54, 0x61, 0x73, 0x6b, 0x12, 0x17, 0x2e, 0x66, 0x6c, 0x65, 0x78, 0x2e,
	0x46, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x18, 0x2e, 0x66, 0x6c, 0x65, 0x78, 0x2e, 0x46, 0x69, 0x6e, 0x69, 0x73, 0x68, 0x54,
	0x61, 0x73, 0x6b, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x4a, 0x0a,
	0x0d, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x46, 0x6c, 0x65, 0x78, 0x6c, 0x65, 0x74, 0x12, 0x1a,
	0x2e, 0x66, 0x6c, 0x65, 0x78, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x46, 0x6c, 0x65, 0x78,
	0x6c, 0x65, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1b, 0x2e, 0x66, 0x6c, 0x65,
	0x78, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x46, 0x6c, 0x65, 0x78, 0x6c, 0x65, 0x74, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x2b, 0x5a, 0x29, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6e, 0x79, 0x61, 0x33, 0x6a, 0x70, 0x2f, 0x66,
	0x6c, 0x65, 0x78, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x66, 0x6c, 0x65,
	0x78, 0x6c, 0x65, 0x74, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_internal_flexletpb_flexlet_service_proto_rawDescOnce sync.Once
	file_internal_flexletpb_flexlet_service_proto_rawDescData = file_internal_flexletpb_flexlet_service_proto_rawDesc
)

func file_internal_flexletpb_flexlet_service_proto_rawDescGZIP() []byte {
	file_internal_flexletpb_flexlet_service_proto_rawDescOnce.Do(func() {
		file_internal_flexletpb_flexlet_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_internal_flexletpb_flexlet_service_proto_rawDescData)
	})
	return file_internal_flexletpb_flexlet_service_proto_rawDescData
}

var file_internal_flexletpb_flexlet_service_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_internal_flexletpb_flexlet_service_proto_goTypes = []interface{}{
	(*WaitTaskRequest)(nil),       // 0: flex.WaitTaskRequest
	(*WaitTaskResponse)(nil),      // 1: flex.WaitTaskResponse
	(*UpdateTaskRequest)(nil),     // 2: flex.UpdateTaskRequest
	(*UpdateTaskResponse)(nil),    // 3: flex.UpdateTaskResponse
	(*FinishTaskRequest)(nil),     // 4: flex.FinishTaskRequest
	(*FinishTaskResponse)(nil),    // 5: flex.FinishTaskResponse
	(*UpdateFlexletRequest)(nil),  // 6: flex.UpdateFlexletRequest
	(*UpdateFlexletResponse)(nil), // 7: flex.UpdateFlexletResponse
	(*Task)(nil),                  // 8: flex.Task
	(*TaskRef)(nil),               // 9: flex.TaskRef
	(*flex.TaskResult)(nil),       // 10: flex.TaskResult
	(*flex.FlexletStatus)(nil),    // 11: flex.FlexletStatus
}
var file_internal_flexletpb_flexlet_service_proto_depIdxs = []int32{
	8,  // 0: flex.WaitTaskResponse.task:type_name -> flex.Task
	9,  // 1: flex.UpdateTaskRequest.ref:type_name -> flex.TaskRef
	9,  // 2: flex.FinishTaskRequest.ref:type_name -> flex.TaskRef
	10, // 3: flex.FinishTaskRequest.result:type_name -> flex.TaskResult
	11, // 4: flex.UpdateFlexletRequest.status:type_name -> flex.FlexletStatus
	0,  // 5: flex.FlexletService.WaitTask:input_type -> flex.WaitTaskRequest
	2,  // 6: flex.FlexletService.UpdateTask:input_type -> flex.UpdateTaskRequest
	4,  // 7: flex.FlexletService.FinishTask:input_type -> flex.FinishTaskRequest
	6,  // 8: flex.FlexletService.UpdateFlexlet:input_type -> flex.UpdateFlexletRequest
	1,  // 9: flex.FlexletService.WaitTask:output_type -> flex.WaitTaskResponse
	3,  // 10: flex.FlexletService.UpdateTask:output_type -> flex.UpdateTaskResponse
	5,  // 11: flex.FlexletService.FinishTask:output_type -> flex.FinishTaskResponse
	7,  // 12: flex.FlexletService.UpdateFlexlet:output_type -> flex.UpdateFlexletResponse
	9,  // [9:13] is the sub-list for method output_type
	5,  // [5:9] is the sub-list for method input_type
	5,  // [5:5] is the sub-list for extension type_name
	5,  // [5:5] is the sub-list for extension extendee
	0,  // [0:5] is the sub-list for field type_name
}

func init() { file_internal_flexletpb_flexlet_service_proto_init() }
func file_internal_flexletpb_flexlet_service_proto_init() {
	if File_internal_flexletpb_flexlet_service_proto != nil {
		return
	}
	file_internal_flexletpb_flexlet_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_internal_flexletpb_flexlet_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WaitTaskRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_internal_flexletpb_flexlet_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WaitTaskResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_internal_flexletpb_flexlet_service_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateTaskRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_internal_flexletpb_flexlet_service_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateTaskResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_internal_flexletpb_flexlet_service_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FinishTaskRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_internal_flexletpb_flexlet_service_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FinishTaskResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_internal_flexletpb_flexlet_service_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateFlexletRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_internal_flexletpb_flexlet_service_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateFlexletResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_internal_flexletpb_flexlet_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_internal_flexletpb_flexlet_service_proto_goTypes,
		DependencyIndexes: file_internal_flexletpb_flexlet_service_proto_depIdxs,
		MessageInfos:      file_internal_flexletpb_flexlet_service_proto_msgTypes,
	}.Build()
	File_internal_flexletpb_flexlet_service_proto = out.File
	file_internal_flexletpb_flexlet_service_proto_rawDesc = nil
	file_internal_flexletpb_flexlet_service_proto_goTypes = nil
	file_internal_flexletpb_flexlet_service_proto_depIdxs = nil
}
