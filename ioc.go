package ioc

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/heimdalr/dag"
)

// container holds the type-based dependency registry and the DAG
// for topological ordering.
type container struct {
	graph        *dag.DAG
	typeToEntry  map[reflect.Type][]entry // return type → entries
	keyToEntry   map[string]entry         // function key → entry (for DAG ordering)
	atEndEntries []entry
	errs         []error
}

type dependency any
type constructor any

type entry struct {
	dagID       string
	key         string // function name key
	file        string // file where the constructor is defined
	line        int    // line number where the constructor is defined
	constructor constructor
	returnType  reflect.Type // the type this constructor provides
	dependency  dependency   // the resolved instance after invocation
}

// visitor walks the DAG in topological order.
type visitor struct {
	orderedKeys *[]string
}

func (v visitor) Visit(vertex dag.Vertexer) {
	_, key := vertex.Vertex()
	*v.orderedKeys = append(*v.orderedKeys, key.(string))
}

// newContainer creates a new, empty container.
func newContainer() *container {
	return &container{
		graph:       dag.NewDAG(),
		typeToEntry: make(map[reflect.Type][]entry),
		keyToEntry:  make(map[string]entry),
	}
}

// reset clears all state. Used internally for testing.
func (c *container) reset() {
	c.graph = dag.NewDAG()
	c.typeToEntry = make(map[reflect.Type][]entry)
	c.keyToEntry = make(map[string]entry)
	c.atEndEntries = nil
	c.errs = nil
}

// Register registers a constructor function. The framework automatically
// infers dependencies by matching the constructor's parameter types to the
// return types of other registered constructors.
//
// The constructor must be a function that returns at most 2 values.
// If it returns 2 values, the second must be of type error.
// The first return value's type becomes the "provided type" for other
// constructors to depend on.
func (c *container) Register(ctor constructor) error {
	ctorKey, file, line, err := getConstructorInfo(ctor)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}

	ctorType := reflect.TypeOf(ctor)

	// Determine the provided return type.
	returnType, err := getReturnType(ctorKey, ctorType)
	if err != nil {
		return fmt.Errorf("%s:%d: register: %w", file, line, err)
	}

	// Duplicate providers check is removed here to allow multiple registrations.
	// Ambiguity is checked during dependency resolution in findProvider.

	dagID, err := c.graph.AddVertex(ctorKey)
	if err != nil {
		return fmt.Errorf("%s:%d: register: failed to add vertex: %w", file, line, err)
	}

	e := entry{
		dagID:       dagID,
		key:         ctorKey,
		file:        file,
		line:        line,
		constructor: ctor,
		returnType:  returnType,
	}

	c.keyToEntry[ctorKey] = e
	if returnType != nil {
		c.typeToEntry[returnType] = append(c.typeToEntry[returnType], e)
	}

	return nil
}

// RegisterAtEnd registers a constructor to be invoked after all other
// dependencies have been resolved.
func (c *container) RegisterAtEnd(ctor constructor) error {
	ctorKey, file, line, err := getConstructorInfo(ctor)
	if err != nil {
		return fmt.Errorf("registerAtEnd: %w", err)
	}

	ctorType := reflect.TypeOf(ctor)
	returnType, err := getReturnType(ctorKey, ctorType)
	if err != nil {
		return fmt.Errorf("%s:%d: registerAtEnd: %w", file, line, err)
	}

	c.atEndEntries = append(c.atEndEntries, entry{
		key:         ctorKey,
		file:        file,
		line:        line,
		constructor: ctor,
		returnType:  returnType,
	})
	return nil
}

