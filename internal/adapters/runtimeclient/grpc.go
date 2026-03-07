package runtimeclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync"

	v1 "github.com/dongowu/agentos/api/gen/agentos/v1"
	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/pkg/taskdsl"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func normalizeActionPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	normalized := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		normalized[key] = value
	}
	if _, hasCommand := normalized["command"]; !hasCommand {
		if cmd, hasCmd := normalized["cmd"]; hasCmd {
			normalized["command"] = cmd
		}
	}
	return normalized
}

// buildStreamOutputRequestMessage uses a dynamic descriptor so streaming can send the
// new payload field before checked-in Go protobuf stubs are regenerated.
var (
	streamOutputRequestDescOnce sync.Once
	streamOutputRequestDesc     protoreflect.MessageDescriptor
	streamOutputRequestDescErr  error
)

func strPtr(value string) *string { return &value }
func int32Ptr(value int32) *int32 { return &value }

func streamOutputRequestDescriptor() (protoreflect.MessageDescriptor, error) {
	streamOutputRequestDescOnce.Do(func() {
		fileProto := protodesc.ToFileDescriptorProto(v1.File_agentos_v1_runtime_proto)
		for _, message := range fileProto.GetMessageType() {
			if message.GetName() != "StreamOutputRequest" {
				continue
			}
			hasPayload := false
			for _, field := range message.GetField() {
				if field.GetName() == "payload" || field.GetNumber() == 3 {
					hasPayload = true
					break
				}
			}
			if !hasPayload {
				message.Field = append(message.Field, &descriptorpb.FieldDescriptorProto{
					Name:     strPtr("payload"),
					JsonName: strPtr("payload"),
					Number:   int32Ptr(3),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(),
				})
			}
		}
		fileDesc, err := protodesc.NewFile(fileProto, nil)
		if err != nil {
			streamOutputRequestDescErr = err
			return
		}
		streamOutputRequestDesc = fileDesc.Messages().ByName("StreamOutputRequest")
		if streamOutputRequestDesc == nil {
			streamOutputRequestDescErr = fmt.Errorf("stream output request descriptor not found")
		}
	})
	return streamOutputRequestDesc, streamOutputRequestDescErr
}

func buildStreamOutputRequestMessage(taskID, actionID string, payload []byte) (proto.Message, error) {
	desc, err := streamOutputRequestDescriptor()
	if err != nil {
		return nil, fmt.Errorf("stream output descriptor: %w", err)
	}
	msg := dynamicpb.NewMessage(desc)
	fields := desc.Fields()
	msg.Set(fields.ByName("task_id"), protoreflect.ValueOfString(taskID))
	msg.Set(fields.ByName("action_id"), protoreflect.ValueOfString(actionID))
	if len(payload) > 0 {
		msg.Set(fields.ByName("payload"), protoreflect.ValueOfBytes(payload))
	}
	return msg, nil
}

type streamOutputReceiver interface {
	Recv() (*v1.StreamChunk, error)
}

type rawStreamOutputClient struct {
	grpc.ClientStream
}

func (c *rawStreamOutputClient) Recv() (*v1.StreamChunk, error) {
	chunk := &v1.StreamChunk{}
	if err := c.ClientStream.RecvMsg(chunk); err != nil {
		return nil, err
	}
	return chunk, nil
}

// GRPCExecutorClient implements runtimeclient.ExecutorClient via gRPC.
type GRPCExecutorClient struct {
	addr   string
	client v1.RuntimeServiceClient
	conn   *grpc.ClientConn
}

// NewGRPCExecutorClient dials the worker and returns a client.
func NewGRPCExecutorClient(ctx context.Context, addr string) (*GRPCExecutorClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}
	return &GRPCExecutorClient{
		addr:   addr,
		client: v1.NewRuntimeServiceClient(conn),
		conn:   conn,
	}, nil
}

