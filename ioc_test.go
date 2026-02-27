package ioc

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// --- Test constructors ---

type testMessage string

func newTestMessage() testMessage {
	return testMessage("hello")
}

type testGreeter struct {
	Message testMessage
}

func newTestGreeter(m testMessage) testGreeter {
	return testGreeter{Message: m}
}

func (g testGreeter) Greet() testMessage {
	return g.Message
}

type testEvent struct {
	Greeter testGreeter
}

func newTestEvent(g testGreeter) testEvent {
	return testEvent{Greeter: g}
}

func newTestMessageWithError() (testMessage, error) {
	return testMessage("hello"), nil
}

func newTestFailingConstructor() (testMessage, error) {
	return "", errors.New("constructor failed intentionally")
}

// Constructor with 3 return values (invalid).
func newTestThreeReturns() (testMessage, error, int) {
	return "", nil, 0
}

// Constructor with 2 returns where second is not error (invalid).
func newTestBadSecondReturn() (testMessage, string) {
	return "", ""
}

// Constructor that returns a non-nil pointer (not an error).
type testService struct{ Name string }

func newTestServicePtr() *testService {
	return &testService{Name: "svc"}
}

// Constructor that returns a nil pointer.
func newTestNilPtr() *testService {
	return nil
}

// Constructor that returns an error as a single return value (nillable kind).
func newTestSingleErrorReturn() error {
	return fmt.Errorf("single error return")
}

// Constructor that returns a nil error as a single return value.
func newTestSingleNilErrorReturn() error {
	return nil
}

// Constructor that returns a slice (nillable, not error).
func newTestSliceReturn() []string {
	return []string{"a", "b"}
}

// Constructor that returns a nil slice.
func newTestNilSliceReturn() []string {
	return nil
}

// Constructor that returns a map (nillable, not error).
func newTestMapReturn() map[string]string {
	return map[string]string{"k": "v"}
}

// Constructor that returns a channel (nillable, not error).
func newTestChanReturn() chan int {
	return make(chan int)
}

// Constructor that returns a func (nillable, not error).
func newTestFuncReturn() func() {
	return func() {}
}

// A void constructor (no return values).
func newTestVoidConstructor(m testMessage) {
	_ = m
}

// Interface-based constructors for testing interface argument validation.
type testStringer interface {
	String() string
}

type testStringerImpl struct{ Val string }

func (s testStringerImpl) String() string { return s.Val }

func newTestStringerImpl() testStringerImpl {
	return testStringerImpl{Val: "impl"}
}

func newTestNeedsStringer(s testStringer) testMessage {
	return testMessage(s.String())
}

// A type that does NOT implement testStringer.
type testNonStringer struct{}

func newTestNonStringer() testNonStringer {
	return testNonStringer{}
}

// --- At-end tracking ---

var atEndCalled bool
var atEndOrder []string

func testAtEndFunc(e testEvent) {
	atEndCalled = true
}

func testAtEndFirst(m testMessage) {
	atEndOrder = append(atEndOrder, "first")
}

func testAtEndSecond(m testMessage) {
	atEndOrder = append(atEndOrder, "second")
}

func testAtEndFailing(m testMessage) error {
	return errors.New("at-end failed")
}

// --- Tests ---

func TestBasicResolution(t *testing.T) {
	c := New()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	dep, err := c.get(newTestMessage)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	msg, ok := dep.(testMessage)
	if !ok {
		t.Fatalf("expected testMessage, got %T", dep)
	}
	if msg != "hello" {
		t.Fatalf("expected 'hello', got '%s'", msg)
	}
}

func TestDependencyChain(t *testing.T) {
	c := New()

	// Register in arbitrary order — the DAG should sort them.
	if err := c.Register(newTestEvent, newTestGreeter); err != nil {
		t.Fatalf("Register newTestEvent failed: %v", err)
	}
	if err := c.Register(newTestGreeter, newTestMessage); err != nil {
		t.Fatalf("Register newTestGreeter failed: %v", err)
	}
	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register newTestMessage failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	dep, err := c.get(newTestEvent)
	if err != nil {
		t.Fatalf("get newTestEvent failed: %v", err)
	}

	event, ok := dep.(testEvent)
	if !ok {
		t.Fatalf("expected testEvent, got %T", dep)
	}
	if event.Greeter.Message != "hello" {
		t.Fatalf("expected 'hello' in greeter message, got '%s'", event.Greeter.Message)
	}
}

