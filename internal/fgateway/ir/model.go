package ir

type PodLocality struct {
	Region  string
	Zone    string
	Subzone string
}

type UniqlyConnectedClient struct {
	Role         string
	Labels       map[string]string
	Locality     PodLocality
	Namespace    string
	resourceName string
}
