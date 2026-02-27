package ioc

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
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

// Constructor that returns a non-nil pointer.
type testService struct{ Name string }

func newTestServicePtr() *testService {
	return &testService{Name: "svc"}
}

// Constructor that returns a nil pointer.
func newTestNilPtr() *testService {
	return nil
}

// Constructor that returns an error as single return value.
func newTestSingleErrorReturn() error {
	return fmt.Errorf("single error return")
}

// Constructor that returns nil error as single return value.
func newTestSingleNilErrorReturn() error {
	return nil
}

// Various nillable return types.
func newTestSliceReturn() []string {
	return []string{"a", "b"}
}

func newTestMapReturn() map[string]string {
	return map[string]string{"k": "v"}
}

func newTestChanReturn() chan int {
	return make(chan int)
}

func newTestFuncReturn() func() {
	return func() {}
}

// Void constructor (no return values).
func newTestVoidConstructor(m testMessage) {
	_ = m
}

// Interface-based injection.
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

// Struct return (non-nillable).
type testConfig struct{ Port int }

func newTestConfig() testConfig {
	return testConfig{Port: 8080}
}

// Second implementation of testStringer (for ambiguity test).
type testStringerImpl2 struct{ Val string }

func (s testStringerImpl2) String() string { return s.Val }

func newTestStringerImpl2() testStringerImpl2 {
	return testStringerImpl2{Val: "impl2"}
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
	c := newContainer()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

func TestDependencyChain(t *testing.T) {
	c := newContainer()

	// Register in any order — types are matched automatically.
	if err := c.Register(newTestEvent); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestGreeter); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

func TestConstructorWithError(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestFailingConstructor); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error from failing constructor, got nil")
	}
	if !strings.Contains(err.Error(), "constructor failed intentionally") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConstructorReturningValueAndNilError(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestMessageWithError); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

func TestRegisterAtEnd(t *testing.T) {
	atEndCalled = false
	c := newContainer()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestGreeter); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestEvent); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.RegisterAtEnd(testAtEndFunc); err != nil {
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
	c := newContainer()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.RegisterAtEnd(testAtEndFirst); err != nil {
		t.Fatalf("RegisterAtEnd failed: %v", err)
	}
	if err := c.RegisterAtEnd(testAtEndSecond); err != nil {
		t.Fatalf("RegisterAtEnd failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	if len(atEndOrder) != 2 || atEndOrder[0] != "first" || atEndOrder[1] != "second" {
		t.Fatalf("expected [first, second], got %v", atEndOrder)
	}
}

func TestRegisterAtEndFailing(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.RegisterAtEnd(testAtEndFailing); err != nil {
		t.Fatalf("RegisterAtEnd failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error from failing atEnd constructor, got nil")
	}
}

func TestDuplicateTypeProvider(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	// newTestMessageWithError also returns testMessage — should fail.
	err := c.Register(newTestMessageWithError)
	if err == nil {
		t.Fatal("expected error on duplicate type provider, got nil")
	}
}

func TestNonFunctionRegistration(t *testing.T) {
	c := newContainer()

	err := c.Register("not a function")
	if err == nil {
		t.Fatal("expected error when registering a non-function, got nil")
	}
}

func TestNonFunctionRegisterAtEnd(t *testing.T) {
	c := newContainer()

	err := c.RegisterAtEnd("not a function")
	if err == nil {
		t.Fatal("expected error when RegisterAtEnd with a non-function, got nil")
	}
}

func TestReset(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	c.reset()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register after reset failed: %v", err)
	}
	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies after reset failed: %v", err)
	}
}

func TestMissingProvider(t *testing.T) {
	c := newContainer()

	// newTestGreeter needs testMessage, but no provider registered.
	if err := c.Register(newTestGreeter); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error for missing provider, got nil")
	}
}

func TestMultipleContainers(t *testing.T) {
	c1 := newContainer()
	c2 := newContainer()

	if err := c1.Register(newTestMessage); err != nil {
		t.Fatalf("c1 Register failed: %v", err)
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
}

// --- Validation edge cases ---

func TestThreeReturnValues(t *testing.T) {
	c := newContainer()

	err := c.Register(newTestThreeReturns)
	if err == nil {
		t.Fatal("expected error for 3 return values, got nil")
	}
}

func TestBadSecondReturn(t *testing.T) {
	c := newContainer()

	err := c.Register(newTestBadSecondReturn)
	if err == nil {
		t.Fatal("expected error for non-error second return, got nil")
	}
}

func TestVoidConstructor(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestVoidConstructor); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

func TestPointerReturn(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestServicePtr); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

func TestSingleReturnError(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestSingleErrorReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error from single-return error constructor, got nil")
	}
}

