package service

// derefStr returns the string pointed to by p, or "" if p is nil.
func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// stringPtrOrNil returns nil for empty strings, otherwise a pointer to s.
// Mirror of derefStr for the storage→nullable direction.
func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