// ExecuteAction implements runtimeclient.ExecutorClient.
func (c *GRPCExecutorClient) ExecuteAction(ctx context.Context, taskID string, action *taskdsl.Action) (*runtimeclient.ExecutionResult, error) {
	payload, _ := json.Marshal(normalizeActionPayload(action.Payload))
	req := &v1.ExecuteActionRequest{
		TaskId:         taskID,
		ActionId:       action.ID,
		RuntimeProfile: action.RuntimeEnv,
		Payload:        payload,
	}
	resp, err := c.client.ExecuteAction(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("execute action: %w", err)
	}
	return &runtimeclient.ExecutionResult{
		ExitCode: int(resp.ExitCode),
		Stdout:   resp.Stdout,
		Stderr:   resp.Stderr,
	}, nil
}

func (c *GRPCExecutorClient) openStreamOutput(ctx context.Context, taskID string, action *taskdsl.Action, payload []byte) (streamOutputReceiver, error) {
	var dynamicErr error
	if c.conn != nil {
		msg, err := buildStreamOutputRequestMessage(taskID, action.ID, payload)
		if err != nil {
			dynamicErr = err
		} else {
			stream, err := c.conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, "/agentos.v1.RuntimeService/StreamOutput")
			if err != nil {
				dynamicErr = err
			} else {
				if err := stream.SendMsg(msg); err != nil {
					dynamicErr = err
				} else if err := stream.CloseSend(); err != nil {
					dynamicErr = err
				} else {
					return &rawStreamOutputClient{ClientStream: stream}, nil
				}
			}
		}
	}
	if c.client == nil {
		if dynamicErr != nil {
			return nil, dynamicErr
		}
		return nil, fmt.Errorf("stream output client not configured")
	}
	legacy, err := c.client.StreamOutput(ctx, &v1.StreamOutputRequest{
		TaskId:   taskID,
		ActionId: string(payload),
	})
	if err != nil {
		if dynamicErr != nil {
			return nil, fmt.Errorf("stream output dynamic path: %v; legacy fallback: %w", dynamicErr, err)
		}
		return nil, err
	}
	return legacy, nil
}

// ExecuteStream implements runtimeclient.StreamingExecutorClient.
func (c *GRPCExecutorClient) ExecuteStream(ctx context.Context, taskID string, action *taskdsl.Action, sink func(runtimeclient.StreamChunk)) (*runtimeclient.ExecutionResult, error) {
	payload, _ := json.Marshal(normalizeActionPayload(action.Payload))
	stream, err := c.openStreamOutput(ctx, taskID, action, payload)
	if err != nil {
		return nil, fmt.Errorf("stream output: %w", err)
	}

	result := &runtimeclient.ExecutionResult{}
	seenExit := false
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("receive stream chunk: %w", err)
		}

		normalized := runtimeclient.StreamChunk{
			TaskID:   taskID,
			ActionID: action.ID,
			Kind:     chunk.GetKind(),
			Data:     append([]byte(nil), chunk.GetData()...),
		}
		switch chunk.GetKind() {
		case "stdout":
			result.Stdout = append(result.Stdout, chunk.GetData()...)
			if sink != nil {
				sink(normalized)
			}
		case "stderr":
			result.Stderr = append(result.Stderr, chunk.GetData()...)
			if sink != nil {
				sink(normalized)
			}
		case "exit":
			exitCode, parseErr := strconv.Atoi(string(chunk.GetData()))
			if parseErr != nil {
				return nil, fmt.Errorf("parse exit code: %w", parseErr)
			}
			result.ExitCode = exitCode
			seenExit = true
		case "error":
			return nil, fmt.Errorf("stream output: %s", string(chunk.GetData()))
		default:
			if sink != nil {
				sink(normalized)
			}
		}
	}
	if !seenExit {
		return nil, fmt.Errorf("stream output ended without exit chunk")
	}
	return result, nil
}

// Close closes the gRPC connection.
func (c *GRPCExecutorClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
