package catalog

import "strings"

// v033ClassificationCorrection records one intentional v0.3.3 kind change
// for a frozen matcher identity (source + value).
type v033ClassificationCorrection struct {
	Name          string
	Justification string
	Previous      Classification
	Current       Classification
}

func v033ClassificationCorrections() []v033ClassificationCorrection {
	infraConfig := []Role{RoleInfra, RoleConfig}
	lockInfra := []Role{RoleLock, RoleInfra}
	return []v033ClassificationCorrection{
		{
			Name:          "docker-compose.yml",
			Justification: "Compose is a container orchestration manifest, not generic YAML",
			Previous:      Classification{Kind: "data.yaml", Roles: infraConfig, Source: SourceFilename, MatcherKey: "docker-compose.yml"},
			Current:       Classification{Kind: "manifest.container", Roles: infraConfig, Source: SourceFilename, MatcherKey: "docker-compose.yml"},
		},
		{
			Name:          "docker-compose.yaml",
			Justification: "Compose is a container orchestration manifest, not generic YAML",
			Previous:      Classification{Kind: "data.yaml", Roles: infraConfig, Source: SourceFilename, MatcherKey: "docker-compose.yaml"},
			Current:       Classification{Kind: "manifest.container", Roles: infraConfig, Source: SourceFilename, MatcherKey: "docker-compose.yaml"},
		},
		{
			Name:          "compose.yml",
			Justification: "Compose is a container orchestration manifest, not generic YAML",
			Previous:      Classification{Kind: "data.yaml", Roles: infraConfig, Source: SourceFilename, MatcherKey: "compose.yml"},
			Current:       Classification{Kind: "manifest.container", Roles: infraConfig, Source: SourceFilename, MatcherKey: "compose.yml"},
		},
		{
			Name:          "compose.yaml",
			Justification: "Compose is a container orchestration manifest, not generic YAML",
			Previous:      Classification{Kind: "data.yaml", Roles: infraConfig, Source: SourceFilename, MatcherKey: "compose.yaml"},
			Current:       Classification{Kind: "manifest.container", Roles: infraConfig, Source: SourceFilename, MatcherKey: "compose.yaml"},
		},
		{
			Name:          "docker-bake.hcl",
			Justification: "Docker Bake is a container build definition, not generic text or HCL source",
			Previous:      Classification{Kind: "document.text", Roles: infraConfig, Source: SourceFilename, MatcherKey: "docker-bake.hcl"},
			Current:       Classification{Kind: "manifest.container", Roles: infraConfig, Source: SourceFilename, MatcherKey: "docker-bake.hcl"},
		},
		{
			Name:          ".terraform.lock.hcl",
			Justification: "Terraform dependency lock belongs to the Terraform family while remaining a lockfile",
			Previous:      Classification{Kind: "document.text", Roles: lockInfra, Source: SourceFilename, MatcherKey: ".terraform.lock.hcl"},
			Current:       Classification{Kind: "manifest.terraform", Roles: lockInfra, Source: SourceFilename, MatcherKey: ".terraform.lock.hcl"},
		},
	}
}

func v033CorrectionByMatcherIdentity() map[string]Classification {
	result := make(map[string]Classification, 6)
	for _, correction := range v033ClassificationCorrections() {
		identity := string(correction.Current.Source) + ":" + strings.ToLower(correction.Current.MatcherKey)
		result[identity] = correction.Current
	}
	return result
}
