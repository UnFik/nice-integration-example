package sessionusecase

// EndSessionURL is where the browser ends its NICE session; idTokenHint is
// optional. Empty when it cannot be built.
func (s *Service) EndSessionURL(idTokenHint string) string {
	u, err := s.NICE.EndSessionURL(idTokenHint)
	if err != nil {
		return ""
	}
	return u
}
