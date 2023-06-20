package agent

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	agentapi "isc.org/stork/api"
	"isc.org/stork/hooks/agent/forwardtokeaoverhttpcallouts"
	"isc.org/stork/hooksutil"
	storkutil "isc.org/stork/util"
)

// Facade for all callouts. It defines the specific calling method for
// each callout.
type HookManager struct {
	hooksutil.HookManager
}

// Interface checks.
var _ forwardtokeaoverhttpcallouts.BeforeForwardToKeaOverHTTPCallouts = (*HookManager)(nil)

// Constructs new hook manager.
func NewHookManager() *HookManager {
	return &HookManager{
		HookManager: *hooksutil.NewHookManager([]reflect.Type{
			reflect.TypeOf((*forwardtokeaoverhttpcallouts.BeforeForwardToKeaOverHTTPCallouts)(nil)).Elem(),
		}),
	}
}

// Callout executed before forwarding a command to Kea over HTTP.
func (hm *HookManager) OnBeforeForwardToKeaOverHTTP(ctx context.Context, in *agentapi.ForwardToKeaOverHTTPReq) error {
	errors := hooksutil.CallSequential(hm.GetExecutor(), func(carrier forwardtokeaoverhttpcallouts.BeforeForwardToKeaOverHTTPCallouts) error {
		err := carrier.OnBeforeForwardToKeaOverHTTP(ctx, in)
		err = errors.Wrap(err, "error occurred in the OnBeforeForwardToKeaOverHTTP callout")
		return err
	})
	return storkutil.CombineErrors("error in the onBeforeForwardToKeaOverHTTP callout", errors)
}
