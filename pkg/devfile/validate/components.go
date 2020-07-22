package validate

import (
	"fmt"

	"github.com/openshift/odo/pkg/devfile/parser/data/common"
)

// Errors
var (
	ErrorNoComponents         = "no components present"
	ErrorNoContainerComponent = fmt.Sprintf("odo requires atleast one component of type '%s' in devfile", common.ContainerComponentType)
)

// validateComponents validates all the devfile components
func validateComponents(components []common.DevfileComponent) error {

	// components cannot be empty
	if len(components) < 1 {
		return fmt.Errorf(ErrorNoComponents)
	}

	// Check if component of type container  is present
	isContainerComponentPresent := false
	for _, component := range components {
		if component.Container != nil {
			isContainerComponentPresent = true
			break
		}
	}

	if !isContainerComponentPresent {
		return fmt.Errorf(ErrorNoContainerComponent)
	}

	// Successful
	return nil
}
