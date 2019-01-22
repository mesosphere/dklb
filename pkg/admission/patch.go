package admission

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/appscode/jsonpatch"
	"k8s.io/apimachinery/pkg/runtime"
)

// CreateRFC6902Patch creates an RFC6902 patch that captures the difference between the specified resources.
func CreateRFC6902Patch(oldObj, newObj runtime.Object) ([]byte, error) {
	// Make sure we're dealing with resources of the same GVK.
	oldGVK := oldObj.GetObjectKind().GroupVersionKind()
	newGVK := newObj.GetObjectKind().GroupVersionKind()
	if !reflect.DeepEqual(oldGVK, newGVK) {
		return nil, fmt.Errorf("gvk mismatch (expected %v, got %v)", oldGVK, newGVK)
	}
	// Marshal the old object.
	oldBytes, err := json.Marshal(oldObj)
	if err != nil {
		return nil, err
	}
	// Marshal the new object.
	newBytes, err := json.Marshal(newObj)
	if err != nil {
		return nil, err
	}
	// Create the RFC6902 patch based on the representations of the old and new objects.
	r, err := jsonpatch.CreatePatch(oldBytes, newBytes)
	if err != nil {
		return nil, err
	}
	// Return a byte array containing the patch.
	return json.Marshal(r)
}
