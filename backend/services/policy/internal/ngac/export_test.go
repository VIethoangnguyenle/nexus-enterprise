package ngac

// ExportCacheKey exposes cacheKey for testing (test files use package ngac_test).
func ExportCacheKey(req AccessRequest) string {
	return cacheKey(req)
}
