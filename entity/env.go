package entity

import (
	"strings"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/jhump/protoreflect/desc"
	"github.com/ktr0731/evans/config"
	"github.com/pkg/errors"
)

var (
	ErrPackageUnselected    = errors.New("package unselected")
	ErrServiceUnselected    = errors.New("service unselected")
	ErrUnknownTarget        = errors.New("unknown target")
	ErrUnknownPackage       = errors.New("unknown package")
	ErrUnknownService       = errors.New("unknown service")
	ErrInvalidServiceName   = errors.New("invalid service name")
	ErrInvalidMessageName   = errors.New("invalid message name")
	ErrInvalidRPCName       = errors.New("invalid RPC name")
	ErrServiceCachingFailed = errors.New("service caching failed")
)

type Environment interface {
	Packages() Packages
	Services() (Services, error)
	Messages() (Messages, error)
	RPCs() (RPCs, error)
	Service(name string) (*Service, error)
	Message(name string) (*Message, error)
	RPC(name string) (*RPC, error)

	Headers() []*Header

	UsePackage(name string) error
	UseService(name string) error
}

type Header struct {
	Key, Val string
}

// pkgList is used by showing all packages
// pkg is used by extract a package by package name
type cache struct {
	pkgList Packages
	pkg     map[string]*Package
}

type state struct {
	currentPackage string
	currentService string
}

type Env struct {
	pkgs Packages

	state state

	config *config.Env

	cache cache
}

func New(pkgs Packages, config *config.Env) (*Env, error) {
	return &Env{
		pkgs:   pkgs,
		config: config,
		cache: cache{
			pkg: map[string]*Package{},
		},
	}, nil
}

func (e *Env) HasCurrentPackage() bool {
	return e.state.currentPackage != ""
}

func (e *Env) HasCurrentService() bool {
	return e.state.currentService != ""
}

func (e *Env) Packages() Packages {
	return e.pkgs
}

func (e *Env) Services() (Services, error) {
	if !e.HasCurrentPackage() {
		return nil, ErrPackageUnselected
	}

	// services, messages and rpc are cached to e.cache when called UsePackage()
	// if messages isn't cached, it occurred panic
	return e.cache.pkg[e.state.currentPackage].Services, nil
}

func (e *Env) Messages() (Messages, error) {
	// TODO: current package 以外からも取得したい
	if !e.HasCurrentPackage() {
		return nil, ErrPackageUnselected
	}

	// same as GetServices()
	return e.cache.pkg[e.state.currentPackage].Messages, nil
}

func (e *Env) RPCs() (RPCs, error) {
	if !e.HasCurrentService() {
		return nil, ErrServiceUnselected
	}

	svc, err := e.Service(e.state.currentService)
	if err != nil {
		return nil, err
	}
	return svc.RPCs, nil
}

func (e *Env) Service(name string) (*Service, error) {
	svc, err := e.Services()
	if err != nil {
		return nil, err
	}
	for _, svc := range svc {
		if name == svc.Name {
			return svc, nil
		}
	}
	return nil, errors.Wrapf(ErrInvalidServiceName, "%s not found", name)
}

func (e *Env) Message(name string) (*Message, error) {
	// Person2 で panic
	msg, err := e.Messages()
	if err != nil {
		return nil, err
	}
	for _, msg := range msg {
		msgName := e.getNameFromFQN(name)
		if msgName == msg.Name {
			return msg, nil
		}
	}
	return nil, errors.Wrapf(ErrInvalidMessageName, "%s not found", name)
}

func (e *Env) RPC(name string) (*RPC, error) {
	rpcs, err := e.RPCs()
	if err != nil {
		return nil, err
	}
	for _, rpc := range rpcs {
		if name == rpc.Name {
			return rpc, nil
		}
	}
	return nil, errors.Wrapf(ErrInvalidRPCName, "%s not found", name)
}

func (e *Env) Headers() []*Header {
	panic("not implemented yet")
	return nil
}

func (e *Env) UsePackage(name string) error {
	for _, p := range e.Packages() {
		if name == p.Name {
			e.state.currentPackage = name
			return nil
		}
	}
	return errors.Wrapf(ErrUnknownPackage, "%s not found", name)
}

func (e *Env) UseService(name string) error {
	// set extracted package if passed service which has package name
	if e.state.currentPackage == "" {
		s := strings.SplitN(name, ".", 2)
		if len(s) != 2 {
			return errors.Wrap(ErrPackageUnselected, "please set package (package_name.service_name or set --package flag)")
		}
		if err := e.UsePackage(s[0]); err != nil {
			return errors.Wrapf(err, name)
		}
	}
	services, err := e.Services()
	if err != nil {
		return errors.Wrapf(err, "failed to get services")
	}
	for _, svc := range services {
		if name == svc.Name {
			e.state.currentService = name
			return nil
		}
	}
	return errors.Wrapf(ErrUnknownService, "%s not found", name)
}

func (e *Env) GetDSN() string {
	if e.state.currentPackage == "" {
		return ""
	}
	dsn := e.state.currentPackage
	if e.state.currentService != "" {
		dsn += "." + e.state.currentService
	}
	return dsn
}

// Full Qualified Name
// It contains message or service with package name
// e.g.: .test.Person -> Person
func (e *Env) getNameFromFQN(fqn string) string {
	return strings.TrimLeft(fqn, "."+e.state.currentPackage+".")
}

// getMessage is a closure which has current states
// it is passed by entity.NewField() for get message from current package
func (e *Env) getMessage() func(typeName string) (*Message, error) {
	return func(msgName string) (*Message, error) {
		return e.Message(msgName)
	}
}

func (e *Env) getService() func(typeName string) (*Service, error) {
	return func(svcName string) (*Service, error) {
		return e.Service(svcName)
	}
}

// TODO: unxport
func IsMessageType(typeName descriptor.FieldDescriptorProto_Type) bool {
	return typeName == descriptor.FieldDescriptorProto_TYPE_MESSAGE
}

func IsOneOf(f *desc.FieldDescriptor) bool {
	return f.GetOneOf() != nil
}

func IsEnumType(f *desc.FieldDescriptor) bool {
	return f.GetEnumType() != nil
}
