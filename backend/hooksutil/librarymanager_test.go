package hooksutil

import (
	"fmt"
	"plugin"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"isc.org/stork/hooks"
)

// Plugin mock.
type pluginMock struct {
	specificLookupOutput map[string]struct {
		result any
		err    error
	}
}

// Constructs the plugin mock instance.
func newPluginMock() *pluginMock {
	return &pluginMock{make(map[string]struct {
		result any
		err    error
	})}
}

// Implements the plugin interface. Returns the fixed values.
func (p *pluginMock) Lookup(symName string) (plugin.Symbol, error) {
	if output, ok := p.specificLookupOutput[symName]; ok {
		return output.result, output.err
	}
	panic("Lookup not registered")
}

// Add a dedicated lookup output for the Version symbol.
func (p *pluginMock) addLookupVersion(result any, err error) *pluginMock {
	p.specificLookupOutput["Version"] = struct {
		result any
		err    error
	}{
		result: result,
		err:    err,
	}
	return p
}

// Add a dedicated lookup output for the Load symbol.
func (p *pluginMock) addLookupLoad(result any, err error) *pluginMock {
	p.specificLookupOutput["Load"] = struct {
		result any
		err    error
	}{
		result: result,
		err:    err,
	}
	return p
}

// Add a dedicated lookup output for the CLIFlags symbol.
func (p *pluginMock) addLookupCLIFlags(result any, err error) *pluginMock {
	p.specificLookupOutput["CLIFlags"] = struct {
		result any
		err    error
	}{
		result: result,
		err:    err,
	}
	return p
}

// Callout carrier mock that stores an received settings.
type calloutCarrierMock struct {
	settings hooks.HookSettings
}

// Implements the mandatory Close function.
func (c *calloutCarrierMock) Close() error {
	return nil
}

// Function with a signature not matching the Load and Version.
func invalidSignature(int64) bool {
	return false
}

// Creates a valid Load function that returns the given output.
// If the error is nil, the function will return callout carrier.
func validLoad(err error) hooks.HookLoadFunction {
	return func(settings hooks.HookSettings) (hooks.CalloutCarrier, error) {
		if err != nil {
			return nil, err
		}
		return &calloutCarrierMock{settings: settings}, nil
	}
}

// Creates a valid CLIFlags function that returns the given output.
func validCLIFlags(settings hooks.HookSettings) hooks.HookCLIFlagsFunction {
	return func() hooks.HookSettings {
		return settings
	}
}

// Creates a valid Version function that returns the given output.
func validVersion(program, version string) hooks.HookVersionFunction {
	return func() (string, string) {
		return program, version
	}
}

// Test that the library constructor returns an error for an unknown file.
func TestNewLibraryManagerReturnErrorForInvalidPath(t *testing.T) {
	// Arrange & Act
	library, err := NewLibraryManager("/non/exist/file")

	// Assert
	require.Nil(t, library)
	require.Error(t, err)
}

// Test that the library manager constructor sets members properly.
func TestNewLibraryManager(t *testing.T) {
	// Arrange
	plugin := newPluginMock()

	// Act
	library := newLibraryManager("foo", plugin)

	// Assert
	require.Equal(t, plugin, library.p)
	require.EqualValues(t, "foo", library.path)
}

// Test that the load library function returns an error if the plugin doesn't
// contain the load function.
func TestLoadReturnErrorForMissingFunction(t *testing.T) {
	// Arrange
	library := newLibraryManager(
		"",
		newPluginMock().addLookupLoad(nil, errors.New("symbol not found")),
	)

	// Act
	calloutCarrier, err := library.Load(nil)

	// Assert
	require.Nil(t, calloutCarrier)
	require.Error(t, err)
}

// Test that the load library function returns an error if the load plugin
// function has unexpected signature.
func TestLoadReturnErrorForInvalidSignature(t *testing.T) {
	// Arrange
	library := newLibraryManager(
		"",
		newPluginMock().addLookupLoad(invalidSignature, nil),
	)

	// Act
	calloutCarrier, err := library.Load(nil)

	// Assert
	require.Nil(t, calloutCarrier)
	require.ErrorContains(t, err, "symbol Load has unexpected signature")
}

// Test that the load library function returns an error if the load plugin
// function returns an error.
func TestLoadReturnErrorOnFail(t *testing.T) {
	// Arrange
	library := newLibraryManager(
		"",
		newPluginMock().addLookupLoad(validLoad(errors.New("error in load")), nil),
	)

	// Act
	calloutCarrier, err := library.Load(nil)

	// Assert
	require.Nil(t, calloutCarrier)
	require.ErrorContains(t, err, "error in load")
}

// Test that the load library function returns a callout carrier on success.
func TestLoadReturnCalloutCarrierOnSuccess(t *testing.T) {
	// Arrange
	library := newLibraryManager("", newPluginMock().
		addLookupLoad(validLoad(nil), nil),
	)

	// Act
	calloutCarrier, err := library.Load(nil)

	// Assert
	require.NotNil(t, calloutCarrier)
	require.NoError(t, err)
}