func TestConstructorWithError(t *testing.T) {
	c := New()

	if err := c.Register(newTestFailingConstructor); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error from failing constructor, got nil")
	}
	if err.Error() != "constructor failed intentionally" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestConstructorReturningValueAndNilError(t *testing.T) {
	c := New()

	if err := c.Register(newTestMessageWithError); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	dep, err := c.get(newTestMessageWithError)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	msg, ok := dep.(testMessage)
	if !ok {
		t.Fatalf("expected testMessage, got %T", dep)
	}
	if msg != "hello" {
		t.Fatalf("expected 'hello', got '%s'", msg)
	}
}

func TestRegisterAtEnd(t *testing.T) {
	atEndCalled = false
	c := New()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestGreeter, newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestEvent, newTestGreeter); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.RegisterAtEnd(testAtEndFunc, newTestEvent); err != nil {
		t.Fatalf("RegisterAtEnd failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	if !atEndCalled {
		t.Fatal("expected atEnd function to be called")
	}
}

func TestMultipleRegisterAtEnd(t *testing.T) {
	atEndOrder = nil
	c := New()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.RegisterAtEnd(testAtEndFirst, newTestMessage); err != nil {
		t.Fatalf("RegisterAtEnd first failed: %v", err)
	}
	if err := c.RegisterAtEnd(testAtEndSecond, newTestMessage); err != nil {
		t.Fatalf("RegisterAtEnd second failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	if len(atEndOrder) != 2 || atEndOrder[0] != "first" || atEndOrder[1] != "second" {
		t.Fatalf("expected [first, second], got %v", atEndOrder)
	}
}

func TestRegisterAtEndFailing(t *testing.T) {
	c := New()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.RegisterAtEnd(testAtEndFailing, newTestMessage); err != nil {
		t.Fatalf("RegisterAtEnd failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error from failing atEnd constructor, got nil")
	}
	if err.Error() != "at-end failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDuplicateRegistration(t *testing.T) {
	c := New()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err := c.Register(newTestMessage)
	if err == nil {
		t.Fatal("expected error on duplicate registration, got nil")
	}
}

func TestNonFunctionRegistration(t *testing.T) {
	c := New()

	err := c.Register("not a function")
	if err == nil {
		t.Fatal("expected error when registering a non-function, got nil")
	}
}

func TestNonFunctionRegisterAtEnd(t *testing.T) {
	c := New()

	err := c.RegisterAtEnd("not a function")
	if err == nil {
		t.Fatal("expected error when RegisterAtEnd with a non-function, got nil")
	}
}

func TestReset(t *testing.T) {
	c := New()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	c.Reset()

	// After reset, should be able to register and resolve again.
	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register after Reset failed: %v", err)
	}
	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies after Reset failed: %v", err)
	}

	dep, err := c.get(newTestMessage)
	if err != nil {
		t.Fatalf("get after Reset failed: %v", err)
	}
	if dep.(testMessage) != "hello" {
		t.Fatalf("expected 'hello', got '%v'", dep)
	}
}

func TestMissingDependency(t *testing.T) {
	c := New()

	// Register newTestGreeter which depends on newTestMessage, but don't register newTestMessage.
	if err := c.Register(newTestGreeter, newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error for missing dependency, got nil")
	}
}

func TestMultipleContainers(t *testing.T) {
	c1 := New()
	c2 := New()

	if err := c1.Register(newTestMessage); err != nil {
		t.Fatalf("c1 Register failed: %v", err)
	}

	// c2 should not see c1's registration.
	_, err := c2.get(newTestMessage)
	if err == nil {
		t.Fatal("expected error getting from c2, got nil")
	}

	if err := c2.Register(newTestMessage); err != nil {
		t.Fatalf("c2 Register failed: %v", err)
	}

	if err := c1.LoadDependencies(); err != nil {
		t.Fatalf("c1 LoadDependencies failed: %v", err)
	}
	if err := c2.LoadDependencies(); err != nil {
		t.Fatalf("c2 LoadDependencies failed: %v", err)
	}

	dep1, _ := c1.get(newTestMessage)
	dep2, _ := c2.get(newTestMessage)

	if dep1.(testMessage) != dep2.(testMessage) {
		t.Fatalf("expected same value, got '%v' and '%v'", dep1, dep2)
	}
}

// --- Validation edge case tests ---

func TestConstructorWithThreeReturnValues(t *testing.T) {
	c := New()

	if err := c.Register(newTestThreeReturns); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error for 3 return values, got nil")
	}
}

func TestConstructorWithBadSecondReturn(t *testing.T) {
	c := New()

	if err := c.Register(newTestBadSecondReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error for non-error second return, got nil")
	}
}

