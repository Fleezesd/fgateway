package controller

import "k8s.io/apimachinery/pkg/runtime"

// SchemeBuilder contains all the Schemes for registering the CRDs with which fgateway interacts.
var SchemeBuilder = runtime.SchemeBuilder{}

// DefaultScheme returns a scheme with all the types registered for fgateway
func DefaultScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = AddToScheme(s)
	return s
}

func AddToScheme(s *runtime.Scheme) error {
	return SchemeBuilder.AddToScheme(s)
}
