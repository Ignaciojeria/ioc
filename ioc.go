package ioc

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/heimdalr/dag"
)

// Container holds the dependency graph and all registered constructors.
// Use New() to create an instance, or use the package-level Default container.
type Container struct {
	graph                  *dag.DAG
	errs                   []error
	dependencyContainerMap map[string]entry
	orderedDependencyKeys  []string
	atEndEntries           []entry
}

type dependency any
type constructor any

type entry struct {
	id                    string
	constructor           constructor
	constructorParameters []constructor
	dependency            dependency
}

// visitor is used to walk the graph in topological order.
type visitor struct {
	orderedDependencyKeys *[]string
}

func (v visitor) Visit(vertex dag.Vertexer) {
	_, key := vertex.Vertex()
	*v.orderedDependencyKeys = append(*v.orderedDependencyKeys, key.(string))
}

// New creates a new, empty Container ready for dependency registration.
func New() *Container {
	return &Container{
		graph:                  dag.NewDAG(),
		dependencyContainerMap: make(map[string]entry),
	}
}

// Reset clears all registered dependencies and errors, returning the container
// to its initial state. Useful for testing.
func (c *Container) Reset() {
	c.graph = dag.NewDAG()
	c.errs = nil
	c.dependencyContainerMap = make(map[string]entry)
	c.orderedDependencyKeys = nil
	c.atEndEntries = nil
}

// Register registers a constructor function and its dependency constructors.
// The constructor must be a function. Dependencies are identified by their
// function pointer and resolved automatically during LoadDependencies.
//
// Returns an error if the constructor is invalid, already registered, or
// cannot be added to the dependency graph.
func (c *Container) Register(ctor constructor, deps ...constructor) error {
	ctorKey, err := getConstructorKey(ctor)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}

	if c.dependencyContainerMap[ctorKey].constructor != nil {
		return fmt.Errorf("register: constructor already registered: %s", ctorKey)
	}

	id, err := c.graph.AddVertex(ctorKey)
	if err != nil {
		return fmt.Errorf("register: failed to add vertex %s: %w", ctorKey, err)
	}

	c.dependencyContainerMap[ctorKey] = entry{
		id:                    id,
		constructor:           ctor,
		constructorParameters: deps,
	}
	return nil
}

// RegisterAtEnd registers a constructor to be invoked after all other
// dependencies have been resolved. Multiple at-end constructors can be
// registered; they will run in the order they were registered.
//
// Returns an error if the constructor is invalid.
func (c *Container) RegisterAtEnd(ctor constructor, deps ...constructor) error {
	ctorKey, err := getConstructorKey(ctor)
	if err != nil {
		return fmt.Errorf("registerAtEnd: %w", err)
	}

	c.atEndEntries = append(c.atEndEntries, entry{
		id:                    ctorKey,
		constructor:           ctor,
		constructorParameters: deps,
	})
	return nil
}

// LoadDependencies builds the dependency graph, resolves the topological order,
// invokes all registered constructors, and finally invokes any at-end constructors.
func (c *Container) LoadDependencies() error {
	// Return the first accumulated error, if any.
	if len(c.errs) > 0 {
		return c.errs[0]
	}

	// Build the dependency edges in the graph.
	for _, e := range c.dependencyContainerMap {
		for _, dep := range e.constructorParameters {
			depEntry, err := c.getEntry(dep)
			if err != nil {
				return fmt.Errorf("loadDependencies: %w", err)
			}
			if err := c.graph.AddEdge(e.id, depEntry.id); err != nil {
				return fmt.Errorf("loadDependencies: failed to add edge: %w", err)
			}
		}
	}

	// Walk the graph in topological order.
	c.orderedDependencyKeys = nil
	c.graph.OrderedWalk(visitor{orderedDependencyKeys: &c.orderedDependencyKeys})

	// Invoke constructors in reverse topological order (leaves first).
	for i := len(c.orderedDependencyKeys) - 1; i >= 0; i-- {
		key := c.orderedDependencyKeys[i]
		if err := c.invokeConstructor(key); err != nil {
			return err
		}
	}

	// Invoke at-end constructors in registration order.
	for _, atEnd := range c.atEndEntries {
		if err := c.invokeAtEndConstructor(atEnd); err != nil {
			return err
		}
	}

	return nil
}

// invokeConstructor invokes a single registered constructor by its key,
// resolving its dependencies from already-initialized entries.
func (c *Container) invokeConstructor(key string) error {
	e := c.dependencyContainerMap[key]
	value := reflect.ValueOf(e.constructor)

	if err := validateReturnSignature(key, value); err != nil {
		return err
	}

	args, err := c.resolveArgs(e.constructorParameters)
	if err != nil {
		return err
	}

	if err := validateArguments(key, value, args); err != nil {
		return err
	}

	result := value.Call(args)
	if err := extractError(key, result); err != nil {
		return err
	}

	if len(result) > 0 {
		e.dependency = result[0].Interface()
		c.dependencyContainerMap[key] = e
	}
	return nil
}

