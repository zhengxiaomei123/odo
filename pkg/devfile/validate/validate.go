package validate

import (
	"fmt"

	"github.com/openshift/odo/pkg/devfile/parser/data/common"
	"k8s.io/klog"

	v100 "github.com/openshift/odo/pkg/devfile/parser/data/1.0.0"
	v200 "github.com/openshift/odo/pkg/devfile/parser/data/2.0.0"
)

// ValidateDevfileData validates whether sections of devfile are odo compatible
func ValidateDevfileData(data interface{}) error {
	var components []common.DevfileComponent

	switch d := data.(type) {
	case *v100.Devfile100:
		components = d.GetComponents()
	case *v200.Devfile200:
		components = d.GetComponents()
	default:
		return fmt.Errorf("unknown devfile type %T", d)
	}

	// Validate Components
	if err := validateComponents(components); err != nil {
		return err
	}

	// Successful
	klog.V(4).Info("Successfully validated devfile sections")
	return nil

}
