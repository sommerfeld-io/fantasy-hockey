package web

import "github.com/sommerfeld-io/fantasy-hockey/internal/store"

// roster is an id-keyed lookup over one of the store's canonical lists
// (teams, NHL Players) - the single shape every "is this submitted id a real
// member" check in this package goes through, so server-side re-validation
// (AD-10) reads the same way for every sheet kind.
type roster[T any] map[string]T

// newRoster indexes items by key, keeping only the items keep accepts (every
// item when keep is nil). An item with an empty key is never indexed, so an
// empty id is never a member - a blank form field can't match anything. The
// store doesn't reject duplicate ids, so the first item with a given key wins,
// matching the linear scans this replaced.
func newRoster[T any](items []T, key func(T) string, keep func(T) bool) roster[T] {
	r := make(roster[T], len(items))
	for _, item := range items {
		id := key(item)
		if id == "" || (keep != nil && !keep(item)) {
			continue
		}
		if _, seen := r[id]; seen {
			continue
		}
		r[id] = item
	}
	return r
}

// has reports whether id is a member of r.
func (r roster[T]) has(id string) bool {
	_, ok := r[id]
	return ok
}

// get returns the member matching id, if any.
func (r roster[T]) get(id string) (T, bool) {
	item, ok := r[id]
	return item, ok
}

// teamKey keys a Team by its ID.
func teamKey(team store.Team) string { return team.ID }

// finalistKey keys an AwardFinalist by its Slug.
func finalistKey(finalist store.AwardFinalist) string { return finalist.Slug }

// teamRoster is every one of st's canonical teams.
func teamRoster(st *store.Store) roster[store.Team] {
	return newRoster(st.Teams(), teamKey, nil)
}

// divisionTeamRoster is st's canonical teams belonging to division - used to
// check a submitted team id belongs to the division it was submitted under.
func divisionTeamRoster(st *store.Store, division string) roster[store.Team] {
	return newRoster(st.Teams(), teamKey, func(team store.Team) bool { return team.Division == division })
}

// playerRoster is every one of st's canonical NHL Players, keyed by slug.
func playerRoster(st *store.Store) roster[store.AwardFinalist] {
	return newRoster(st.NHLPlayers(), finalistKey, nil)
}