func TestVoidConstructor(t *testing.T) {
	c := New()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestVoidConstructor, newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

func TestPointerReturn(t *testing.T) {
	c := New()

	if err := c.Register(newTestServicePtr); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	dep, err := c.get(newTestServicePtr)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	svc := dep.(*testService)
	if svc.Name != "svc" {
		t.Fatalf("expected 'svc', got '%s'", svc.Name)
	}
}

func TestSingleReturnError(t *testing.T) {
	c := New()

	if err := c.Register(newTestSingleErrorReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error from single-return error constructor, got nil")
	}
	if err.Error() != "single error return" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSingleReturnNilError(t *testing.T) {
	c := New()

	if err := c.Register(newTestSingleNilErrorReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Constructor returns nil (interface kind, nil value) — should not fail.
	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

func TestSliceReturn(t *testing.T) {
	c := New()

	if err := c.Register(newTestSliceReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	dep, err := c.get(newTestSliceReturn)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	slice := dep.([]string)
	if len(slice) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(slice))
	}
}

func TestMapReturn(t *testing.T) {
	c := New()

	if err := c.Register(newTestMapReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	dep, err := c.get(newTestMapReturn)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	m := dep.(map[string]string)
	if m["k"] != "v" {
		t.Fatalf("expected 'v', got '%s'", m["k"])
	}
}

func TestChanReturn(t *testing.T) {
	c := New()

	if err := c.Register(newTestChanReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	dep, err := c.get(newTestChanReturn)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if dep == nil {
		t.Fatal("expected non-nil channel")
	}
}

func TestFuncReturn(t *testing.T) {
	c := New()

	if err := c.Register(newTestFuncReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	dep, err := c.get(newTestFuncReturn)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if dep == nil {
		t.Fatal("expected non-nil func")
	}
}

func TestInterfaceArgument(t *testing.T) {
	c := New()

	if err := c.Register(newTestStringerImpl); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestNeedsStringer, newTestStringerImpl); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	dep, err := c.get(newTestNeedsStringer)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if dep.(testMessage) != "impl" {
		t.Fatalf("expected 'impl', got '%v'", dep)
	}
}

func TestInterfaceArgumentNotImplemented(t *testing.T) {
	c := New()

	// Register a non-stringer and try to pass it where a stringer is expected.
	if err := c.Register(newTestNonStringer); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestNeedsStringer, newTestNonStringer); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error for non-implementing interface argument, got nil")
	}
}

func TestArgumentTypeMismatch(t *testing.T) {
	c := New()

	// newTestGreeter expects testMessage, but we give it newTestServicePtr.
	if err := c.Register(newTestServicePtr); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestGreeter, newTestServicePtr); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error for type mismatch, got nil")
	}
}

// --- Global wrapper tests ---

func TestGlobalRegisterAndLoad(t *testing.T) {
	Reset()

	if err := Register(newTestMessage); err != nil {
		t.Fatalf("global Register failed: %v", err)
	}
	if err := Register(newTestGreeter, newTestMessage); err != nil {
		t.Fatalf("global Register failed: %v", err)
	}

	if err := LoadDependencies(); err != nil {
		t.Fatalf("global LoadDependencies failed: %v", err)
	}

	Reset() // cleanup
}

func TestGlobalRegisterAtEnd(t *testing.T) {
	atEndCalled = false
	Reset()

	if err := Register(newTestMessage); err != nil {
		t.Fatalf("global Register failed: %v", err)
	}
	if err := Register(newTestGreeter, newTestMessage); err != nil {
		t.Fatalf("global Register failed: %v", err)
	}
	if err := Register(newTestEvent, newTestGreeter); err != nil {
		t.Fatalf("global Register failed: %v", err)
	}
	if err := RegisterAtEnd(testAtEndFunc, newTestEvent); err != nil {
		t.Fatalf("global RegisterAtEnd failed: %v", err)
	}

	if err := LoadDependencies(); err != nil {
		t.Fatalf("global LoadDependencies failed: %v", err)
	}

	if !atEndCalled {
		t.Fatal("expected atEnd to be called via global wrapper")
	}

	Reset() // cleanup
}

func TestGlobalReset(t *testing.T) {
	Reset()

	if err := Register(newTestMessage); err != nil {
		t.Fatalf("global Register failed: %v", err)
	}

	Reset()

	// After reset, should be able to register again.
	if err := Register(newTestMessage); err != nil {
		t.Fatalf("global Register after Reset failed: %v", err)
	}

	Reset() // cleanup
}

// --- AtEnd validation edge cases ---

func TestRegisterAtEndWithThreeReturns(t *testing.T) {
	c := New()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// newTestThreeReturns has 3 return values — should fail at invocation time.
	if err := c.RegisterAtEnd(newTestThreeReturns); err != nil {
		t.Fatalf("RegisterAtEnd failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error for atEnd with 3 return values, got nil")
	}
}

func TestRegisterAtEndMissingDependency(t *testing.T) {
	c := New()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// AtEnd depends on newTestEvent which is not registered.
	if err := c.RegisterAtEnd(testAtEndFunc, newTestEvent); err != nil {
		t.Fatalf("RegisterAtEnd failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error for missing atEnd dependency, got nil")
	}
}

func TestRegisterAtEndArgumentMismatch(t *testing.T) {
	c := New()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// testAtEndFunc expects testEvent, but we pass newTestMessage.
	if err := c.RegisterAtEnd(testAtEndFunc, newTestMessage); err != nil {
		t.Fatalf("RegisterAtEnd failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error for atEnd argument type mismatch, got nil")
	}
}

// --- LoadDependencies accumulated error ---

func TestLoadDependenciesWithAccumulatedErrors(t *testing.T) {
	c := New()
	c.errs = append(c.errs, errors.New("accumulated error"))

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected accumulated error, got nil")
	}
	if err.Error() != "accumulated error" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Empty container ---

func TestEmptyContainerLoad(t *testing.T) {
	c := New()

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies on empty container failed: %v", err)
	}
}

