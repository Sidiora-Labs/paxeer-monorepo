package types

import (
	fmt "fmt"
	"reflect"

	"github.com/gogo/protobuf/proto"
)

type legacyInterfaceRegistry struct {
	interfaceNames map[string]reflect.Type
	interfaceImpls map[reflect.Type]interfaceMap
	typeURLMap     map[string]reflect.Type
}

// NewInterfaceRegistry returns a new InterfaceRegistry
func NewLegacyInterfaceRegistry() InterfaceRegistry {
	return &legacyInterfaceRegistry{
		interfaceNames: map[string]reflect.Type{},
		interfaceImpls: map[reflect.Type]interfaceMap{},
		typeURLMap:     map[string]reflect.Type{},
	}
}

func (registry *legacyInterfaceRegistry) RegisterInterface(protoName string, iface interface{}, impls ...proto.Message) {
	typ := reflect.TypeOf(iface)
	if typ.Elem().Kind() != reflect.Interface {
		panic(fmt.Errorf("%T is not an interface type", iface))
	}
	registry.interfaceNames[protoName] = typ
	registry.RegisterImplementations(iface, impls...)
}

// RegisterImplementations registers a concrete proto Message which implements
// the given interface.
//
// This function PANICs if different concrete types are registered under the
// same typeURL.
func (registry *legacyInterfaceRegistry) RegisterImplementations(iface interface{}, impls ...proto.Message) {
	for _, impl := range impls {
		typeURL := "/" + proto.MessageName(impl)
		registry.registerImpl(iface, typeURL, impl)
	}
}

// RegisterCustomTypeURL registers a concrete type which implements the given
// interface under `typeURL`.
//
// This function PANICs if different concrete types are registered under the
// same typeURL.
func (registry *legacyInterfaceRegistry) RegisterCustomTypeURL(iface interface{}, typeURL string, impl proto.Message) {
	registry.registerImpl(iface, typeURL, impl)
}

// registerImpl registers a concrete type which implements the given
// interface under `typeURL`.
//
// This function PANICs if different concrete types are registered under the
// same typeURL.
func (registry *legacyInterfaceRegistry) registerImpl(iface interface{}, typeURL string, impl proto.Message) {
	ityp := reflect.TypeOf(iface).Elem()
	imap, found := registry.interfaceImpls[ityp]
	if !found {
		imap = map[string]reflect.Type{}
	}

	implType := reflect.TypeOf(impl)
	if !implType.AssignableTo(ityp) {
		panic(fmt.Errorf("type %T doesn't actually implement interface %+v", impl, ityp))
	}

	// Check if we already registered something under the given typeURL. It's
	// okay to register the same concrete type again, but if we are registering
	// a new concrete type under the same typeURL, then we throw an error (here,
	// we panic).
	foundImplType, found := imap[typeURL]
	if found && foundImplType != implType {
		panic(
			fmt.Errorf(
				"concrete type %s has already been registered under typeURL %s, cannot register %s under same typeURL. "+
					"This usually means that there are conflicting modules registering different concrete types "+
					"for a same interface implementation",
				foundImplType,
				typeURL,
				implType,
			),
		)
	}

	imap[typeURL] = implType
	registry.typeURLMap[typeURL] = implType

	registry.interfaceImpls[ityp] = imap
}

func (registry *legacyInterfaceRegistry) ListAllInterfaces() []string {
	interfaceNames := registry.interfaceNames
	keys := make([]string, 0, len(interfaceNames))
	for key := range interfaceNames {
		keys = append(keys, key)
	}
	return keys
}

func (registry *legacyInterfaceRegistry) ListImplementations(ifaceName string) []string {
	typ, ok := registry.interfaceNames[ifaceName]
	if !ok {
		return []string{}
	}

	impls, ok := registry.interfaceImpls[typ.Elem()]
	if !ok {
		return []string{}
	}

	keys := make([]string, 0, len(impls))
	for key := range impls {
		keys = append(keys, key)
	}
	return keys
}

func (registry *legacyInterfaceRegistry) UnpackAny(any *Any, iface interface{}) error {
	boundedRegistry := &interfaceRegistry{
		interfaceNames: registry.interfaceNames,
		interfaceImpls: registry.interfaceImpls,
		typeURLMap:     registry.typeURLMap,
	}
	return boundedRegistry.UnpackAny(any, iface)
}

// Resolve returns the proto message given its typeURL. It works with types
// registered with RegisterInterface/RegisterImplementations, as well as those
// registered with RegisterWithCustomTypeURL.
func (registry *legacyInterfaceRegistry) Resolve(typeURL string) (proto.Message, error) {
	typ, found := registry.typeURLMap[typeURL]
	if !found {
		return nil, fmt.Errorf("unable to resolve type URL %s", typeURL)
	}

	msg, ok := reflect.New(typ.Elem()).Interface().(proto.Message)
	if !ok {
		return nil, fmt.Errorf("can't resolve type URL %s", typeURL)
	}

	return msg, nil
}
