package hero

import (
	stdContext "context"
	"fmt"
	"time"

	"github.com/kataras/iris/v12/context"
	"github.com/kataras/iris/v12/sessions"
)

func fatalf(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}

// Default is the default container value which can be used for dependencies share.
var Default = New()

// Container contains and delivers the Dependencies that will be binded
// to the controller(s) or handler(s) that can be created
// using the Container's `Handler` and `Struct` methods.
//
// This is not exported for being used by everyone, use it only when you want
// to share containers between multi mvc.go#Application
// or make custom hero handlers that can be used on the standard
// iris' APIBuilder.
//
// For a more high-level structure please take a look at the "mvc.go#Application".
type Container struct {
	// Indicates the path parameter start index for inputs binding.
	// Defaults to 0.
	ParamStartIndex int
	// Sorter specifies how the inputs should be sorted before binded.
	// Defaults to sort by "thinnest" target empty interface.
	Sorter Sorter
	// The dependencies entries.
	Dependencies []*Dependency
	// GetErrorHandler should return a valid `ErrorHandler` to handle bindings AND handler dispatch errors.
	// Defaults to a functon which returns the `DefaultErrorHandler`.
	GetErrorHandler func(context.Context) ErrorHandler // cannot be nil.
}

var BuiltinDependencies = []*Dependency{
	// iris context dependency.
	NewDependency(func(ctx context.Context) context.Context { return ctx }),
	// standard context dependency.
	NewDependency(func(ctx context.Context) stdContext.Context {
		return ctx.Request().Context()
	}),
	// iris session dependency.
	NewDependency(func(ctx context.Context) *sessions.Session {
		session := sessions.Get(ctx)
		if session == nil {
			panic("binding: session is nil - app.Use(sess.Handler()) to fix it")
		}

		return session
	}),
	// time.Time to time.Now dependency.
	NewDependency(func(ctx context.Context) time.Time {
		return time.Now()
	}),

	// payload and param bindings are dynamically allocated and declared at the end of the `binding` source file.
}

// New returns a new Container, a container for dependencies and a factory
// for handlers and controllers, this is used internally by the `mvc#Application` structure.
// Please take a look at the structure's documentation for more information.
func New(dependencies ...interface{}) *Container {
	deps := make([]*Dependency, len(BuiltinDependencies))
	copy(deps, BuiltinDependencies)

	c := &Container{
		ParamStartIndex: 0,
		Sorter:          sortByNumMethods,
		Dependencies:    deps,
		GetErrorHandler: func(context.Context) ErrorHandler {
			return DefaultErrorHandler
		},
	}

	for _, dependency := range dependencies {
		c.Register(dependency)
	}

	return c
}

// Clone returns a new cloned container.
// It copies the ErrorHandler, Dependencies and all Options from "c" receiver.
func (c *Container) Clone() *Container {
	cloned := New()
	cloned.ParamStartIndex = c.ParamStartIndex
	cloned.GetErrorHandler = c.GetErrorHandler
	cloned.Sorter = c.Sorter
	clonedDeps := make([]*Dependency, len(c.Dependencies))
	copy(clonedDeps, c.Dependencies)
	cloned.Dependencies = clonedDeps
	return cloned
}

// Register adds a dependency.
// The value can be a single struct value-instance or a function
// which has one input and one output, that output type
// will be binded to the handler's input argument, if matching.
//
// Usage:
// - Register(loggerService{prefix: "dev"})
// - Register(func(ctx iris.Context) User {...})
// - Register(func(User) OtherResponse {...})
func Register(dependency interface{}) *Dependency {
	return Default.Register(dependency)
}

// Register adds a dependency.
// The value can be a single struct value or a function.
// Follow the rules:
// * <T>{structValue}
// * func(accepts <T>)                                 returns <D> or (<D>, error)
// * func(accepts iris.Context)                        returns <D> or (<D>, error)
// * func(accepts1 iris.Context, accepts2 *hero.Input) returns <D> or (<D>, error)
//
// A Dependency can accept a previous registered dependency and return a new one or the same updated.
// * func(accepts1 <D>, accepts2 <T>)                  returns <E> or (<E>, error) or error
// * func(acceptsPathParameter1 string, id uint64)     returns <T> or (<T>, error)
//
// Usage:
//
// - Register(loggerService{prefix: "dev"})
// - Register(func(ctx iris.Context) User {...})
// - Register(func(User) OtherResponse {...})
func (c *Container) Register(dependency interface{}) *Dependency {
	d := NewDependency(dependency, c.Dependencies...)
	if d.DestType == nil {
		// prepend the dynamic dependency so it will be tried at the end
		// (we don't care about performance here, design-time)
		c.Dependencies = append([]*Dependency{d}, c.Dependencies...)
	} else {
		c.Dependencies = append(c.Dependencies, d)
	}

	return d
}

// Handler accepts a "handler" function which can accept any input arguments that match
// with the Container's `Dependencies` and any output result; like string, int (string,int),
// custom structs, Result(View | Response) and anything you can imagine.
// It returns a standard `iris/context.Handler` which can be used anywhere in an Iris Application,
// as middleware or as simple route handler or subdomain's handler.
func Handler(fn interface{}) context.Handler {
	return Default.Handler(fn)
}

// Handler accepts a handler "fn" function which can accept any input arguments that match
// with the Container's `Dependencies` and any output result; like string, int (string,int),
// custom structs, Result(View | Response) and more.
// It returns a standard `iris/context.Handler` which can be used anywhere in an Iris Application,
// as middleware or as simple route handler or subdomain's handler.
func (c *Container) Handler(fn interface{}) context.Handler {
	return makeHandler(fn, c)
}

func (c *Container) Struct(ptrValue interface{}) *Struct {
	return makeStruct(ptrValue, c)
}