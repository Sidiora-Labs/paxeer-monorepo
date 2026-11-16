package delivery

import "regexp"

// deploymentVersionToken is the manifest DeploymentVersion rule. Deployment
// identities are kept within this alphabet because they are serialized into
// durable documents and identity digests.
var deploymentVersionToken = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,95}$`)

// ValidDeploymentVersion reports whether value is admitted as a deployment
// identity by the deployment manifest.
func ValidDeploymentVersion(value string) bool {
	return deploymentVersionToken.MatchString(value)
}
