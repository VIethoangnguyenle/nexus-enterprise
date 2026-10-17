package store

// ForceWideIndex makes the store behave as if channels_one_workspace_wide exists
// (present) or does not (migration 060 not yet applied), whatever the test
// database holds, so both sides of the migration can be tested on one database.
func (s *Store) ForceWideIndex(present bool) {
	if present {
		s.wideIndex.Store(wideIndexPresent)
		return
	}
	s.wideIndex.Store(wideIndexAbsent)
}