// --- Constructor returning nil pointer ---

func TestNilPointerReturn(t *testing.T) {
	c := New()

	if err := c.Register(newTestNilPtr); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// nil pointer return is nillable, nil, and not an error — should succeed.
	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

// --- Constructor returning nil slice ---

func TestNilSliceReturn(t *testing.T) {
	c := New()

	if err := c.Register(newTestNilSliceReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

// --- Struct return (non-nillable kind, covers extractError fallthrough) ---

type testConfig struct{ Port int }

func newTestConfig() testConfig {
	return testConfig{Port: 8080}
}

func TestStructReturn(t *testing.T) {
	c := New()

	if err := c.Register(newTestConfig); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	dep, err := c.get(newTestConfig)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	cfg := dep.(testConfig)
	if cfg.Port != 8080 {
		t.Fatalf("expected 8080, got %d", cfg.Port)
	}
}

// --- Internal function edge cases ---

func TestGetEntryWithNonFunction(t *testing.T) {
	c := New()

	_, err := c.getEntry("not a function")
	if err == nil {
		t.Fatal("expected error from getEntry with non-function, got nil")
	}
}

func TestGetWithNonFunction(t *testing.T) {
	c := New()

	_, err := c.get("not a function")
	if err == nil {
		t.Fatal("expected error from get with non-function, got nil")
	}
}

func TestValidateArgumentsCountMismatch(t *testing.T) {
	// validateArguments expects func(testMessage) but we give 0 args.
	value := reflect.ValueOf(newTestGreeter)
	var args []reflect.Value // empty, but func expects 1 arg

	err := validateArguments("test-key", value, args)
	if err == nil {
		t.Fatal("expected error for arg count mismatch, got nil")
	}
}

func TestInvokeConstructorResolveFails(t *testing.T) {
	c := New()

	// Manually insert an entry that references a dependency not in the map,
	// so invokeConstructor's resolveArgs will fail.
	c.dependencyContainerMap["fake-key"] = entry{
		id:          "fake-id",
		constructor: newTestGreeter,
		// This dependency (newTestMessage) is not registered, so resolveArgs
		// will fail during invocation.
		constructorParameters: []constructor{newTestMessage},
	}

	err := c.invokeConstructor("fake-key")
	if err == nil {
		t.Fatal("expected error from invokeConstructor when dependency missing, got nil")
	}
}

func TestRegisterAddVertexError(t *testing.T) {
	c := New()

	// Pre-add the vertex key to the DAG to provoke AddVertex failure.
	ctorKey, _ := getConstructorKey(newTestMessage)
	c.graph.AddVertex(ctorKey)

	err := c.Register(newTestMessage)
	if err == nil {
		t.Fatal("expected error from AddVertex, got nil")
	}
}

func TestLoadDependenciesAddEdgeError(t *testing.T) {
	c := New()

	// Register constructors normally.
	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestGreeter, newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Corrupt the dep entry's ID to provoke AddEdge failure (invalid vertex ID).
	msgKey, _ := getConstructorKey(newTestMessage)
	e := c.dependencyContainerMap[msgKey]
	e.id = "non-existent-id"
	c.dependencyContainerMap[msgKey] = e

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error from AddEdge, got nil")
	}
}
