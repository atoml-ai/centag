// Package edition identifies the product edition (personal vs team vs minimal).
package edition

// Edition is the deployment/product mode.
type Edition string

const (
	Personal Edition = "personal"
	Team     Edition = "team"
	Minimal  Edition = "minimal"
)

// Parse normalises an edition string. Unknown values default to team.
func Parse(value string) Edition {
	switch value {
	case string(Personal):
		return Personal
	case string(Minimal):
		return Minimal
	default:
		return Team
	}
}

func (e Edition) String() string {
	return string(e)
}

// IsPersonal reports whether multi-tenant / multi-user admin features are disabled.
func (e Edition) IsPersonal() bool {
	return e == Personal
}

// IsTeam reports whether full team/server features are enabled.
func (e Edition) IsTeam() bool {
	return e == Team
}

// IsMinimal reports whether this is a minimal distribution (no full frontend).
func (e Edition) IsMinimal() bool {
	return e == Minimal
}