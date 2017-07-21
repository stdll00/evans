package env

import (
	"fmt"
	"io"
	"strconv"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/k0kubun/pp"
	"github.com/lycoris0731/evans/model"
	"github.com/peterh/liner"
	"github.com/pkg/errors"
)

// msg なら再帰的構造になる
type field struct {
	isPrimitive bool
	name        string
	pVal        *string
	mVal        []*field
	fType       string
}

func (e *Env) Call(name string) (string, error) {
	rpc, err := e.GetRPC(name)
	if err != nil {
		return "", err
	}

	_ = e.genEndpoint(name)

	req, err := e.GetMessage(rpc.RequestType.GetName())
	if err != nil {
		return "", err
	}

	pp.Println(req)
	// req, err := e.GetMessage(rpc.ResponseType.String())
	// if err != nil {
	// 	return "", err
	// }
	// pp.Println(req)

	_, err = inputFields(req.Fields)
	if err != nil {
		return "", err
	}

	// marshal して
	// invoke

	return "", nil
}

func (e *Env) genEndpoint(rpcName string) string {
	ep := fmt.Sprint("/%s.%s/%s", e.currentPackage, e.currentService, rpcName)
	return ep
}

func inputFields(fields []*model.Field) ([]*field, error) {
	const format = "%s (%s) | "

	liner := liner.NewLiner()
	defer liner.Close()

	input := make([]*field, len(fields))
	max := maxLen(fields, format)
	for i, f := range fields {
		input[i] = &field{
			name:  f.JSONName,
			fType: f.Type.String(),
		}

		if isMessageType(f.Type) {
			fields, err := inputFields(f.Fields)
			if err != nil {
				return nil, errors.Wrap(err, "failed to read inputs")
			}

			input[i].mVal = fields
		} else {
			l, err := liner.Prompt(fmt.Sprintf("%"+strconv.Itoa(max)+"s", fmt.Sprintf(format, f.Name, f.Type)))
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

func maxLen(fields []*model.Field, format string) int {
	var max int
	for _, f := range fields {
		if isMessageType(f.Type) {
			continue
		}
		l := len(fmt.Sprintf(format, f.Name, f.Type))
		if l > max {
			max = l
		}
	}
	return max
}

func isMessageType(typeName descriptor.FieldDescriptorProto_Type) bool {
	return typeName == descriptor.FieldDescriptorProto_TYPE_MESSAGE
}