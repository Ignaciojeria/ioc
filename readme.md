# Golang Minimalist Dependency Injection Framework 🪡

## 🔧 Installation

    go get github.com/Ignaciojeria/ioc@latest

## 👨‍💻 Quick Example

```go
package main

import (
	"fmt"
	"log"

	"github.com/Ignaciojeria/ioc"
)

type Message string

func NewMessage() Message {
	return Message("Hi there!")
}

type Greeter struct {
	Message Message
}

func NewGreeter(m Message) Greeter {
	return Greeter{Message: m}
}

func (g Greeter) Greet() Message {
	return g.Message
}

type Event struct {
	Greeter Greeter
}

func NewEvent(g Greeter) Event {
	return Event{Greeter: g}
}

func main() {
	// No need to worry about order — the framework resolves
	// dependencies in the correct topological order.
	ioc.Register(NewEvent, NewGreeter)
	ioc.Register(NewGreeter, NewMessage)
	ioc.Register(NewMessage)

	if err := ioc.LoadDependencies(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Dependencies loaded successfully!")
}
```

## 🏗️ Advanced: Using a Container

For testing or multiple independent dependency graphs, create your own `Container`:

```go
c := ioc.New()

c.Register(NewMessage)
c.Register(NewGreeter, NewMessage)
c.Register(NewEvent, NewGreeter)

if err := c.LoadDependencies(); err != nil {
    log.Fatal(err)
}

// Reset the container for a clean slate (useful in tests).
c.Reset()
```

## 📌 API

| Function / Method | Description |
|---|---|
| `ioc.New()` | Create a new independent `Container` |
| `c.Register(ctor, deps...)` | Register a constructor and its dependencies |
| `c.RegisterAtEnd(ctor, deps...)` | Register a constructor to run after all others |
| `c.LoadDependencies()` | Resolve and invoke all constructors |
| `c.Reset()` | Clear all state (for testing) |
| `ioc.Register(...)` | Shortcut using the default global container |
| `ioc.RegisterAtEnd(...)` | Shortcut using the default global container |
| `ioc.LoadDependencies()` | Shortcut using the default global container |
| `ioc.Reset()` | Reset the default global container |

## 📜 License

MIT