// Test that the load library function accepts settings and returns a callout
// carrier on success.
func TestLoadWithSettingsReturnCalloutCarrierOnSuccess(t *testing.T) {
	// Arrange
	library := newLibraryManager("", newPluginMock().
		addLookupLoad(validLoad(nil), nil),
	)

	type settings struct {
		value string
	}

	// Act
	calloutCarrier, err := library.Load(&settings{value: "foo"})

	// Assert
	require.NotNil(t, calloutCarrier)
	require.NoError(t, err)
	require.Equal(t, "foo", calloutCarrier.(*calloutCarrierMock).settings.(*settings).value)
}

// Test that the version library function returns an error if the plugin doesn't
// contain the version function.
func TestVersionReturnErrorForMissingFunction(t *testing.T) {
	// Arrange
	library := newLibraryManager("", newPluginMock().addLookupVersion(nil, errors.New("symbol not found")))

	// Act
	program, version, err := library.Version()

	// Assert
	require.Empty(t, program)
	require.Empty(t, version)
	require.Error(t, err)
}

// Test that the version library function returns an error if the version plugin
// function has unexpected signature.
func TestVersionReturnErrorForInvalidSignature(t *testing.T) {
	// Arrange
	library := newLibraryManager("", newPluginMock().addLookupVersion(invalidSignature, nil))

	// Act
	program, version, err := library.Version()

	// Assert
	require.Empty(t, program)
	require.Empty(t, version)
	require.ErrorContains(t, err, "symbol Version has unexpected signature")
}

// Test that the version library function returns an application name and
// version string on success.
func TestVersionReturnAppAndVersionOnSuccess(t *testing.T) {
	// Arrange
	library := newLibraryManager("", newPluginMock().
		addLookupVersion(validVersion("bar", "baz"), nil),
	)

	// Act
	program, version, err := library.Version()

	// Assert
	require.EqualValues(t, "bar", program)
	require.EqualValues(t, "baz", version)
	require.NoError(t, err)
}

// Test that the CLIFlags library function returns nil and no error if the
// plugin doesn't contain the related function.
func TestCLIFlagsReturnNoErrorForMissingFunction(t *testing.T) {
	// Arrange
	library := newLibraryManager("", newPluginMock().
		addLookupCLIFlags(nil, errors.New("symbol not found")),
	)

	// Act
	settings, err := library.CLIFlags()

	// Assert
	require.NoError(t, err)
	require.Nil(t, settings)
}

// Test that the CLIFlags library function returns an error if the related
// plugin function has unexpected signature.
func TestCLIFlagsReturnErrorForInvalidSignature(t *testing.T) {
	// Arrange
	library := newLibraryManager("", newPluginMock().
		addLookupCLIFlags(invalidSignature, nil),
	)

	// Act
	settings, err := library.CLIFlags()

	// Assert
	require.Nil(t, settings)
	require.ErrorContains(t, err, "symbol CLIFlags has unexpected signature")
}

// Test that the CLIFlags library function returns the settings on success.
func TestCLIFlagsReturnSettingsOnSuccess(t *testing.T) {
	// Arrange
	library := newLibraryManager("", newPluginMock().
		addLookupCLIFlags(validCLIFlags(&struct{}{}), nil),
	)

	// Act
	settings, err := library.CLIFlags()

	// Assert
	require.NotNil(t, settings)
	require.NoError(t, err)
}

// Test that the CLIFlags library function can return nil.
func TestCLIFlagsReturnNil(t *testing.T) {
	// Arrange
	library := newLibraryManager("", newPluginMock().
		addLookupCLIFlags(validCLIFlags(nil), nil),
	)

	// Act
	settings, err := library.CLIFlags()

	// Assert
	require.Nil(t, settings)
	require.NoError(t, err)
}

// Test that the CLIFlags library function must return pointer to a struct.
func TestCLIFlagsReturnNonStructPointer(t *testing.T) {
	// Arrange
	var integer int

	values := []any{integer, &integer, true, struct{}{}}
	for i, value := range values {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			library := newLibraryManager("", newPluginMock().
				addLookupCLIFlags(validCLIFlags(value), nil),
			)

			// Act
			settings, err := library.CLIFlags()

			// Assert
			require.Nil(t, settings)
			require.ErrorContains(t, err, "must be a pointer to struct")
		})
	}
}

// Test that the path is returned properly.
func TestGetPath(t *testing.T) {
	// Arrange
	library := newLibraryManager("foo", nil)

	// Act
	path := library.GetPath()

	// Assert
	require.EqualValues(t, "foo", path)
}

// Test that the name is returned properly.
func TestGetName(t *testing.T) {
	paths := []string{
		"foo",
		"./foo",
		"/bar/bar/foo",
		"foo.bar",
		"./foo.bar",
		"/bar/foo.bar",
		"bar/foo.bar",
		"bar-bar/bar/foo.__bar__",
	}

	for _, path := range paths {
		// Arrange
		library := newLibraryManager(path, nil)

		// Act
		path := library.GetName()

		// Assert
		require.EqualValues(t, "foo", path)
	}
}
