package proto

import (
	"context"
	"fmt"
	"os"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

//go:generate moq -out mock.go . DescriptorSource
type DescriptorSource interface {
	ListServices() ([]string, error)
	FindSymbol(name string) (protoreflect.Descriptor, error)
}

type reflection struct {
	client interface {
		ListServices() ([]string, error)
		FindSymbol(name string) (protoreflect.Descriptor, error)
	}
}

func NewDescriptorSourceFromReflection(c interface {
	ListServices() ([]string, error)
	FindSymbol(name string) (protoreflect.Descriptor, error)
}) DescriptorSource {
	return &reflection{c}
}

func (r *reflection) ListServices() ([]string, error) {
	return r.client.ListServices()
}

func (r *reflection) FindSymbol(name string) (protoreflect.Descriptor, error) {
	return r.client.FindSymbol(name)
}

func NewDescriptorSourceFromProtoset(fnames []string) (DescriptorSource, error) {
	var fds []linker.File
	for _, fileName := range fnames {
		b, err := os.ReadFile(fileName)
		if err != nil {
			return nil, fmt.Errorf("could not load protoset file %q: %v", fileName, err)
		}
		var fs descriptorpb.FileDescriptorSet
		err = proto.Unmarshal(b, &fs)
		if err != nil {
			return nil, fmt.Errorf("could not parse contents of protoset file %q: %v", fileName, err)
		}
		fd, err := protodesc.NewFiles(&fs)
		if err != nil {
			return nil, fmt.Errorf("cloud not create protoregistry.Files from filedescriptor set %w", err)
		}
		{
			var err error
			fd.RangeFiles(func(descriptor protoreflect.FileDescriptor) bool {
				file, err := linker.NewFileRecursive(descriptor)
				if err != nil {

					return false
				}
				if file == nil {
					return true
				}
				fds = append(fds, file)
				return true
			})
			if err != nil {
				return nil, err
			}
		}
	}
	return &files{fds: fds}, nil
}

type files struct {
	fds linker.Files
}

func NewDescriptorSourceFromFiles(importPaths []string, fnames []string) (DescriptorSource, error) {
	c := &protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: importPaths,
		}),
	}
	compiled, err := c.Compile(context.TODO(), fnames...)
	if err != nil {
		return nil, errors.Wrap(err, "proto: failed to compile proto files")
	}

	return &files{fds: compiled}, nil
}

var errSymbolNotFound = errors.New("proto: symbol not found")

func (f *files) ListServices() ([]string, error) {
	var services []string
	for _, fd := range f.fds {
		for i := 0; i < fd.Services().Len(); i++ {
			services = append(services, string(fd.Services().Get(i).FullName()))
		}
	}

	return services, nil
}

func (f *files) FindSymbol(name string) (protoreflect.Descriptor, error) {
	for _, fd := range f.fds {
		if d := fd.FindDescriptorByName(protoreflect.FullName(name)); d != nil {
			return d, nil
		}
	}

	return nil, errors.Wrapf(errSymbolNotFound, "symbol %s", name)
}
