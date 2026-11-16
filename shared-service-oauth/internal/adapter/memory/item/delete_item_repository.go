package itemmemory

func (r *Repository) Delete(org, id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[org][id]; !ok {
		return false
	}
	delete(r.tenants[org], id)
	return true
}
