package evmrpc

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/ethereum/go-ethereum/rpc"
)

func ServedMethods(apis []rpc.API) []string {
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
	subscriptionType := reflect.TypeOf(rpc.Subscription{})
	names := make(map[string]struct{})
	for _, api := range apis {
		typ := reflect.TypeOf(api.Service)
		for i := 0; i < typ.NumMethod(); i++ {
			method := typ.Method(i)
			if method.PkgPath != "" || !newCallback(method.Type) {
				continue
			}
			name := []rune(method.Name)
			name[0] = unicode.ToLower(name[0])
			prefix := api.Namespace + "_"
			if method.Type.NumIn() >= 2 && method.Type.In(1) == contextType &&
				method.Type.NumOut() == 2 && method.Type.Out(1).Implements(errorType) {
				resultType := method.Type.Out(0)
				for resultType.Kind() == reflect.Ptr {
					resultType = resultType.Elem()
				}
				if resultType == subscriptionType {
					prefix += "subscribe:"
				}
			}
			names[prefix+string(name)] = struct{}{}
		}
	}
	methods := make([]string, 0, len(names))
	for name := range names {
		methods = append(methods, name)
	}
	sort.Strings(methods)
	return methods
}

func newCallback(method reflect.Type) bool {
	if method.NumOut() > 2 {
		return false
	}
	if method.NumOut() == 2 {
		errorType := reflect.TypeOf((*error)(nil)).Elem()
		return !method.Out(0).Implements(errorType) && method.Out(1).Implements(errorType)
	}
	return true
}

func registeredAPIs(apis []rpc.API, modules []string) []rpc.API {
	allowed := make(map[string]bool, len(modules))
	for _, module := range modules {
		allowed[module] = true
	}
	registered := make([]rpc.API, 0, len(apis))
	for _, api := range apis {
		if len(allowed) == 0 || allowed[api.Namespace] {
			registered = append(registered, api)
		}
	}
	return registered
}

func (h *HTTPServer) HTTPMethods() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	methods := ServedMethods(h.apis)
	served := make([]string, 0, len(methods))
	for _, method := range methods {
		if strings.Contains(method, "_subscribe:") || paxLegacyGateError(method, h.HTTPConfig.PaxLegacyAllowlist) != nil {
			continue
		}
		served = append(served, method)
	}
	return served
}

func (h *HTTPServer) WSMethods() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return ServedMethods(h.wsAPIs)
}
