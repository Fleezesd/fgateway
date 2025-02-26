package deployer

type ControlPlaneInfo struct {
	XdsHost string
	XdsPort int32
}

type AwsInfo struct {
	EnableServiceAccountCredentials bool
	StsClusterName                  string
	StsUri                          string
}