func TestSingleReturnNilError(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestSingleNilErrorReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

func TestSliceReturn(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestSliceReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

func TestMapReturn(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestMapReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

func TestChanReturn(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestChanReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

func TestFuncReturn(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestFuncReturn); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

func TestStructReturn(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestConfig); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

func TestInterfaceInjection(t *testing.T) {
	c := newContainer()

	// testStringerImpl implements testStringer.
	// newTestNeedsStringer(testStringer) should get testStringerImpl injected.
	if err := c.Register(newTestStringerImpl); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestNeedsStringer); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}
}

// --- Global wrapper tests ---

func TestGlobalRegisterAndLoad(t *testing.T) {
	resetDefault()

	if err := Register(newTestMessage); err != nil {
		t.Fatalf("global Register failed: %v", err)
	}
	if err := Register(newTestGreeter); err != nil {
		t.Fatalf("global Register failed: %v", err)
	}

	if err := LoadDependencies(); err != nil {
		t.Fatalf("global LoadDependencies failed: %v", err)
	}

	resetDefault()
}

func TestGlobalRegisterAtEnd(t *testing.T) {
	atEndCalled = false
	resetDefault()

	if err := Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := Register(newTestGreeter); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := Register(newTestEvent); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := RegisterAtEnd(testAtEndFunc); err != nil {
		t.Fatalf("RegisterAtEnd failed: %v", err)
	}

	if err := LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies failed: %v", err)
	}

	if !atEndCalled {
		t.Fatal("expected atEnd to be called")
	}

	resetDefault()
}

func TestGlobalReset(t *testing.T) {
	resetDefault()

	if err := Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	resetDefault()

	if err := Register(newTestMessage); err != nil {
		t.Fatalf("Register after reset failed: %v", err)
	}

	resetDefault()
}

// --- AtEnd edge cases ---

func TestRegisterAtEndWithThreeReturns(t *testing.T) {
	c := newContainer()

	err := c.RegisterAtEnd(newTestThreeReturns)
	if err == nil {
		t.Fatal("expected error for atEnd with 3 return values, got nil")
	}
}

func TestRegisterAtEndMissingDep(t *testing.T) {
	c := newContainer()

	// testAtEndFunc needs testEvent, but nothing registered provides it.
	if err := c.RegisterAtEnd(testAtEndFunc); err != nil {
		t.Fatalf("RegisterAtEnd failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error for missing atEnd dependency, got nil")
	}
}

// --- Accumulated errors ---

func TestAccumulatedErrors(t *testing.T) {
	c := newContainer()
	c.errs = append(c.errs, errors.New("accumulated error"))

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected accumulated error, got nil")
	}
}

// --- Empty container ---

func TestEmptyContainerLoad(t *testing.T) {
	c := newContainer()

	if err := c.LoadDependencies(); err != nil {
		t.Fatalf("LoadDependencies on empty container failed: %v", err)
	}
}

// --- Internal defensive path tests ---

func TestAddVertexError(t *testing.T) {
	c := newContainer()

	// Pre-add the vertex key to the DAG to provoke AddVertex failure.
	ctorKey, _, _, _ := getConstructorInfo(newTestMessage)
	c.graph.AddVertex(ctorKey)

	err := c.Register(newTestMessage)
	if err == nil {
		t.Fatal("expected error from AddVertex, got nil")
	}
}

func TestAddEdgeError(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestGreeter); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Corrupt the dagID of the provider to force AddEdge failure.
	msgType := reflect.TypeOf(testMessage(""))
	e := c.typeToEntry[msgType]
	e.dagID = "non-existent-id"
	c.typeToEntry[msgType] = e

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error from AddEdge, got nil")
	}
}

func TestInvokeAndStoreValidationError(t *testing.T) {
	c := newContainer()

	// Manually insert an entry with a bad constructor (3 return values)
	// that bypasses Register validation.
	key := "bad-ctor"
	c.keyToEntry[key] = entry{
		key:         key,
		constructor: newTestThreeReturns,
	}

	err := c.invokeAndStore(key, c.keyToEntry[key])
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestResolveArgsDependencyNotInitialized(t *testing.T) {
	c := newContainer()

	if err := c.Register(newTestMessage); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Don't call LoadDependencies — message is registered but not initialized.
	// Try to resolve args for a function that needs testMessage.
	ctorType := reflect.TypeOf(newTestGreeter)
	_, err := c.resolveArgsByType(ctorType)
	if err == nil {
		t.Fatal("expected error for uninitialized dependency, got nil")
	}
}

func TestAmbiguousInterfaceProvider(t *testing.T) {
	c := newContainer()

	// Both testStringerImpl and testStringerImpl2 implement testStringer.
	if err := c.Register(newTestStringerImpl); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestStringerImpl2); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := c.Register(newTestNeedsStringer); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := c.LoadDependencies()
	if err == nil {
		t.Fatal("expected error for ambiguous interface provider, got nil")
	}
}
