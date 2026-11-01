/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package octeliumc

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/octelium/octelium/apis/rsc/rpubsubv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

func TestPubSubC(t *testing.T) {

	t.Setenv("OCTELIUM_ENABLE_SPIFFE_CSI", "false")
	t.Setenv("SPIFFE_ENDPOINT_SOCKET", "")

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.Nil(t, err)
	t.Cleanup(func() {
		lis.Close()
	})

	srv := &pubSubServer{
		requests: make(chan proto.Message, 3),
		message: &rpubsubv1.Message{
			Id:   []byte("1-0"),
			Type: &rpubsubv1.Message_Data{Data: []byte("message")},
		},
	}
	grpcSrv := grpc.NewServer()
	rpubsubv1.RegisterMainServiceServer(grpcSrv, srv)
	t.Cleanup(grpcSrv.Stop)
	go grpcSrv.Serve(lis)

	client, err := NewClient(ctx, &Opts{Addr: lis.Addr().String()})
	require.Nil(t, err)
	var octeliumC ClientInterface = client
	pubSubC := octeliumC.PubSubC()
	require.NotNil(t, pubSubC)

	pubReq := &rpubsubv1.PublishRequest{
		Topic: []byte("topic"),
		Messages: []*rpubsubv1.Message{
			{Type: &rpubsubv1.Message_Data{Data: srv.message.GetData()}},
		},
		MaxLength: 10,
	}
	pubResp, err := pubSubC.Publish(ctx, pubReq)
	require.Nil(t, err)
	assert.True(t, proto.Equal(pubReq, <-srv.requests))
	assert.Equal(t, [][]byte{srv.message.Id}, pubResp.Ids)

	subReq := &rpubsubv1.SubscribeRequest{
		Topic:    pubReq.Topic,
		Group:    []byte("group"),
		Position: rpubsubv1.SubscribeRequest_EARLIEST,
	}
	stream, err := pubSubC.Subscribe(ctx, subReq)
	require.Nil(t, err)
	subResp, err := stream.Recv()
	require.Nil(t, err)
	assert.True(t, proto.Equal(subReq, <-srv.requests))
	require.Len(t, subResp.Messages, 1)
	assert.True(t, proto.Equal(srv.message, subResp.Messages[0]))
	_, err = stream.Recv()
	assert.Equal(t, io.EOF, err)

	ackReq := &rpubsubv1.AcknowledgeRequest{
		Topic: pubReq.Topic,
		Group: subReq.Group,
		Ids:   pubResp.Ids,
	}
	ackResp, err := pubSubC.Acknowledge(ctx, ackReq)
	require.Nil(t, err)
	assert.True(t, proto.Equal(ackReq, <-srv.requests))
	assert.Equal(t, uint32(1), ackResp.Count)
}

type pubSubServer struct {
	rpubsubv1.UnimplementedMainServiceServer
	requests chan proto.Message
	message  *rpubsubv1.Message
}

func (s *pubSubServer) Publish(ctx context.Context, req *rpubsubv1.PublishRequest) (*rpubsubv1.PublishResponse, error) {
	s.requests <- req
	return &rpubsubv1.PublishResponse{Ids: [][]byte{s.message.Id}}, nil
}

func (s *pubSubServer) Subscribe(req *rpubsubv1.SubscribeRequest, stream grpc.ServerStreamingServer[rpubsubv1.SubscribeResponse]) error {
	s.requests <- req
	return stream.Send(&rpubsubv1.SubscribeResponse{Messages: []*rpubsubv1.Message{s.message}})
}

func (s *pubSubServer) Acknowledge(ctx context.Context, req *rpubsubv1.AcknowledgeRequest) (*rpubsubv1.AcknowledgeResponse, error) {
	s.requests <- req
	return &rpubsubv1.AcknowledgeResponse{Count: uint32(len(req.Ids))}, nil
}
