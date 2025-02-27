package ironic

import (
	"fmt"
	"reflect"

	"github.com/go-logr/logr"

	"github.com/gophercloud/gophercloud/openstack/baremetal/v1/nodes"
)

type optionsData map[string]interface{}

func optionValueEqual(current, value interface{}) bool {
	if reflect.DeepEqual(current, value) {
		return true
	}
	switch curVal := current.(type) {
	case []interface{}:
		// newType could reasonably be either []interface{} or e.g. []string,
		// so we must use reflection.
		newType := reflect.TypeOf(value)
		switch newType.Kind() {
		case reflect.Slice, reflect.Array:
		default:
			return false
		}
		newList := reflect.ValueOf(value)
		if newList.Len() != len(curVal) {
			return false
		}
		for i, v := range curVal {
			if !optionValueEqual(newList.Index(i).Interface(), v) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		// newType could reasonably be either map[string]interface{} or
		// e.g. map[string]string, so we must use reflection.
		newType := reflect.TypeOf(value)
		if newType.Kind() != reflect.Map ||
			newType.Key().Kind() != reflect.String {
			return false
		}
		newMap := reflect.ValueOf(value)
		if newMap.Len() != len(curVal) {
			return false
		}
		for k, v := range curVal {
			newV := newMap.MapIndex(reflect.ValueOf(k))
			if !(newV.IsValid() && optionValueEqual(newV.Interface(), v)) {
				return false
			}
		}
		return true
	}
	return false
}

func deref(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	if reflect.TypeOf(v).Kind() != reflect.Ptr {
		return v
	}
	if ptrVal := reflect.ValueOf(v); ptrVal.IsNil() {
		return nil
	} else {
		return ptrVal.Elem().Interface()
	}
}

func getUpdateOperation(name string, currentData map[string]interface{}, desiredValue interface{}, path string, log logr.Logger) *nodes.UpdateOperation {
	current, present := currentData[name]

	desiredValue = deref(desiredValue)
	if desiredValue != nil {
		if !(present && optionValueEqual(deref(current), desiredValue)) {
			if log != nil {
				if present {
					log.Info("updating option data",
						"value", desiredValue, "old_value", current)
				} else {
					log.Info("adding option data",
						"value", desiredValue)
				}
			}
			return &nodes.UpdateOperation{
				Op:    nodes.AddOp, // Add also does replace
				Path:  path,
				Value: desiredValue,
			}
		}
	} else {
		if present {
			if log != nil {
				log.Info("removing option data")
			}
			return &nodes.UpdateOperation{
				Op:   nodes.RemoveOp,
				Path: path,
			}
		}
	}
	return nil
}

type nodeUpdater struct {
	Updates nodes.UpdateOpts
	log     logr.Logger
}

func updateOptsBuilder(logger logr.Logger) *nodeUpdater {
	return &nodeUpdater{
		log: logger,
	}
}

func (nu *nodeUpdater) logger(basepath, option string) logr.Logger {
	if nu.log == nil {
		return nil
	}
	log := nu.log.WithValues("option", option, "section", basepath[1:])
	return log
}

func (nu *nodeUpdater) path(basepath, option string) string {
	return fmt.Sprintf("%s/%s", basepath, option)
}

func (nu *nodeUpdater) setSectionUpdateOpts(currentData map[string]interface{}, settings optionsData, basepath string) {
	for name, desiredValue := range settings {
		updateOp := getUpdateOperation(name, currentData, desiredValue,
			nu.path(basepath, name), nu.logger(basepath, name))
		if updateOp != nil {
			nu.Updates = append(nu.Updates, *updateOp)
		}
	}
}

func (nu *nodeUpdater) SetPropertiesOpts(settings optionsData, node *nodes.Node) *nodeUpdater {
	nu.setSectionUpdateOpts(node.Properties, settings, "/properties")
	return nu
}

func (nu *nodeUpdater) SetInstanceInfoOpts(settings optionsData, node *nodes.Node) *nodeUpdater {
	nu.setSectionUpdateOpts(node.InstanceInfo, settings, "/instance_info")
	return nu
}