// LoadDependencies builds the dependency graph by inspecting constructor
// parameter types, resolves the topological order, and invokes all
// constructors followed by any at-end constructors.
func (c *container) LoadDependencies() error {
	if len(c.errs) > 0 {
		return c.errs[0]
	}

	// Build DAG edges by matching parameter types to provider return types.
	for _, e := range c.keyToEntry {
		ctorType := reflect.TypeOf(e.constructor)
		for i := 0; i < ctorType.NumIn(); i++ {
			paramType := ctorType.In(i)
			provider, err := c.findProvider(paramType)
			if err != nil {
				return fmt.Errorf("%s:%d: loadDependencies: %s requires %v: %w", e.file, e.line, e.key, paramType, err)
			}
			if err := c.graph.AddEdge(e.dagID, provider.dagID); err != nil {
				return fmt.Errorf("%s:%d: loadDependencies: failed to add edge: %w", e.file, e.line, err)
			}
		}
	}

	// Topological walk.
	var orderedKeys []string
	c.graph.OrderedWalk(visitor{orderedKeys: &orderedKeys})

	// Invoke constructors in reverse topological order (leaves first).
	for i := len(orderedKeys) - 1; i >= 0; i-- {
		key := orderedKeys[i]
		if err := c.invokeConstructor(key); err != nil {
			return err
		}
	}

	// Invoke at-end constructors.
	for _, atEnd := range c.atEndEntries {
		if err := c.invokeEntry(atEnd); err != nil {
			return err
		}
	}

	return nil
}

// findProvider finds the registered entry that provides the given type.
// Supports both exact type matches and interface satisfaction.
// Returns an error if multiple providers implement the same interface or type.
func (c *container) findProvider(t reflect.Type) (entry, error) {
	// Exact type match.
	if entries, ok := c.typeToEntry[t]; ok {
		if len(entries) == 1 {
			return entries[0], nil
		}
		if len(entries) > 1 {
			names := make([]string, len(entries))
			for i, m := range entries {
				names[i] = fmt.Sprintf("%s (%s:%d)", m.key, m.file, m.line)
			}
			return entry{}, fmt.Errorf("multiple providers for exact type %v: %v", t, names)
		}
	}

	// Interface match: find providers whose return type implements t.
	if t.Kind() == reflect.Interface {
		var matches []entry
		for _, entries := range c.typeToEntry {
			for _, e := range entries {
				if e.returnType.Implements(t) {
					matches = append(matches, e)
				}
			}
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
		if len(matches) > 1 {
			names := make([]string, len(matches))
			for i, m := range matches {
				names[i] = fmt.Sprintf("%s (%s:%d)", m.key, m.file, m.line)
			}
			return entry{}, fmt.Errorf("multiple providers implement %v: %v", t, names)
		}
	}

	return entry{}, fmt.Errorf("no provider registered for %v", t)
}

// invokeConstructor invokes a constructor by its key, resolving args from
// already-initialized providers.
func (c *container) invokeConstructor(key string) error {
	e := c.keyToEntry[key]
	return c.invokeAndStore(key, e)
}

// invokeEntry invokes an arbitrary entry (used for at-end constructors).
func (c *container) invokeEntry(e entry) error {
	return c.invokeAndStore(e.key, e)
}

// invokeAndStore invokes a constructor, validates its result, and stores
// the resolved dependency.
func (c *container) invokeAndStore(key string, e entry) error {
	value := reflect.ValueOf(e.constructor)
	ctorType := value.Type()

	if err := validateReturnSignature(key, ctorType); err != nil {
		return err
	}

	// Resolve arguments by type.
	args, err := c.resolveArgsByType(ctorType)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}

	result := value.Call(args)
	if err := extractError(key, result); err != nil {
		return fmt.Errorf("%s:%d: %w", e.file, e.line, err)
	}

	// Store the result for dependents.
	if len(result) > 0 && e.returnType != nil {
		e.dependency = result[0].Interface()
		c.keyToEntry[key] = e

		entries := c.typeToEntry[e.returnType]
		for i, existing := range entries {
			if existing.key == key {
				entries[i] = e
				break
			}
		}
	}

	return nil
}

