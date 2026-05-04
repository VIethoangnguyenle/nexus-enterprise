package ngac

// ExportCacheKey exposes cacheKey for testing (test files use package ngac_test).
func ExportCacheKey(req AccessRequest) string {
	return cacheKey(req)
}

// ExportVersionScope exposes versionScope for testing.
func ExportVersionScope(req AccessRequest) string {
	return versionScope(req)
}
