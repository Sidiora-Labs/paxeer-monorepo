package deployment

import "testing"

func TestRepairM_953_PublicExportRPCFixture(t *testing.T) {
	t.Run("root_endpoint_and_confidentiality", TestPublicExportSecretsRevisionAndHash)
}

func TestRepairM_953_SerialExportRPCFixture(t *testing.T) {
	t.Run("canonical_root_endpoint_and_no_partial_export", TestSerialExportUsesOneCanonicalIdentityWithoutPartialExport)
	t.Run("manifest_and_interface_binding", TestCanonicalRevisionRejectsManifestAndInterfaceDrift)
}
