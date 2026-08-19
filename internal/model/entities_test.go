package model

import "testing"

func TestEnvironmentManifestRepresentsAllDiscoverySections(t *testing.T) {
	manifest := EnvironmentManifest{
		Metadata:     EnvironmentManifestMetadata{ManifestID: ManifestID("manifest-1")},
		Platform:     Platform{OSFamily: OSFamilyLinux, Architecture: CPUArchAMD64},
		Packages:     []Package{{Name: "git"}},
		Credentials:  []CredentialReference{{Type: "ssh-key", Path: "/home/user/.ssh/id_ed25519"}},
		Completeness: map[string]OperationStatus{"packages": StatusCompleted},
	}
	if manifest.Metadata.ManifestID != ManifestID("manifest-1") || manifest.Platform.OSFamily != OSFamilyLinux || len(manifest.Packages) != 1 || len(manifest.Credentials) != 1 {
		t.Fatal("manifest did not retain documented discovery data")
	}
}

func TestArchiveReferencesTheDocumentedIdentityAndStorageFields(t *testing.T) {
	archive := Archive{ID: ArchiveID("archive-1"), ManifestID: ManifestID("manifest-1"), Encryption: EncryptionAES256GCM, StorageLocations: []StorageLocation{{Backend: "local", Identifier: "backup.aers"}}}
	if archive.ID == "" || archive.ManifestID == "" || archive.Encryption != EncryptionAES256GCM || len(archive.StorageLocations) != 1 {
		t.Fatal("archive did not retain documented metadata")
	}
}
