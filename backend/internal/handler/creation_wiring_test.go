//go:build unit

package handler

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProvideHandlersIncludesCreationPublication(t *testing.T) {
	publication := &CreationPublicationHandler{}
	provider := reflect.ValueOf(ProvideHandlers)
	args := make([]reflect.Value, provider.Type().NumIn())
	found := false
	for i := range args {
		argumentType := provider.Type().In(i)
		args[i] = reflect.Zero(argumentType)
		if argumentType == reflect.TypeOf(publication) {
			args[i] = reflect.ValueOf(publication)
			found = true
		}
	}
	require.True(t, found, "the startup provider must request the publication handler")
	handlers := provider.Call(args)[0].Interface().(*Handlers)
	require.Same(t, publication, handlers.CreationPublication, "publication routes must not silently be skipped at startup")
}

func TestProvideHandlersIncludesCreationMediaPricing(t *testing.T) {
	pricing := &CreationMediaPricingHandler{}
	provider := reflect.ValueOf(ProvideHandlers)
	args := make([]reflect.Value, provider.Type().NumIn())
	found := false
	for i := range args {
		argumentType := provider.Type().In(i)
		args[i] = reflect.Zero(argumentType)
		if argumentType == reflect.TypeOf(pricing) {
			args[i] = reflect.ValueOf(pricing)
			found = true
		}
	}
	require.True(t, found, "the startup provider must request the pricing handler")
	handlers := provider.Call(args)[0].Interface().(*Handlers)
	require.Same(t, pricing, handlers.CreationMediaPricing)
}
