// Package stdio provides a Transport implementation that uses standard input/output.
package stdio

import (
	"github.com/localrivet/gomcp/types" // For types.Transport, types.TransportOptions, types.TransportFactory
)

// StdioTransportFactory creates StdioTransport instances.
type StdioTransportFactory struct {
	defaultOptions types.TransportOptions
}

// Compile-time check to ensure StdioTransportFactory implements TransportFactory.
var _ types.TransportFactory = (*StdioTransportFactory)(nil)

// NewStdioTransportFactory creates a new StdioTransportFactory with default options.
func NewStdioTransportFactory() *StdioTransportFactory {
	return &StdioTransportFactory{
		defaultOptions: types.TransportOptions{}, // Initialize with empty options
	}
}

// NewStdioTransportFactoryWithOptions creates a new StdioTransportFactory with the specified default options.
func NewStdioTransportFactoryWithOptions(opts types.TransportOptions) *StdioTransportFactory {
	return &StdioTransportFactory{
		defaultOptions: opts,
	}
}

// NewTransport creates a new StdioTransport with the factory's default options.
func (f *StdioTransportFactory) NewTransport() (types.Transport, error) {
	// Use the constructor that takes options
	return NewStdioTransportWithOptions(f.defaultOptions), nil
}

// NewTransportWithOptions creates a new StdioTransport with the specified options,
// potentially overriding the factory's defaults (though current implementation just uses provided opts).
func (f *StdioTransportFactory) NewTransportWithOptions(opts types.TransportOptions) (types.Transport, error) {
	// Use the constructor that takes options
	return NewStdioTransportWithOptions(opts), nil
}