// resolveArgsByType resolves args by finding the provider for each parameter type.
func (c *container) resolveArgsByType(ctorType reflect.Type) ([]reflect.Value, error) {
	var args []reflect.Value
	for i := 0; i < ctorType.NumIn(); i++ {
		paramType := ctorType.In(i)
		provider, err := c.findProvider(paramType)
		if err != nil {
			return nil, err
		}
		if provider.dependency == nil {
			return nil, fmt.Errorf("dependency %v not yet initialized", paramType)
		}
		args = append(args, reflect.ValueOf(provider.dependency))
	}
	return args, nil
}

// --- Validation helpers ---

// getReturnType extracts the provided type from a constructor's return signature.
// Returns nil for void constructors.
func getReturnType(key string, ctorType reflect.Type) (reflect.Type, error) {
	numOut := ctorType.NumOut()

	if numOut > 2 {
		return nil, fmt.Errorf("%s: constructor must have at most 2 return values, got %d", key, numOut)
	}

	if numOut == 2 {
		if ctorType.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
			return nil, fmt.Errorf("%s: second return value must be of type error", key)
		}
	}

	if numOut == 0 {
		return nil, nil
	}

	returnType := ctorType.Out(0)

	// If the only return is error, it's a side-effect constructor.
	if numOut == 1 && returnType == reflect.TypeOf((*error)(nil)).Elem() {
		return nil, nil
	}

	return returnType, nil
}

// validateReturnSignature checks return value rules.
func validateReturnSignature(key string, ctorType reflect.Type) error {
	_, err := getReturnType(key, ctorType)
	return err
}

// extractError inspects the result of a constructor call and returns any error.
func extractError(key string, result []reflect.Value) error {
	if len(result) == 1 {
		firstVal := result[0]
		if isNillableKind(firstVal.Kind()) && !firstVal.IsNil() {
			if err, ok := firstVal.Interface().(error); ok {
				return err
			}
		}
	}

	if len(result) == 2 {
		secondVal := result[1]
		if secondVal.IsNil() {
			return nil
		}
		return secondVal.Interface().(error)
	}

	return nil
}

// isNillableKind returns true if the reflect.Kind can be nil.
func isNillableKind(k reflect.Kind) bool {
	switch k {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func, reflect.Interface:
		return true
	}
	return false
}

// getConstructorInfo derives a unique string key, file and line from a constructor function.
func getConstructorInfo(ctor constructor) (string, string, int, error) {
	funcValue := reflect.ValueOf(ctor)

	if funcValue.Kind() != reflect.Func {
		return "", "", 0, errors.New("constructor must be a function")
	}

	ptr := funcValue.Pointer()
	f := runtime.FuncForPC(ptr)
	if f == nil {
		return "", "", 0, errors.New("cannot determine function info")
	}

	funcName := f.Name()
	file, line := f.FileLine(ptr)

	parts := strings.Split(funcName, "/")
	lastPart := parts[len(parts)-1]
	subParts := strings.SplitN(lastPart, ".", 2)

	packageName := strings.Join(parts[:len(parts)-1], "/") + "/" + subParts[0]
	functionName := subParts[1]

	return packageName + "." + functionName, file, line, nil
}

// --- default global container ---

var defaultContainer = newContainer()

// Register registers a constructor. Dependencies are inferred automatically
// by matching parameter types to return types of other registered constructors.
func Register(ctor constructor) error {
	return defaultContainer.Register(ctor)
}

// RegisterAtEnd registers a constructor to run after all others.
func RegisterAtEnd(ctor constructor) error {
	return defaultContainer.RegisterAtEnd(ctor)
}

// LoadDependencies resolves the dependency graph and invokes all constructors.
func LoadDependencies() error {
	return defaultContainer.LoadDependencies()
}

// resetDefault clears the default container. Used internally for testing.
func resetDefault() {
	defaultContainer.reset()
}
