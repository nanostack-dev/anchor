package buildinfo

var (
	Version   = "dev"
	CommitSHA = "unknown"
	BuildDate = ""
)

func ServiceInfo(environment string) map[string]string {
	return map[string]string{
		"service":     "anchor",
		"environment": environment,
		"version":     Version,
		"commit_sha":  CommitSHA,
		"build_date":  BuildDate,
	}
}