// invokeAtEndConstructor invokes a single at-end constructor.
func (c *Container) invokeAtEndConstructor(atEnd entry) error {
	value := reflect.ValueOf(atEnd.constructor)

	if err := validateReturnSignature(atEnd.id, value); err != nil {
		return err
	}

	args, err := c.resolveArgs(atEnd.constructorParameters)
	if err != nil {
		return err
	}

	if err := validateArguments(atEnd.id, value, args); err != nil {
		return err
	}

	result := value.Call(args)
	return extractError(atEnd.id, result)
}

// resolveArgs resolves the already-initialized dependencies for a list of
// constructor parameters.
func (c *Container) resolveArgs(params []constructor) ([]reflect.Value, error) {
	var args []reflect.Value
	for _, param := range params {
		dep, err := c.get(param)
		if err != nil {
			return nil, err
		}
		args = append(args, reflect.ValueOf(dep))
	}
	return args, nil
}

// getEntry returns the entry for a constructor, or an error if not found.
func (c *Container) getEntry(ctor constructor) (entry, error) {
	ctorKey, err := getConstructorKey(ctor)
	if err != nil {
		return entry{}, fmt.Errorf("getEntry: %w", err)
	}
	e, ok := c.dependencyContainerMap[ctorKey]
	if !ok {
		return entry{}, fmt.Errorf("getEntry: constructor not registered: %s", ctorKey)
	}
	return e, nil
}

// get returns the resolved dependency for a constructor, or an error if
// the dependency has not yet been initialized.
func (c *Container) get(ctor constructor) (dependency, error) {
	ctorKey, err := getConstructorKey(ctor)
	if err != nil {
		return nil, err
	}
	dep := c.dependencyContainerMap[ctorKey].dependency
	if dep != nil {
		return dep, nil
	}
	return nil, fmt.Errorf("dependency not present: %s", ctorKey)
}

// --- Validation helpers (pure functions, no state) ---

// validateReturnSignature checks that a constructor has at most 2 return values,
// and that the second (if present) is of type error.
func validateReturnSignature(key string, value reflect.Value) error {
	funcType := value.Type()
	numOut := funcType.NumOut()

	if numOut > 2 {
		return fmt.Errorf("%s: constructor must have at most 2 return values, got %d", key, numOut)
	}

	if numOut == 2 {
		if funcType.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
			return fmt.Errorf("%s: second return value must be of type error", key)
		}
	}

	return nil
}

// validateArguments checks that the provided arguments match the constructor's
// parameter types in number and kind.
func validateArguments(key string, value reflect.Value, args []reflect.Value) error {
	funcType := value.Type()

	if funcType.NumIn() != len(args) {
		return fmt.Errorf("%s: expected %d arguments, got %d", key, funcType.NumIn(), len(args))
	}

	for i := 0; i < len(args); i++ {
		expectedType := funcType.In(i)
		argType := args[i].Type()

		if expectedType.Kind() == reflect.Interface && !argType.Implements(expectedType) {
			return fmt.Errorf("%s: argument %d does not implement %v (got %v)", key, i, expectedType, argType)
		}

		if expectedType.Kind() != reflect.Interface && expectedType != argType {
			return fmt.Errorf("%s: argument %d type mismatch, expected %v, got %v", key, i, expectedType, argType)
		}
	}

	return nil
}

// extractError inspects the result of a constructor call and returns any error.
// Handles both single-return (where the value itself might be an error) and
// two-return (value, error) signatures.
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
		// Safe to cast: validateReturnSignature already verified the second
		// return type is error before we reach this point.
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

// getConstructorKey derives a unique string key from a constructor function
// using its runtime function pointer.
func getConstructorKey(ctor constructor) (string, error) {
	funcValue := reflect.ValueOf(ctor)

	if funcValue.Kind() != reflect.Func {
		return "", errors.New("constructor must be a function")
	}

	funcName := runtime.FuncForPC(funcValue.Pointer()).Name()
	parts := strings.Split(funcName, "/")
	lastPart := parts[len(parts)-1]
	subParts := strings.SplitN(lastPart, ".", 2)

	packageName := strings.Join(parts[:len(parts)-1], "/") + "/" + subParts[0]
	functionName := subParts[1]

	return packageName + "." + functionName, nil
}

// --- Default global container ---

// Default is the package-level Container used by the top-level Register,
// RegisterAtEnd, and LoadDependencies functions.
var Default = New()

// Register registers a constructor on the Default container.
// See Container.Register for details.
func Register(ctor constructor, deps ...constructor) error {
	return Default.Register(ctor, deps...)
}

// RegisterAtEnd registers an at-end constructor on the Default container.
// See Container.RegisterAtEnd for details.
func RegisterAtEnd(ctor constructor, deps ...constructor) error {
	return Default.RegisterAtEnd(ctor, deps...)
}

// LoadDependencies resolves and initializes all dependencies on the Default container.
// See Container.LoadDependencies for details.
func LoadDependencies() error {
	return Default.LoadDependencies()
}

// Reset clears the Default container. See Container.Reset for details.
func Reset() {
	Default.Reset()
}
