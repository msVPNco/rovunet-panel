package coreinstall

// PrepareReviewedEngines is an explicit installer operation. Existing runtimes
// first follow the authenticated, recoverable update transaction; the normal
// boot/start admission path still rejects an unexpected executable.
func PrepareReviewedEngines(protocols []string) error {
	if err := RefreshInstalledEngines(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, protocol := range protocols {
		if seen[protocol] {
			continue
		}
		seen[protocol] = true
		if err := ensureEngine(protocol); err != nil {
			return err
		}
	}
	return nil
}
