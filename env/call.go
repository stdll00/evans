package env

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"google.golang.org/grpc"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/peterh/liner"
	"github.com/pkg/errors"
)

// msg なら再帰的構造になる
type field struct {
	isPrimitive bool
	name        string
	pVal        *string
	mVal        []*field
	descType    descriptor.FieldDescriptorProto_Type
	desc        *desc.FieldDescriptor
}

func (e *Env) Call(name string) (string, error) {
	rpc, err := e.GetRPC(name)
	if err != nil {
		return "", err
	}

	ep := e.genEndpoint(name)

	input, err := inputFields(rpc.RequestType.GetFields())
	if err != nil {
		return "", err
	}

	req := dynamic.NewMessage(rpc.RequestType)
	if err = e.setInput(req, input); err != nil {
		return "", err
	}

	// marshal して
	// invoke
	res := dynamic.NewMessage(rpc.ResponseType)

	// TODO: other than localhost
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", e.config.port), grpc.WithInsecure())
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if err := grpc.Invoke(context.Background(), ep, req, res, conn); err != nil {
		return "", err
	}

	m := jsonpb.Marshaler{
		Indent: "  ",
	}
	data, err := m.MarshalToString(res)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (e *Env) genEndpoint(rpcName string) string {
	ep := fmt.Sprintf("/%s.%s/%s", e.currentPackage, e.currentService, rpcName)
	return ep
}

func (e *Env) setInput(req *dynamic.Message, fields []*field) error {
	for _, f := range fields {
		if !f.isPrimitive {
			// TODO
			msg := dynamic.NewMessage(f.desc.GetMessageType())
			if err := e.setInput(msg, f.mVal); err != nil {
				return err
			}
			req.SetField(f.desc, msg)
		} else {
			pv := *f.pVal
			switch f.descType {
			case descriptor.FieldDescriptorProto_TYPE_DOUBLE:
				v, err := strconv.ParseFloat(pv, 64)
				if err != nil {
					return err
				}
				if err := req.TrySetField(f.desc, v); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_FLOAT:
				v, err := strconv.ParseFloat(pv, 32)
				if err != nil {
					return err
				}
				if err := req.TrySetField(f.desc, float32(v)); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_INT64:
				v, err := strconv.ParseInt(pv, 10, 64)
				if err != nil {
					return err
				}
				if err := req.TrySetField(f.desc, v); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_UINT64:
				v, err := strconv.ParseUint(pv, 10, 64)
				if err != nil {
					return err
				}
				if err := req.TrySetField(f.desc, v); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_INT32:
				v, err := strconv.ParseInt(*f.pVal, 10, 32)
				if err != nil {
					return err
				}
				if err := req.TrySetField(f.desc, int32(v)); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_UINT32:
				v, err := strconv.ParseUint(pv, 10, 32)
				if err != nil {
					return err
				}
				if err := req.TrySetField(f.desc, uint32(v)); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_FIXED64:
				v, err := strconv.ParseUint(pv, 10, 64)
				if err != nil {
					return err
				}
				if err := req.TrySetField(f.desc, v); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_FIXED32:
				v, err := strconv.ParseUint(pv, 10, 32)
				if err != nil {
					return err
				}
				if err := req.TrySetField(f.desc, uint32(v)); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_BOOL:
				v, err := strconv.ParseBool(pv)
				if err != nil {
					return err
				}
				if err := req.TrySetField(f.desc, v); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_STRING:
				// already string
				v := pv
				if err := req.TrySetField(f.desc, v); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_BYTES:
				v := []byte(pv)
				if err := req.TrySetField(f.desc, v); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_SFIXED64:
				v, err := strconv.ParseUint(pv, 10, 64)
				if err != nil {
					return err
				}
				if err := req.TrySetField(f.desc, v); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_SFIXED32:
				v, err := strconv.ParseUint(pv, 10, 32)
				if err != nil {
					return err
				}
				if err := req.TrySetField(f.desc, int32(v)); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_SINT64:
				v, err := strconv.ParseInt(pv, 10, 64)
				if err != nil {
					return err
				}
				if err := req.TrySetField(f.desc, v); err != nil {
					return err
				}

			case descriptor.FieldDescriptorProto_TYPE_SINT32:
				v, err := strconv.ParseInt(pv, 10, 32)
				if err != nil {
					return err
				}
				if err := req.TrySetField(f.desc, int32(v)); err != nil {
					return err
				}

			default:
				return fmt.Errorf("invalid type: %#v", f.descType)
			}

		}
	}
	return nil
}

func inputFields(fields []*desc.FieldDescriptor) ([]*field, error) {
	const format = "%s (%s) | "

	liner := liner.NewLiner()
	defer liner.Close()

	input := make([]*field, len(fields))
	max := maxLen(fields, format)
	for i, f := range fields {
		input[i] = &field{
			name:     f.GetName(),
			desc:     f,
			descType: f.GetType(),
		}

		if isMessageType(f.GetType()) {
			fields, err := inputFields(f.GetMessageType().GetFields())
			if err != nil {
				return nil, errors.Wrap(err, "failed to read inputs")
			}

			input[i].mVal = fields
		} else {
			l, err := liner.Prompt(fmt.Sprintf("%"+strconv.Itoa(max)+"s", fmt.Sprintf(format, f.GetName(), f.GetType())))
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			input[i].isPrimitive = true
			input[i].pVal = &l
		}

	}
	return input, nil
}

func maxLen(fields []*desc.FieldDescriptor, format string) int {
	var max int
	for _, f := range fields {
		if isMessageType(f.GetType()) {
			continue
		}
		l := len(fmt.Sprintf(format, f.GetName(), f.GetType()))
		if l > max {
			max = l
		}
	}
	return max
}

func isMessageType(typeName descriptor.FieldDescriptorProto_Type) bool {
	return typeName == descriptor.FieldDescriptorProto_TYPE_MESSAGE
}
